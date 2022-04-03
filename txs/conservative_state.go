package txs

import (
	"fmt"
	"time"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/database"
	"github.com/spacemeshos/go-spacemesh/events"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/sql"
)

// ConservativeState provides the conservative version of the SVM state by taking into accounts of
// nonce and balances for pending transactions in un-applied blocks and mempool.
type ConservativeState struct {
	svmState
	*cache

	logger log.Log
	store  txProvider
	//cache  *cache
}

// NewConservativeState returns a ConservativeState.
func NewConservativeState(state svmState, db *sql.Database, logger log.Log) *ConservativeState {
	s := newStore(db, logger)
	return &ConservativeState{
		svmState: state,
		cache:    newCache(s, state, logger),
		store:    s,
		logger:   logger,
	}
}

func (cs *ConservativeState) getState(addr types.Address) (uint64, uint64) {
	return cs.svmState.GetNonce(addr), cs.svmState.GetBalance(addr)
}

// SelectTXsForProposal picks a specific numbers of transactions to pack in a proposal.
// Transactions are picked based on gas prices, from highest to lowest.
func (cs *ConservativeState) SelectTXsForProposal(numTXs int) ([]types.TransactionID, error) {
	mi, err := newMempoolIterator(cs.logger, cs.cache, numTXs)
	if err != nil {
		return nil, err
	}
	return mi.PopAll(), nil
}

// GetProjection returns the projected nonce and balance for the address with pending transactions
// in un-applied blocks and mempool.
//func (cs *ConservativeState) GetProjection(addr types.Address) (uint64, uint64, error) {
//	// TODO return error if cache is not yet active
//	nonce, balance := cs.cache.GetProjection(addr)
//	return nonce, balance, nil
//}

// HasTx returns true if we already have this transaction in tp.
func (cs *ConservativeState) HasTx(tid types.TransactionID) (bool, error) {
	return cs.store.Has(tid)
}

// AddToCache adds the provided transaction to the conservative cache.
func (cs *ConservativeState) AddToCache(tx *types.Transaction, newTX bool) error {
	received := time.Now()
	// save all new transactions as long as they are syntactically correct
	if newTX {
		if err := cs.store.Add(tx, received); err != nil {
			return err
		}
		events.ReportNewTx(types.LayerID{}, tx)
		events.ReportAccountUpdate(tx.Origin())
		events.ReportAccountUpdate(tx.GetRecipient())
	}

	return cs.cache.Add(tx, received)
}

//func (cs *ConservativeState) LinkTXsWithProposal(lid types.LayerID, pid types.ProposalID, tids []types.TransactionID) error {
//	if len(tids) == 0 {
//		return nil
//	}
//	if err := cs.store.AddToProposal(lid, pid, tids); err != nil {
//		return err
//	}
//	return cs.cache.AddToLayer(lid, types.EmptyBlockID, tids)
//}
//
//func (cs *ConservativeState) LinkTXsWithBlock(lid types.LayerID, bid types.BlockID, tids []types.TransactionID) error {
//	if len(tids) == 0 {
//		return nil
//	}
//	if err := cs.store.AddToBlock(lid, bid, tids); err != nil {
//		return err
//	}
//	return cs.cache.AddToLayer(lid, bid, tids)
//}

// GetMeshTransaction retrieves a tx by its id.
func (cs *ConservativeState) GetMeshTransaction(tid types.TransactionID) (*types.MeshTransaction, error) {
	return cs.store.Get(tid)
}

// GetMeshTransactions retrieves a list of layerTXs by their id's.
func (cs *ConservativeState) GetMeshTransactions(ids []types.TransactionID) ([]*types.MeshTransaction, map[types.TransactionID]struct{}) {
	missing := make(map[types.TransactionID]struct{})
	txs := make([]*types.MeshTransaction, 0, len(ids))
	// TODO: optimize by querying multiple IDs at once
	for _, tid := range ids {
		if mtx, err := cs.GetMeshTransaction(tid); err != nil {
			cs.logger.With().Warning("could not get tx", tid, log.Err(err))
			missing[tid] = struct{}{}
		} else {
			txs = append(txs, mtx)
		}
	}
	return txs, missing
}

// GetTransactionsByAddress retrieves layerTXs for a single address in between layers [from, to].
// Guarantees that transaction will appear exactly once, even if origin and recipient is the same, and in insertion order.
func (cs *ConservativeState) GetTransactionsByAddress(from, to types.LayerID, address types.Address) ([]*types.MeshTransaction, error) {
	return cs.store.GetByAddress(from, to, address)
}

func (cs *ConservativeState) RevertState(revertTo types.LayerID) (types.Hash32, error) {
	root, err := cs.svmState.Rewind(revertTo)
	if err != nil {
		return root, fmt.Errorf("svm rewind to %v: %w", revertTo, err)
	}

	return root, cs.cache.RevertToLayer(revertTo)
}

// ApplyLayer applies the transactions specified by the ids to the state.
func (cs *ConservativeState) ApplyLayer(toApply *types.Block) ([]*types.Transaction, error) {
	logger := cs.logger.WithFields(toApply.LayerIndex, toApply.ID())
	logger.Info("applying layer to conservative state")
	txs, err := cs.getTXsToApply(toApply)
	if err != nil {
		return nil, err
	}

	rewardByMiner := map[types.Address]uint64{}
	for _, r := range toApply.Rewards {
		rewardByMiner[r.Address] += r.Amount
	}

	// TODO: should miner IDs be sorted in a deterministic order prior to applying rewards?
	failedTxs, svmErr := cs.svmState.ApplyLayer(toApply.LayerIndex, txs, rewardByMiner)
	if svmErr != nil {
		logger.With().Error("failed to apply layerTXs",
			toApply.LayerIndex,
			log.Int("num_failed_txs", len(failedTxs)),
			log.Err(svmErr))
		// TODO: We want to panic here once we have a way to "remember" that we didn't apply these layerTXs
		//  e.g. persist the last layer transactions were applied from and use that instead of `oldVerified`
		return failedTxs, fmt.Errorf("apply layer: %w", svmErr)
	}

	if err = cs.cache.ApplyLayer(toApply.LayerIndex, toApply.ID(), txs); err != nil {
		return failedTxs, err
	}
	return failedTxs, nil
}

func (cs *ConservativeState) getTXsToApply(toApply *types.Block) ([]*types.Transaction, error) {
	mtxs, missing := cs.GetMeshTransactions(toApply.TxIDs)
	if len(missing) > 0 {
		return nil, fmt.Errorf("find layerTXs %v for applying layer %v", missing, toApply.LayerIndex)
	}

	txs := make([]*types.Transaction, 0, len(mtxs))
	for _, mtx := range mtxs {
		// some TXs in the block may be already applied previously
		if mtx.State == types.APPLIED {
			continue
		}
		txs = append(txs, &mtx.Transaction)
	}

	return txs, nil
}

// Transactions exports the transactions DB.
func (cs *ConservativeState) Transactions() database.Getter {
	return &txFetcher{store: cs.store}
}

type txFetcher struct {
	store txProvider
}

// Get transaction blob, by transaction id.
func (f *txFetcher) Get(hash []byte) ([]byte, error) {
	id := types.TransactionID{}
	copy(id[:], hash)
	return f.store.GetBlob(id)
}
