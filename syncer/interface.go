package syncer

import (
	"context"
	"time"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/fetch"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/system"
)

//go:generate mockgen -typed -package=mocks -destination=./mocks/mocks.go -source=./interface.go

type layerTicker interface {
	CurrentLayer() types.LayerID
}

type meshProvider interface {
	SetZeroBlockLayer(context.Context, types.LayerID)
}

type activeSetCache interface {
	GetMissingActiveSet(types.EpochID, []types.ATXID) []types.ATXID
}

// fetchLogic is the interface between syncer and low-level fetching.
// it handles all data fetching related logic (for layer or for epoch, from all peers or from any random peer ...etc).
type fetchLogic interface {
	fetcher

	PollMaliciousProofs(ctx context.Context) error
	PollLayerData(context.Context, types.LayerID, ...p2p.Peer) error
	PollLayerOpinions(
		context.Context,
		types.LayerID,
		bool,
		[]p2p.Peer,
	) ([]*fetch.LayerOpinion, []*types.Certificate, error)
	GetEpochATXs(context.Context, types.EpochID) error
}

// fetcher is the interface to the low-level fetching.
type fetcher interface {
	GetMaliciousIDs(
		context.Context,
		[]p2p.Peer,
		func([]byte, p2p.Peer),
		func(error, p2p.Peer),
	) error
	GetLayerData(
		context.Context,
		[]p2p.Peer,
		types.LayerID,
		func([]byte, p2p.Peer),
		func(error, p2p.Peer),
	) error
	GetLayerOpinions(
		context.Context,
		[]p2p.Peer,
		types.LayerID,
		func([]byte, p2p.Peer),
		func(error, p2p.Peer),
	) error
	GetCert(context.Context, types.LayerID, types.BlockID, []p2p.Peer) (*types.Certificate, error)

	GetMalfeasanceProofs(context.Context, []types.NodeID) error
	system.AtxFetcher
	GetBallots(context.Context, []types.BallotID) error
	GetBlocks(context.Context, []types.BlockID) error
	RegisterPeerHashes(peer p2p.Peer, hashes []types.Hash32)

	SelectBestShuffled(int) []p2p.Peer
	PeerEpochInfo(context.Context, p2p.Peer, types.EpochID) (*fetch.EpochData, error)
	PeerMeshHashes(context.Context, p2p.Peer, *fetch.MeshHashRequest) (*fetch.MeshHashes, error)
}

type layerPatrol interface {
	IsHareInCharge(types.LayerID) bool
}

type certHandler interface {
	HandleSyncedCertificate(context.Context, types.LayerID, *types.Certificate) error
}

type forkFinder interface {
	AddResynced(types.LayerID, types.Hash32)
	NeedResync(types.LayerID, types.Hash32) bool
	UpdateAgreement(p2p.Peer, types.LayerID, types.Hash32, time.Time)
	FindFork(context.Context, p2p.Peer, types.LayerID, types.Hash32) (types.LayerID, error)
	Purge(bool, ...p2p.Peer)
}

type idProvider interface {
	IdentityExists(id types.NodeID) (bool, error)
}
