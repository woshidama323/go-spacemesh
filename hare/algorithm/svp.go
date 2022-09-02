package algorithm

// baseSafeValueProof is the type of SVP that is used when no commitCertificate is available.
// Until f+1 eligible nodes have committed to a proposed set, the leader of an
// itteration must depend on nodes having seen all the preRound messages.
type baseSafeValueProof struct {
}

// nextSafeValueProof is the type of SVP that is used when there is a commitCertificate
// available. f+1 nodes have already committed to a proposed set but the notify round
// has failed, so the leader of an itteration can depend on the commitCertificate to
// construct the safeValueProof.
type nextSafeValueProof struct {
}
