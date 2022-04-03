package txs

import (
	"context"
	"fmt"
	"time"

	"github.com/spacemeshos/go-spacemesh/sql/layers"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/transactions"
)

type store struct {
	logger log.Log
	db     *sql.Database
}

func newStore(db *sql.Database, logger log.Log) *store {
	return &store{
		logger: logger,
		db:     db,
	}
}

func (s *store) LastAppliedLayer() (types.LayerID, error) {
	return layers.GetLastApplied(s.db)
}

// Add adds a transaction to the database.
func (s *store) Add(tx *types.Transaction, received time.Time) error {
	return transactions.Add(s.db, tx, received)
}

// Has returns true if a transaction already exists in the database.
func (s *store) Has(tid types.TransactionID) (bool, error) {
	return transactions.Has(s.db, tid)
}

// Get returns a transaction from the database.
func (s *store) Get(tid types.TransactionID) (*types.MeshTransaction, error) {
	return transactions.Get(s.db, tid)
}

// GetBlob returns a transaction as a byte array.
func (s *store) GetBlob(tid types.TransactionID) ([]byte, error) {
	return transactions.GetBlob(s.db, tid)
}

// GetByAddress returns a list of transactions from `address` with layers in [from, to].
func (s *store) GetByAddress(from, to types.LayerID, address types.Address) ([]*types.MeshTransaction, error) {
	return transactions.GetByAddress(s.db, from, to, address)
}

// AddToProposal adds a transaction to a proposal in the database.
func (s *store) AddToProposal(lid types.LayerID, pid types.ProposalID, tids []types.TransactionID) error {
	dbtx, err := s.db.Tx(context.Background())
	if err != nil {
		return err
	}
	defer dbtx.Release()
	for _, tid := range tids {
		if err = transactions.AddToProposal(dbtx, tid, lid, pid); err != nil {
			return err
		}
		_, err = transactions.UpdateIfBetter(dbtx, tid, lid, types.EmptyBlockID)
		if err != nil {
			return err
		}
	}
	return dbtx.Commit()
}

// AddToBlock adds a transaction to a block in the database.
func (s *store) AddToBlock(lid types.LayerID, bid types.BlockID, tids []types.TransactionID) error {
	dbtx, err := s.db.Tx(context.Background())
	if err != nil {
		return err
	}
	defer dbtx.Release()
	for _, tid := range tids {
		if err = transactions.AddToBlock(dbtx, tid, lid, bid); err != nil {
			return err
		}
		_, err = transactions.UpdateIfBetter(dbtx, tid, lid, bid)
		if err != nil {
			return err
		}
	}
	return dbtx.Commit()
}

// ApplyLayer sets transactions to applied and discarded accordingly, and sets the layer at which the
// transactions are applied/discarded.
func (s *store) ApplyLayer(lid types.LayerID, bid types.BlockID, addr types.Address, appliedByNonce map[uint64]types.TransactionID) error {
	dbtx, err := s.db.Tx(context.Background())
	if err != nil {
		return err
	}
	defer dbtx.Release()
	// nonce order doesn't matter here
	for nonce, tid := range appliedByNonce {
		updated, err := transactions.Apply(dbtx, tid, lid, bid)
		if err != nil {
			return err
		}
		if updated == 0 {
			s.logger.With().Error("unexpected update for apply", tid, bid)
		}
		if err = transactions.DiscardByAcctNonce(dbtx, tid, lid, addr, nonce); err != nil {
			s.logger.With().Error("failed to discard txs",
				addr,
				log.Uint64("nonce", nonce))
			return err
		}
	}
	return dbtx.Commit()
}

// DiscardNonceBelow discards pending transactions with nonce lower than `nonce`.
func (s *store) DiscardNonceBelow(addr types.Address, nonce uint64) error {
	return transactions.DiscardNonceBelow(s.db, addr, nonce)
}

// UndoLayers resets all transactions that were applied/discarded between `from` and the most recent layer,
// and reset their layers if they were included in a proposal/block
func (s *store) UndoLayers(from types.LayerID) error {
	dbtx, err := s.db.Tx(context.Background())
	if err != nil {
		return err
	}
	defer dbtx.Release()

	tids, err := transactions.UndoLayers(s.db, from)
	if err != nil {
		return fmt.Errorf("undo %w", err)
	}
	for _, tid := range tids {
		if _, _, err := transactions.SetNextLayer(s.db, tid, from.Sub(1)); err != nil {
			return fmt.Errorf("reset for undo %w", err)
		}
	}
	return dbtx.Commit()
}

// SetNextLayerBlock sets and returns the next applicable layer/block for the transaction.
func (s *store) SetNextLayerBlock(tid types.TransactionID, lid types.LayerID) (types.LayerID, types.BlockID, error) {
	return transactions.SetNextLayer(s.db, tid, lid)
}

// GetAllPending gets all pending transactions for all accounts from database.
func (s *store) GetAllPending() ([]*types.MeshTransaction, error) {
	return transactions.GetAllPending(s.db)
}

// GetAcctPendingAtNonce gets all pending transactions with nonce == `nonce` for an account.
//func (s *store) GetAcctPendingAtNonce(addr types.Address, nonce uint64) ([]*types.MeshTransaction, error) {
//	return transactions.GetAcctPendingAtNonce(s.db, addr, nonce)
//}

// GetAcctPendingFromNonce gets all pending transactions with nonce <= `from` for an account.
func (s *store) GetAcctPendingFromNonce(addr types.Address, from uint64) ([]*types.MeshTransaction, error) {
	return transactions.GetAcctPendingFromNonce(s.db, addr, from)
}
