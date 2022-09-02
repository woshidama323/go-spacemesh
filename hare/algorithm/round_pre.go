package algorithm

// tryParticipatePreRound, if eligible, will generate a preRoundMessage to be
// broadcasted to the network.
func tryParticipatePreRound(set, messageCache) (preRoundMessage, error) {
	return nil
}

// tryParticipateCommitRound, will attempt to generate a preRoundCertificate to
// be used in the safeValueProof. If successful, the preRoundCertificate will
// contain an acceptedSet. If unable to generate a preRoundCertificate,
// it will return nil.
func tryResolvePreRound(set, messageCache) (preRoundCertificate, error) {
	valueCertificates := make(map[setElement][]set)

	for _, v := range set.values {

	}
}
