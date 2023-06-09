package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemeshos/go-spacemesh/hare3"
)

type ProtocolRunner struct {
	clock    RoundClock
	protocol hare3.Protocol
	maxRound hare3.AbsRound
	handler  *hare3.Handler
	messages chan MultiMsg
	gossiper NetworkGossiper
}

func NewProtocolRunner(
	clock RoundClock,
	protocol *hare3.Protocol,
	iterationLimit int8,
	handler *hare3.Handler,
	messages chan MultiMsg,
	gossiper NetworkGossiper,
) *ProtocolRunner {
	return &ProtocolRunner{
		clock:    clock,
		protocol: *protocol,
		maxRound: hare3.NewAbsRound(iterationLimit, 0),
		handler:  handler,
		messages: messages,
		gossiper: gossiper,
	}
}

// Run waits for successive rounds from the clock and handles messages received from the network
func (r *ProtocolRunner) Run(ctx context.Context) ([]hare3.Hash20, error) {
	for {
		if r.protocol.Round() == r.maxRound {
			return nil, fmt.Errorf("hare protocol runner exceeded iteration limit of %d", r.maxRound.Iteration())
		}
		select {
		// We await the beginning of the round, which is achieved by calling AwaitEndOfRound with (round - 1).
		case <-r.clock.AwaitEndOfRound(uint32(r.protocol.Round() - 1)):
			// This will need to be set per round to determine if this parcicipant is active in this round.
			var active bool
			toSend, output := r.protocol.NextRound(active)
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
			if r.handler.HandleMsg(m.Key, m.Values, m.Round) {
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
	Key    []byte
	Values []hare3.Hash20
	Round  int8
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
