package datastore

import (
	"errors"
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/spacemeshos/go-spacemesh/atxsdata"
	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/activesets"
	"github.com/spacemeshos/go-spacemesh/sql/atxs"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/blocks"
	"github.com/spacemeshos/go-spacemesh/sql/identities"
	"github.com/spacemeshos/go-spacemesh/sql/poets"
	"github.com/spacemeshos/go-spacemesh/sql/proposals"
	"github.com/spacemeshos/go-spacemesh/sql/transactions"
)

type VrfNonceKey struct {
	ID    types.NodeID
	Epoch types.EpochID
}

// CachedDB is simply a database injected with cache.
type CachedDB struct {
	*sql.Database
	logger log.Log

	// cache is optional
	atxsdata *atxsdata.Data

	atxHdrCache   *lru.Cache[types.ATXID, *types.ActivationTxHeader]
	vrfNonceCache *lru.Cache[VrfNonceKey, *types.VRFPostIndex]

	// used to coordinate db update and cache
	mu               sync.Mutex
	malfeasanceCache *lru.Cache[types.NodeID, *types.MalfeasanceProof]
}

type Config struct {
	// ATXSize must be larger than the sum of all ATXs in last 2 epochs to be effective
	// (Epoch 12: ~ 300k, Epoch 11: ~ 200k)
	ATXSize         int `mapstructure:"atx-size"`
	MalfeasanceSize int `mapstructure:"malfeasance-size"`
}

func DefaultConfig() Config {
	return Config{
		ATXSize:         1_000_000, // to be in line with fetch.EpochData size (see fetch/wire_types.go)
		MalfeasanceSize: 1_000,
	}
}

type cacheOpts struct {
	cfg      Config
	atxsdata *atxsdata.Data
}

type Opt func(*cacheOpts)

func WithConfig(cfg Config) Opt {
	return func(o *cacheOpts) {
		o.cfg = cfg
	}
}

func WithConsensusCache(c *atxsdata.Data) Opt {
	return func(o *cacheOpts) {
		o.atxsdata = c
	}
}

// NewCachedDB create an instance of a CachedDB.
func NewCachedDB(db *sql.Database, lg log.Log, opts ...Opt) *CachedDB {
	o := cacheOpts{cfg: DefaultConfig()}
	for _, opt := range opts {
		opt(&o)
	}
	lg.With().Info("initialized datastore", log.Any("config", o.cfg))

	atxHdrCache, err := lru.New[types.ATXID, *types.ActivationTxHeader](o.cfg.ATXSize)
	if err != nil {
		lg.Fatal("failed to create atx cache", err)
	}

	malfeasanceCache, err := lru.New[types.NodeID, *types.MalfeasanceProof](o.cfg.MalfeasanceSize)
	if err != nil {
		lg.Fatal("failed to create malfeasance cache", err)
	}

	vrfNonceCache, err := lru.New[VrfNonceKey, *types.VRFPostIndex](o.cfg.ATXSize)
	if err != nil {
		lg.Fatal("failed to create vrf nonce cache", err)
	}

	return &CachedDB{
		Database:         db,
		atxsdata:         o.atxsdata,
		logger:           lg,
		atxHdrCache:      atxHdrCache,
		malfeasanceCache: malfeasanceCache,
		vrfNonceCache:    vrfNonceCache,
	}
}

func (db *CachedDB) MalfeasanceCacheSize() int {
	return db.malfeasanceCache.Len()
}

// IsMalicious returns true if the NodeID is malicious.
func (db *CachedDB) IsMalicious(id types.NodeID) (bool, error) {
	if id == types.EmptyNodeID {
		db.logger.Fatal("invalid argument to IsMalicious")
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if proof, ok := db.malfeasanceCache.Get(id); ok {
		if proof == nil {
			return false, nil
		} else {
			return true, nil
		}
	}

	bad, err := identities.IsMalicious(db, id)
	if err != nil {
		return false, err
	}
	if !bad {
		db.malfeasanceCache.Add(id, nil)
	}
	return bad, nil
}

// GetMalfeasanceProof gets the malfeasance proof associated with the NodeID.
func (db *CachedDB) GetMalfeasanceProof(id types.NodeID) (*types.MalfeasanceProof, error) {
	if id == types.EmptyNodeID {
		db.logger.Fatal("invalid argument to GetMalfeasanceProof")
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if proof, ok := db.malfeasanceCache.Get(id); ok {
		if proof == nil {
			return nil, sql.ErrNotFound
		}
		return proof, nil
	}

	proof, err := identities.GetMalfeasanceProof(db.Database, id)
	if err != nil && err != sql.ErrNotFound {
		return nil, err
	}
	db.malfeasanceCache.Add(id, proof)
	return proof, err
}

func (db *CachedDB) CacheMalfeasanceProof(id types.NodeID, proof *types.MalfeasanceProof) {
	if id == types.EmptyNodeID {
		db.logger.Fatal("invalid argument to CacheMalfeasanceProof")
	}
	if db.atxsdata != nil {
		db.atxsdata.SetMalicious(id)
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	db.malfeasanceCache.Add(id, proof)
}

// VRFNonce returns the VRF nonce of for the given node in the given epoch. This function is thread safe and will
// return an error if the nonce is not found in the ATX DB.
func (db *CachedDB) VRFNonce(id types.NodeID, epoch types.EpochID) (types.VRFPostIndex, error) {
	key := VrfNonceKey{id, epoch}
	if nonce, ok := db.vrfNonceCache.Get(key); ok {
		return *nonce, nil
	}

	nonce, err := atxs.VRFNonce(db, id, epoch)
	if err != nil {
		return types.VRFPostIndex(0), err
	}

	db.vrfNonceCache.Add(key, &nonce)
	return nonce, nil
}

// GetAtxHeader returns the ATX header by the given ID. This function is thread safe and will return an error if the ID
// is not found in the ATX DB.
func (db *CachedDB) GetAtxHeader(id types.ATXID) (*types.ActivationTxHeader, error) {
	if id == types.EmptyATXID {
		return nil, errors.New("trying to fetch empty atx id")
	}

	if atxHeader, gotIt := db.atxHdrCache.Get(id); gotIt {
		return atxHeader, nil
	}

	return db.getAndCacheHeader(id)
}

// GetFullAtx returns the full atx struct of the given atxId id, it returns an error if the full atx cannot be found
// in all databases.
func (db *CachedDB) GetFullAtx(id types.ATXID) (*types.VerifiedActivationTx, error) {
	if id == types.EmptyATXID {
		return nil, errors.New("trying to fetch empty atx id")
	}

	atx, err := atxs.Get(db, id)
	if err != nil {
		return nil, fmt.Errorf("get ATXs from DB: %w", err)
	}

	db.atxHdrCache.Add(id, atx.ToHeader())
	return atx, nil
}

// getAndCacheHeader fetches the full atx struct from the database, caches it and returns the cached header.
func (db *CachedDB) getAndCacheHeader(id types.ATXID) (*types.ActivationTxHeader, error) {
	_, err := db.GetFullAtx(id)
	if err != nil {
		return nil, err
	}

	atxHeader, gotIt := db.atxHdrCache.Get(id)
	if !gotIt {
		return nil, fmt.Errorf("inconsistent state: failed to get atx header: %w", err)
	}

	return atxHeader, nil
}

// GetEpochWeight returns the total weight of ATXs targeting the given epochID.
func (db *CachedDB) GetEpochWeight(epoch types.EpochID) (uint64, []types.ATXID, error) {
	var (
		weight uint64
		ids    []types.ATXID
	)
	if err := db.IterateEpochATXHeaders(epoch, func(header *types.ActivationTxHeader) error {
		weight += header.GetWeight()
		ids = append(ids, header.ID)
		return nil
	}); err != nil {
		return 0, nil, err
	}
	return weight, ids, nil
}

// IterateEpochATXHeaders iterates over ActivationTxs that target an epoch.
func (db *CachedDB) IterateEpochATXHeaders(
	epoch types.EpochID,
	iter func(*types.ActivationTxHeader) error,
) error {
	ids, err := atxs.GetIDsByEpoch(db, epoch-1)
	if err != nil {
		return err
	}
	for _, id := range ids {
		header, err := db.GetAtxHeader(id)
		if err != nil {
			return err
		}
		if err := iter(header); err != nil {
			return err
		}
	}
	return nil
}

func (db *CachedDB) IterateMalfeasanceProofs(
	iter func(types.NodeID, *types.MalfeasanceProof) error,
) error {
	ids, err := identities.GetMalicious(db)
	if err != nil {
		return err
	}
	for _, id := range ids {
		proof, err := db.GetMalfeasanceProof(id)
		if err != nil {
			return err
		}
		if err := iter(id, proof); err != nil {
			return err
		}
	}
	return nil
}

// GetLastAtx gets the last atx header of specified node ID.
func (db *CachedDB) GetLastAtx(nodeID types.NodeID) (*types.ActivationTxHeader, error) {
	if atxid, err := atxs.GetLastIDByNodeID(db, nodeID); err != nil {
		return nil, fmt.Errorf("no prev atx found: %w", err)
	} else if atx, err := db.GetAtxHeader(atxid); err != nil {
		return nil, fmt.Errorf("inconsistent state: failed to get atx header: %w", err)
	} else {
		return atx, nil
	}
}

// GetEpochAtx gets the atx header of specified node ID published in the specified epoch.
func (db *CachedDB) GetEpochAtx(
	epoch types.EpochID,
	nodeID types.NodeID,
) (*types.ActivationTxHeader, error) {
	vatx, err := atxs.GetByEpochAndNodeID(db, epoch, nodeID)
	if err != nil {
		return nil, fmt.Errorf("no epoch atx found: %w", err)
	}
	header := vatx.ToHeader()
	db.atxHdrCache.Add(vatx.ID(), header)
	return header, nil
}

// IdentityExists returns true if this NodeID has published any ATX.
func (db *CachedDB) IdentityExists(nodeID types.NodeID) (bool, error) {
	_, err := atxs.GetLastIDByNodeID(db, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (db *CachedDB) MaxHeightAtx() (types.ATXID, error) {
	return atxs.GetIDWithMaxHeight(db, types.EmptyNodeID)
}

// Hint marks which DB should be queried for a certain provided hash.
type Hint string

// DB hints per DB.
const (
	BallotDB    Hint = "ballotDB"
	BlockDB     Hint = "blocksDB"
	ProposalDB  Hint = "proposalDB"
	ATXDB       Hint = "ATXDB"
	TXDB        Hint = "TXDB"
	POETDB      Hint = "POETDB"
	Malfeasance Hint = "malfeasance"
	ActiveSet   Hint = "activeset"
)

// NewBlobStore returns a BlobStore.
func NewBlobStore(db *sql.Database) *BlobStore {
	return &BlobStore{DB: db}
}

// BlobStore gets data as a blob to serve direct fetch requests.
type BlobStore struct {
	DB *sql.Database
}

// Get gets an ATX as bytes by an ATX ID as bytes.
func (bs *BlobStore) Get(hint Hint, key []byte) ([]byte, error) {
	switch hint {
	case ATXDB:
		return atxs.GetBlob(bs.DB, key)
	case ProposalDB:
		return proposals.GetBlob(bs.DB, key)
	case BallotDB:
		id := types.BallotID(types.BytesToHash(key).ToHash20())
		blt, err := ballots.Get(bs.DB, id)
		if err != nil {
			return nil, fmt.Errorf("get ballot blob: %w", err)
		}
		data, err := codec.Encode(blt)
		if err != nil {
			return data, fmt.Errorf("serialize: %w", err)
		}
		return data, nil
	case BlockDB:
		id := types.BlockID(types.BytesToHash(key).ToHash20())
		blk, err := blocks.Get(bs.DB, id)
		if err != nil {
			return nil, fmt.Errorf("get block: %w", err)
		}
		data, err := codec.Encode(blk)
		if err != nil {
			return data, fmt.Errorf("serialize: %w", err)
		}
		return data, nil
	case TXDB:
		return transactions.GetBlob(bs.DB, key)
	case POETDB:
		var ref types.PoetProofRef
		copy(ref[:], key)
		return poets.Get(bs.DB, ref)
	case Malfeasance:
		return identities.GetMalfeasanceBlob(bs.DB, key)
	case ActiveSet:
		return activesets.GetBlob(bs.DB, key)
	}
	return nil, fmt.Errorf("blob store not found %s", hint)
}
