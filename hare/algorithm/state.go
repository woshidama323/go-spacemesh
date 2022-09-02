package algorithm

type state struct {
	inputSet    set
	acceptedSet set
	// preRoundCert should only be set once. It should be available after preRound
	// messages are received.
	preRoundCert *preRoundCertificate
	// commitCert is the newest commitCertificate of which this node is aware.
	commitCert *commitCertificate
}

func (s *state) greatestCertifiedItteration() int {
	if s.commitCert == nil {
		return -1
	}
	return s.commitCert.itteration()
}
