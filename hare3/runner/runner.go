package runner

import (
	"context"
	"time"

	"github.com/spacemeshos/go-spacemesh/hare3"
)

type ProtocolRunner struct {
	messages chan MultiMsg
	handler  hare3.Handler
	protocol hare3.Protocol
	clock    RoundClock
	gossiper NetworkGossiper
}

func (r *ProtocolRunner) Run(ctx context.Context) ([]hare3.Hash20, error) {
	for {
		select {
		// We await the beginning of the round, which is achieved by calling AwaitEndOfRound with (round - 1).
		case <-r.clock.AwaitEndOfRound(uint32(r.protocol.Round() - 1)):
			toSend, output := r.protocol.NextRound()
			if toSend != nil {
				msg, err := buildEncodedOutputMessgae(toSend)
				if err != nil {
					// This should never happen
					panic(err)
				}
				r.gossiper.Gossip(msg)
			}
			if output != nil {
				return output, nil
			}
		case multiMsg := <-r.messages:
			// It is assumed that messages received here have had their
			// signature verified by the broker and that the broker made use of
			// the message sid to route the message to this instance.
			m := multiMsg.Message
			if r.handler.HandleMsg(m.key, m.values, m.round) {
				// Send raw message to peers
				err := r.gossiper.Gossip(multiMsg.RawMessage)
				if err != nil {
					logerr(err)
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type MultiMsg struct {
	RawMessage []byte
	Message    Msg
}

type Msg struct {
	sid, key []byte
	values   []hare3.Hash20
	round    int8
}

type NetworkGossiper interface {
	Gossip(msg []byte) error
}

func buildEncodedOutputMessgae(m *hare3.OutputMessage) ([]byte, error) {
	return nil, nil
}

func logerr(err error) {
}

// RoundClock is a timer interface.
type RoundClock interface {
	AwaitWakeup() <-chan struct{}
	// RoundEnd returns the time at which round ends, passing round-1 will
	// return the time at which round starts.
	RoundEnd(round uint32) time.Time
	AwaitEndOfRound(round uint32) <-chan struct{}
}
