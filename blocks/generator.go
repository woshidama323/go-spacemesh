package blocks

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hare/eligibility"
	"github.com/spacemeshos/go-spacemesh/hare3"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/layers"
	"github.com/spacemeshos/go-spacemesh/sql/proposals"
	"github.com/spacemeshos/go-spacemesh/system"
)

// Generator generates a block from proposals.
type Generator struct {
	logger log.Log
	cfg    Config
	once   sync.Once
	eg     errgroup.Group
	stop   func()

	db       *sql.Database
	atxs     *atxsdata.Data
	msh      meshProvider
	executor executor
	fetcher  system.ProposalFetcher
	cert     certifier
	patrol   layerPatrol

	hareCh           <-chan hare3.ConsensusOutput
	optimisticOutput map[types.LayerID]*proposalMetadata
}

// Config is the config for Generator.
type Config struct {
	GenBlockInterval   time.Duration
	BlockGasLimit      uint64
	OptFilterThreshold int
}

func defaultConfig() Config {
	return Config{
		GenBlockInterval:   time.Second,
		BlockGasLimit:      math.MaxUint64,
		OptFilterThreshold: 90,
	}
}

// GeneratorOpt for configuring Generator.
type GeneratorOpt func(*Generator)

// WithConfig defines cfg for Generator.
func WithConfig(cfg Config) GeneratorOpt {
	return func(g *Generator) {
		g.cfg = cfg
	}
}

// WithGeneratorLogger defines logger for Generator.
func WithGeneratorLogger(logger log.Log) GeneratorOpt {
	return func(g *Generator) {
		g.logger = logger
	}
}

// WithHareOutputChan sets the chan to listen to hare output.
func WithHareOutputChan(ch <-chan hare3.ConsensusOutput) GeneratorOpt {
	return func(g *Generator) {
		g.hareCh = ch
	}
}

// NewGenerator creates new block generator.
func NewGenerator(
	db *sql.Database,
	atxs *atxsdata.Data,
	exec executor,
	m meshProvider,
	f system.ProposalFetcher,
	c certifier,
	p layerPatrol,
	opts ...GeneratorOpt,
) *Generator {
	g := &Generator{
		logger:           log.NewNop(),
		cfg:              defaultConfig(),
		db:               db,
		atxs:             atxs,
		msh:              m,
		executor:         exec,
		fetcher:          f,
		cert:             c,
		patrol:           p,
		optimisticOutput: map[types.LayerID]*proposalMetadata{},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Start starts listening to hare output.
func (g *Generator) Start(ctx context.Context) {
	g.once.Do(func() {
		ctx, g.stop = context.WithCancel(ctx)
		g.eg.Go(func() error {
			return g.run(ctx)
		})
	})
}

// Stop stops listening to hare output.
func (g *Generator) Stop() {
	if g.stop == nil {
		return
	}
	g.stop()
	err := g.eg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		g.logger.With().Error("blockGen task failure", log.Err(err))
	}
}

func (g *Generator) run(ctx context.Context) error {
	var maxLayer types.LayerID
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context done: %w", ctx.Err())
		case out, open := <-g.hareCh:
			if !open {
				return nil
			}
			g.logger.With().Debug("received hare output",
				log.Context(ctx),
				out.Layer,
				log.Int("num_proposals", len(out.Proposals)),
			)
			maxLayer = max(maxLayer, out.Layer)
			_, err := g.processHareOutput(ctx, out)
			if err != nil {
				if errors.Is(err, errNodeHasBadMeshHash) {
					g.logger.With().Info("node has different mesh hash from majority, will download block instead",
						log.Context(ctx),
						out.Layer,
						log.Err(err),
					)
				} else {
					g.logger.With().Error("failed to process hare output",
						log.Context(ctx),
						out.Layer,
						log.Err(err),
					)
				}
			}
			if len(g.optimisticOutput) > 0 {
				g.processOptimisticLayers(maxLayer)
			}
		case <-time.After(g.cfg.GenBlockInterval):
			if len(g.optimisticOutput) > 0 {
				g.processOptimisticLayers(maxLayer)
			}
		}
	}
}

func (g *Generator) getProposals(pids []types.ProposalID) ([]*types.Proposal, error) {
	var (
		result = make([]*types.Proposal, 0, len(pids))
		p      *types.Proposal
		err    error
	)
	for _, pid := range pids {
		if p, err = proposals.Get(g.db, pid); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (g *Generator) processHareOutput(ctx context.Context, out hare3.ConsensusOutput) (*types.Block, error) {
	var md *proposalMetadata
	if len(out.Proposals) > 0 {
		getMetadata := func() error {
			// fetch proposals from peers if not locally available
			if err := g.fetcher.GetProposals(ctx, out.Proposals); err != nil {
				failFetchCnt.Inc()
				return fmt.Errorf("preprocess fetch layer %d proposals: %w", out.Layer, err)
			}
			// now all proposals should be in local DB
			props, err := g.getProposals(out.Proposals)
			if err != nil {
				failErrCnt.Inc()
				return fmt.Errorf("preprocess get layer %d proposals: %w", out.Layer, err)
			}
			md, err = getProposalMetadata(ctx, g.logger, g.db, g.atxs, g.cfg, out.Layer, props)
			if err != nil {
				return err
			}
			return nil
		}
		if err := getMetadata(); err != nil {
			g.patrol.CompleteHare(out.Layer)
			return nil, err
		}
	}

	if md != nil && md.optFilter {
		g.optimisticOutput[out.Layer] = md
		return nil, nil
	}

	defer g.patrol.CompleteHare(out.Layer)
	var (
		block      *types.Block
		hareOutput types.BlockID
	)
	if md != nil {
		block = &types.Block{
			InnerBlock: types.InnerBlock{
				LayerIndex: md.lid,
				TickHeight: md.tickHeight,
				Rewards:    md.rewards,
				TxIDs:      md.tids,
			},
		}
		block.Initialize()
		hareOutput = block.ID()
		g.logger.With().Info("generated block", out.Layer, block.ID())
	}
	if err := g.saveAndCertify(ctx, out.Layer, block); err != nil {
		return block, err
	}
	if err := g.msh.ProcessLayerPerHareOutput(ctx, out.Layer, hareOutput, false); err != nil {
		return block, err
	}
	return block, nil
}

func (g *Generator) processOptimisticLayers(max types.LayerID) {
	lastApplied, err := layers.GetLastApplied(g.db)
	if err != nil {
		g.logger.Error("failed to get latest applied layer", log.Err(err))
		return
	}
	next := lastApplied.Add(1)
	for lid := next; lid <= max; lid++ {
		md, ok := g.optimisticOutput[lid]
		if !ok {
			return
		}
		delete(g.optimisticOutput, lid)

		doit := func() error {
			defer g.patrol.CompleteHare(lid)
			block, err := g.genBlockOptimistic(md.ctx, md)
			if err != nil {
				failGenCnt.Inc()
				return err
			}
			g.logger.With().Info("generated block (optimistic)", lid, block.ID())
			if err = g.msh.ProcessLayerPerHareOutput(md.ctx, lid, block.ID(), true); err != nil {
				return err
			}
			return nil
		}
		if err = doit(); err != nil {
			g.logger.With().Error("failed to process optimistic layer",
				log.Context(md.ctx),
				lid,
				log.Err(err),
			)
			return
		}
	}
}

func (g *Generator) saveAndCertify(ctx context.Context, lid types.LayerID, block *types.Block) error {
	hareOutput := types.EmptyBlockID
	if block != nil {
		if err := g.msh.AddBlockWithTXs(ctx, block); err != nil {
			failErrCnt.Inc()
			return fmt.Errorf("post process add block: %w", err)
		}
		blockOkCnt.Inc()
		hareOutput = block.ID()
	} else {
		emptyOutputCnt.Inc()
	}

	if err := g.cert.RegisterForCert(ctx, lid, hareOutput); err != nil {
		g.logger.With().Warning("failed to register hare output for certifying",
			log.Context(ctx),
			lid,
			hareOutput,
			log.Err(err),
		)
	}

	if err := g.cert.CertifyIfEligible(ctx, lid, hareOutput); err != nil && !errors.Is(err, eligibility.ErrNotActive) {
		g.logger.With().Warning("failed to certify block",
			log.Context(ctx),
			lid,
			hareOutput,
			log.Err(err),
		)
	}
	return nil
}

func (g *Generator) genBlockOptimistic(ctx context.Context, md *proposalMetadata) (*types.Block, error) {
	block, err := g.executor.ExecuteOptimistic(ctx, md.lid, md.tickHeight, md.rewards, md.tids)
	if err != nil {
		failGenCnt.Inc()
		return nil, fmt.Errorf("execute in situ: %w", err)
	}
	if err = g.saveAndCertify(ctx, md.lid, block); err != nil {
		return nil, fmt.Errorf("post-process block (optimistic): %w", err)
	}
	return block, nil
}
