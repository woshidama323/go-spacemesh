package algorithm

type message interface {
	Bytes() []byte
}

type roundMessageHeader struct {
	Round int
}
type roundMessage struct {
	Header roundMessageHeader
	Body   []byte
}
type signedRoundMessage struct {
	Message   []byte
	Signature []byte
}

type preRoundMessage struct {
	inputSet set
}
type statusRoundMessage struct {
	acceptedSet set
}
type proposalRoundMessage struct {
	proposedSet set
}
type commitRoundMessage struct {
	set set
}
type notifyRoundMessage struct {
	set set
}
