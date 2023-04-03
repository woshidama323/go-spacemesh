package hare

import (
	"github.com/spacemeshos/go-spacemesh/common/types"
)

type msgsTracker interface {
	Track(*Msg)
	NodeID(*Message) types.NodeID
}

type defaultMsgsTracker struct {
	sigToPub map[types.EdSignature]types.NodeID
}

func (mt *defaultMsgsTracker) Track(m *Msg) {
	println("real track")
	mt.sigToPub[m.Signature] = m.NodeID
}

func (mt *defaultMsgsTracker) NodeID(m *Message) types.NodeID {
	return mt.sigToPub[m.Signature]
}

func newMsgsTracker() *defaultMsgsTracker {
	return &defaultMsgsTracker{sigToPub: make(map[types.EdSignature]types.NodeID)}
}
