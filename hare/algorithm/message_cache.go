package algorithm

type itterationCache struct {
	preRoundMessages      []*preRoundMessage
	statusRoundMessages   []*statusRoundMessage
	proposalRoundMessages []*proposalRoundMessage
	commitRoundMessages   []*commitRoundMessage
	notifyRoundMessages   []*notifyRoundMessage
}

// messageCache stores all messages received in a given consensus process.
type messageCache struct {
	messages map[uint]itterationCache
}
