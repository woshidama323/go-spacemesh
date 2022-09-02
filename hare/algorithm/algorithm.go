package algorithm

import (
	"context"
	"errors"
	"sync"

	"github.com/spacemeshos/go-spacemesh/log"
	"golang.org/x/sync/errgroup"
)

type round int

const (
	preRound      = round(-1)
	statusRound   = round(0)
	proposalRound = round(1)
	commitRound   = round(2)
	notifyRound   = round(3)
)

func roundCountToRound(roundCount int) round {
	if roundCount == 0 {
		return preRound
	}
	return round(roundCount % 4)
}

type RoundClock interface {
	AwaitEndOfRound(round uint32) <-chan struct{}
}

type ConsensusProcess struct {
	state
	msgs       *messageCache
	roundClock RoundClock
	maxItter   int

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
	go cp.runRound(preRound)
	// Continue running rounds until the context is cancelled or max itterations
	// is reached.
	for roundCount := 0; ; roundCount++ {
		select {
		case <-cp.ctx.Done():
			return nil
		case <-cp.roundClock.AwaitEndOfRound(uint32(roundCount)):
			r := roundCountToRound(roundCount)
			cp.eg.Go(func() error {
				return cp.runRound(r)
			})
		}
	}
}

func (cp *ConsensusProcess) runRound(roundCount round) error {
	switch roundCount {
	case preRound:
		return cp.preRound()
	case statusRound:
		return cp.statusRound()
	case proposalRound:
		return cp.proposalRound()
	case commitRound:
		return cp.commitRound()
	case notifyRound:
		return cp.notifyRound()
	}
}
