package algorithm

// preRoundCertificate is the certificate used in the construction of the
// safeValueProof when a commitCertificate is not available.
type preRoundCertificate struct {
}

// commitCertificate proves f+1 eligible nodes committed to a proposed set.
// A notifyRound participant must only broadcast a notify message once it has
// obtained or constructed a valid commitCertificate.
// It is also used in the construction of the safeValueProof once available.
type commitCertificate struct {
}

func (c *commitCertificate) itteration() {

}
