package transactions

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/svm/transaction"
)

func createTX(t *testing.T, principal *signing.EdSigner, dest types.Address, nonce, amount, gas, fee uint64) *types.Transaction {
	t.Helper()
	tx, err := transaction.GenerateCallTransaction(principal, dest, nonce, amount, gas, fee)
	require.NoError(t, err)
	return tx
}

func packedInProposal(t *testing.T, db *sql.Database, tid types.TransactionID, lid types.LayerID, pid types.ProposalID, expectUpdated int) {
	t.Helper()
	dbtx, err := db.Tx(context.Background())
	require.NoError(t, err)
	defer dbtx.Release()

	require.NoError(t, AddToProposal(dbtx, tid, lid, pid))
	updated, err := UpdateIfBetter(dbtx, tid, lid, types.EmptyBlockID)
	require.NoError(t, err)
	require.Equal(t, expectUpdated, updated)
	require.NoError(t, dbtx.Commit())
}

func packedInBlock(t *testing.T, db *sql.Database, tid types.TransactionID, lid types.LayerID, bid types.BlockID, expectUpdated int) {
	t.Helper()
	dbtx, err := db.Tx(context.Background())
	require.NoError(t, err)
	defer dbtx.Release()

	require.NoError(t, AddToBlock(dbtx, tid, lid, bid))
	updated, err := UpdateIfBetter(dbtx, tid, lid, bid)
	require.NoError(t, err)
	require.Equal(t, expectUpdated, updated)
	require.NoError(t, dbtx.Commit())
}

func makeMeshTX(tx *types.Transaction, lid types.LayerID, bid types.BlockID, received time.Time, applied, discarded bool) *types.MeshTransaction {
	return &types.MeshTransaction{
		Transaction: *tx,
		LayerID:     lid,
		BlockID:     bid,
		Received:    received,
		Applied:     applied,
	}
}

func getAndCheckMeshTX(t *testing.T, db sql.Executor, tid types.TransactionID, expected *types.MeshTransaction) {
	t.Helper()
	got, err := Get(db, tid)
	require.NoError(t, err)
	checkMeshTXEqual(t, *expected, *got)
}

func checkMeshTXEqual(t *testing.T, expected, got types.MeshTransaction) {
	t.Helper()
	require.EqualValues(t, expected.Received.UnixNano(), got.Received.UnixNano())
	got.Received = time.Time{}
	expected.Received = time.Time{}
	require.Equal(t, expected, got)
}

func TestAddGetHas(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer1 := signing.NewEdSignerFromRand(rng)
	signer2 := signing.NewEdSignerFromRand(rng)
	txs := []*types.Transaction{
		createTX(t, signer1, types.Address{1}, 1, 191, 1, 1),
		createTX(t, signer2, types.Address{2}, 1, 191, 1, 1),
		createTX(t, signer1, types.Address{3}, 1, 191, 1, 1),
	}

	received := time.Now()
	for _, tx := range txs {
		require.NoError(t, Add(db, tx, received))
	}

	for _, tx := range txs {
		got, err := Get(db, tx.ID())
		require.NoError(t, err)
		expected := makeMeshTX(tx, types.LayerID{}, types.EmptyBlockID, received, false, false)
		checkMeshTXEqual(t, *expected, *got)

		has, err := Has(db, tx.ID())
		require.NoError(t, err)
		require.True(t, has)
	}

	tid := types.RandomTransactionID()
	_, err := Get(db, tid)
	require.ErrorIs(t, err, sql.ErrNotFound)

	has, err := Has(db, tid)
	require.NoError(t, err)
	require.False(t, has)
}

func TestAddToProposal(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer := signing.NewEdSignerFromRand(rng)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1, 1)
	require.NoError(t, Add(db, tx, time.Now()))

	lid := types.NewLayerID(10)
	pid := types.ProposalID{1, 1}
	require.NoError(t, AddToProposal(db, tx.ID(), lid, pid))

	has, err := HasProposalTx(db, pid, tx.ID())
	require.NoError(t, err)
	require.True(t, has)

	has, err = HasProposalTx(db, types.ProposalID{2, 2}, tx.ID())
	require.NoError(t, err)
	require.False(t, has)
}

func TestAddToBlock(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer := signing.NewEdSignerFromRand(rng)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1, 1)
	require.NoError(t, Add(db, tx, time.Now()))

	lid := types.NewLayerID(10)
	bid := types.BlockID{1, 1}
	require.NoError(t, AddToBlock(db, tx.ID(), lid, bid))

	has, err := HasBlockTx(db, bid, tx.ID())
	require.NoError(t, err)
	require.True(t, has)

	has, err = HasBlockTx(db, types.BlockID{2, 2}, tx.ID())
	require.NoError(t, err)
	require.False(t, has)
}

func TestApplyUnapply(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer := signing.NewEdSignerFromRand(rng)
	tx1 := createTX(t, signer, types.Address{1}, 1, 191, 1, 1)
	received := time.Now()
	require.NoError(t, Add(db, tx1, received))
	expected := makeMeshTX(tx1, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx1.ID(), expected)

	lid1 := types.NewLayerID(10)
	bid1 := types.BlockID{1, 1}
	updated, err := Apply(db, tx1.ID(), lid1, bid1)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected = makeMeshTX(tx1, lid1, bid1, received, true, false)
	getAndCheckMeshTX(t, db, tx1.ID(), expected)

	tx2 := createTX(t, signer, types.Address{1}, 2, 191, 1, 1)
	require.NoError(t, Add(db, tx2, received))
	lid2 := lid1.Add(1)
	bid2 := types.BlockID{2, 2}
	updated, err = Apply(db, tx2.ID(), lid2, bid2)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected = makeMeshTX(tx2, lid2, bid2, received, true, false)
	getAndCheckMeshTX(t, db, tx2.ID(), expected)

	tx3 := createTX(t, signer, types.Address{1}, 3, 191, 1, 1)
	require.NoError(t, Add(db, tx3, received))
	lid3 := lid1.Add(2)
	bid3 := types.BlockID{3, 3}
	updated, err = Apply(db, tx3.ID(), lid3, bid3)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected = makeMeshTX(tx3, lid3, bid3, received, true, false)
	getAndCheckMeshTX(t, db, tx3.ID(), expected)

	require.NoError(t, UndoLayers(db, lid2, lid1))
	expected = makeMeshTX(tx1, lid1, bid1, received, true, false)
	getAndCheckMeshTX(t, db, tx1.ID(), expected)

	expected = makeMeshTX(tx2, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx2.ID(), expected)

	expected = makeMeshTX(tx3, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx3.ID(), expected)

	//updated, err = Unapply(db, tx.ID())
	//require.NoError(t, err)
	//require.Equal(t, 1, updated)
	//expected = makeMeshTX(tx, types.LayerID{}, types.EmptyBlockID, received, false, false)
	//getAndCheckMeshTX(t, db, tx.ID(), expected)
}

func TestUpdateLayerAndBlock(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer := signing.NewEdSignerFromRand(rng)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1, 1)
	received := time.Now()
	require.NoError(t, Add(db, tx, received))
	expected := makeMeshTX(tx, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	lid := types.NewLayerID(10)
	updated, err := UpdateIfBetter(db, tx.ID(), lid, types.EmptyBlockID)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected = makeMeshTX(tx, lid, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	updated, err = UpdateIfBetter(db, tx.ID(), lid.Add(1), types.EmptyBlockID)
	require.NoError(t, err)
	require.Equal(t, 0, updated)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	updated, err = UpdateIfBetter(db, tx.ID(), lid.Sub(1), types.EmptyBlockID)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected.LayerID = lid.Sub(1)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	bid := types.BlockID{3, 4, 5}
	updated, err = UpdateIfBetter(db, tx.ID(), types.NewLayerID(9), bid)
	require.NoError(t, err)
	require.Equal(t, 1, updated)
	expected.BlockID = bid
	getAndCheckMeshTX(t, db, tx.ID(), expected)
}

func TestSetNextLayer(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer := signing.NewEdSignerFromRand(rng)
	tx := createTX(t, signer, types.Address{1}, 1, 191, 1, 1)
	received := time.Now()
	require.NoError(t, Add(db, tx, received))
	expected := makeMeshTX(tx, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	lid9 := types.NewLayerID(9)
	lid10 := types.NewLayerID(10)
	lid11 := types.NewLayerID(11)
	lid12 := types.NewLayerID(12)
	p9 := types.RandomProposalID()
	p10 := types.RandomProposalID()
	p11 := types.RandomProposalID()
	p12 := types.RandomProposalID()
	packedInProposal(t, db, tx.ID(), lid9, p9, 1)
	packedInProposal(t, db, tx.ID(), lid10, p10, 0)
	packedInProposal(t, db, tx.ID(), lid11, p11, 0)
	packedInProposal(t, db, tx.ID(), lid12, p12, 0)
	expected = makeMeshTX(tx, lid9, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	b10 := types.RandomBlockID()
	b11 := types.RandomBlockID()
	packedInBlock(t, db, tx.ID(), lid10, b10, 0)
	packedInBlock(t, db, tx.ID(), lid11, b11, 0)
	expected = makeMeshTX(tx, lid9, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	next, bid, err := SetNextLayer(db, tx.ID(), lid9)
	require.NoError(t, err)
	require.Equal(t, lid10, next)
	require.Equal(t, b10, bid)
	expected = makeMeshTX(tx, lid10, b10, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	next, bid, err = SetNextLayer(db, tx.ID(), types.NewLayerID(10))
	require.NoError(t, err)
	require.Equal(t, lid11, next)
	require.Equal(t, b11, bid)
	expected = makeMeshTX(tx, lid11, b11, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	next, bid, err = SetNextLayer(db, tx.ID(), types.NewLayerID(11))
	require.NoError(t, err)
	require.Equal(t, lid12, next)
	require.Equal(t, types.EmptyBlockID, bid)
	expected = makeMeshTX(tx, lid12, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)

	next, bid, err = SetNextLayer(db, tx.ID(), types.NewLayerID(12))
	require.NoError(t, err)
	require.Equal(t, types.LayerID{}, next)
	require.Equal(t, types.EmptyBlockID, bid)
	expected = makeMeshTX(tx, types.LayerID{}, types.EmptyBlockID, received, false, false)
	getAndCheckMeshTX(t, db, tx.ID(), expected)
}

// mempool->proposal->block->applied
// mempool->proposal->block->mempool.
//func TestTXHappyFlow(t *testing.T) {
//	db := sql.InMemory()
//
//	rng := rand.New(rand.NewSource(1001))
//	tx := createTX(t, signing.NewEdSignerFromRand(rng), types.Address{1}, 1, 191, 1, 1)
//	lid := types.NewLayerID(10)
//
//	require.NoError(t, Add(db, tx))
//	getAndCheckMeshTX(t, db, tx, types.LayerID{}, types.EmptyBlockID, false)
//
//	pid := types.ProposalID{1, 1}
//	packedInProposal(t, db, tx.ID(), lid, pid, 1)
//	getAndCheckMeshTX(t, db, tx, types.LayerID{}, types.EmptyBlockID, false)
//
//	bid := types.BlockID{1, 1}
//	packedInBlock(t, db, tx.ID(), lid, bid, 0)
//	getAndCheckMeshTX(t, db, tx, lid, bid, false)
//
//	// bid is applied
//	applyBlock(t, db, tx.ID(), lid, bid, 1)
//}
//
//func TestUpdateForBlockAndProposal(t *testing.T) {
//	db := sql.InMemory()
//
//	rng := rand.New(rand.NewSource(1001))
//	tx := createTX(t, signing.NewEdSignerFromRand(rng), types.Address{1}, 1, 191, 1, 1)
//	require.NoError(t, Add(db, tx))
//	getAndCheckMeshTX(t, db, tx, types.LayerID{}, types.EmptyBlockID, false)
//
//	lid := types.NewLayerID(10)
//	bid := types.RandomBlockID()
//	packedInBlock(t, db, tx.ID(), lid, bid, 1)
//	getAndCheckMeshTX(t, db, tx, lid, bid, false)
//
//	// tx is in a proposal too
//	pid := types.RandomProposalID()
//	packedInProposal(t, db, tx.ID(), lid, pid, 0)
//	getAndCheckMeshTX(t, db, tx, lid, bid, false)
//}
//
//func TestUpdateForBlocksInDiffLayers(t *testing.T) {
//	db := sql.InMemory()
//
//	rng := rand.New(rand.NewSource(1001))
//	tx := createTX(t, signing.NewEdSignerFromRand(rng), types.Address{1}, 1, 191, 1, 1)
//	require.NoError(t, Add(db, tx))
//	getAndCheckMeshTX(t, db, tx, types.LayerID{}, types.EmptyBlockID, false)
//
//	lid1 := types.NewLayerID(10)
//	bid1 := types.RandomBlockID()
//	packedInBlock(t, db, tx.ID(), lid1, bid1, 1)
//	getAndCheckMeshTX(t, db, tx, lid1, bid1, false)
//
//	lid2 := lid1.Sub(1)
//	bid2 := types.RandomBlockID()
//	packedInBlock(t, db, tx.ID(), lid2, bid2, 0)
//	// earlier layer block should be retrieved
//	getAndCheckMeshTX(t, db, tx, lid2, bid2, false)
//}

func TestGetBlob(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	tx := createTX(t, signing.NewEdSignerFromRand(rng), types.Address{1}, 1, 191, 1, 1)

	require.NoError(t, Add(db, tx, time.Now()))
	buf, err := GetBlob(db, tx.ID())
	require.NoError(t, err)
	encoded, err := codec.Encode(tx)
	require.NoError(t, err)
	require.Equal(t, encoded, buf)
}

func TestGetByAddress(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer1 := signing.NewEdSignerFromRand(rng)
	signer2 := signing.NewEdSignerFromRand(rng)
	signer2Address := types.BytesToAddress(signer2.PublicKey().Bytes())
	lid := types.NewLayerID(10)
	pid := types.ProposalID{1, 1}
	bid := types.BlockID{2, 2}
	txs := []*types.Transaction{
		createTX(t, signer1, types.Address{1}, 1, 191, 1, 1),
		createTX(t, signer2, types.Address{2}, 1, 191, 1, 1),
		createTX(t, signer1, signer2Address, 1, 191, 1, 1),
	}
	received := time.Now()
	for _, tx := range txs {
		require.NoError(t, Add(db, tx, received))
		packedInProposal(t, db, tx.ID(), lid, pid, 1)
	}
	packedInBlock(t, db, txs[2].ID(), lid, bid, 1)

	// should be nothing before lid
	got, err := GetByAddress(db, types.NewLayerID(1), lid.Sub(1), signer2Address)
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = GetByAddress(db, types.LayerID{}, lid, signer2Address)
	require.NoError(t, err)
	require.Len(t, got, 2)
	expected1 := makeMeshTX(txs[1], lid, types.EmptyBlockID, received, false, false)
	checkMeshTXEqual(t, *expected1, *got[0])
	expected2 := makeMeshTX(txs[2], lid, bid, received, false, false)
	checkMeshTXEqual(t, *expected2, *got[1])
}

func TestGetAllPending(t *testing.T) {
	db := sql.InMemory()

	rng := rand.New(rand.NewSource(1001))
	signer1 := signing.NewEdSignerFromRand(rng)
	signer2 := signing.NewEdSignerFromRand(rng)
	txs := []*types.Transaction{
		createTX(t, signer1, types.Address{1}, 1, 191, 1, 1),
		createTX(t, signer2, types.Address{2}, 1, 191, 1, 1),
		createTX(t, signer1, types.Address{3}, 1, 191, 1, 1),
	}

	received := time.Now()
	for i, tx := range txs {
		require.NoError(t, Add(db, tx, received.Add(time.Duration(i))))
	}

	got, err := GetAllPending(db)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for _, tx := range got {
		require.Equal(t, types.LayerID{}, tx.LayerID)
		require.Equal(t, types.EmptyBlockID, tx.BlockID)
	}

	lid := types.NewLayerID(10)
	bid := types.BlockID{2, 2}
	for i, tx := range txs {
		packedInBlock(t, db, tx.ID(), lid.Add(uint32(i)), types.EmptyBlockID, 1)
	}

	got, err = GetAllPending(db)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i, tx := range got {
		expected := makeMeshTX(txs[i], lid.Add(uint32(i)), types.EmptyBlockID, received.Add(time.Duration(i)), false, false)
		checkMeshTXEqual(t, *expected, *tx)
	}

	// apply tx
	updated, err := Apply(db, txs[0].ID(), lid, bid)
	require.NoError(t, err)
	require.Equal(t, 1, updated)

	expected := makeMeshTX(txs[0], lid, bid, received, true, false)
	getAndCheckMeshTX(t, db, txs[0].ID(), expected)
	got, err = GetAllPending(db)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for i, tx := range got {
		expected = makeMeshTX(txs[i+1], lid.Add(uint32(i+1)), types.EmptyBlockID, received.Add(time.Duration(i+1)), false, false)
		checkMeshTXEqual(t, *expected, *tx)
	}
}
