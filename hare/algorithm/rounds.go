package algorithm

import (
	"math"

	"github.com/spacemeshos/go-spacemesh/log"
)

type round int8
type itter uint8
type roundCount uint32

const (
	preRound      = round(-1)
	statusRound   = round(0)
	proposalRound = round(1)
	commitRound   = round(2)
	notifyRound   = round(3)
)

func roundCountToRound(rc roundCount) round {
	if rc == math.MaxUint32 {
		return preRound
	}
	return round(rc % 4)
}
func roundToRoundCount(r round, itter uint32) roundCount {
	if r == preRound && itter > 0 {
		log.Panic("hare: preRound can only be run once; a preRound in "+
			"itteration %d is not possible", itter)
	} else if r == preRound && itter == 0 {
		return math.MaxUint32
	}
	return roundCount(itter*4 + uint32(r))
}
