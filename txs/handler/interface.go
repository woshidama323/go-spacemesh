package handler

import (
	"github.com/spacemeshos/go-spacemesh/common/types"
)

//go:generate mockgen -package=mocks -destination=./mocks/mocks.go -source=./interface.go

type conservativeState interface {
	HasTx(types.TransactionID) (bool, error)
	AddressExists(types.Address) bool
	AddToCache(tx *types.Transaction, newTX bool) error
}
