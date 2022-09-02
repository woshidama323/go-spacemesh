package algorithm

import (
	"context"
	"errors"
	"sync"

	"github.com/spacemeshos/go-spacemesh/log"
	"golang.org/x/sync/errgroup"
)

type RoundClock interface {
	AwaitEndOfRound(round, itter) <-chan struct{}
}

type ConsensusProcess struct {
	state
	msgs       *messageCache
	roundClock RoundClock
	maxItter   itter

	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	eg     errgroup.Group

	logger log.Logger
}

func (cp ConsensusProcess) Start() {
	cp.once.Do(func() {
		cp.eg.Go(func() error {
			return cp.run()
		})
	})
}

func (cp *ConsensusProcess) Stop() {
	cp.cancel()
	err := cp.eg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		cp.logger.With().Error("blockGen task failure", log.Err(err))
	}
}

func (cp *ConsensusProcess) run() error {
	// Run preRound
	// Broadcast inputSet in preRound message if eligible
	// TODO: Broadcast inputSet in preRound message if eligible
	// Wait for end of preRound to reduce messages to preRoundCertificate
	select {
	case <-cp.ctx.Done():
		return nil
	case <-cp.roundClock.AwaitEndOfRound(preRound, 0):
		cp.eg.Go(func() error {
			return resolvePreRound(cp.state.inputSet, *cp.msgs)
		})

	}

	// Continue running itterations until the context is cancelled or max itterations
	// is reached.
	for itter := itter(0); itter < cp.maxItter; itter++ {
		// Status Round
		select {
		case <-ctx.Done():
			return nil
		case <-c.AwaitEndOfRound(statusRound, itter):
			cp.eg.Go(func() error {
				return cp.runRound(r)
			})
		}
	}
}
