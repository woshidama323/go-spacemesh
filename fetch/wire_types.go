package fetch

import (
	"fmt"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/p2p"
)

//go:generate scalegen

const MaxHashesInReq = 100

// RequestMessage is sent to the peer for hash query.
type RequestMessage struct {
	Hint datastore.Hint `scale:"max=256"` // TODO(mafa): covert to an enum
	Hash types.Hash32
}

// ResponseMessage is sent to the node as a response.
type ResponseMessage struct {
	Hash types.Hash32
	Data []byte `scale:"max=20971520"` // limit to 20 MiB
}

// RequestBatch is a batch of requests and a hash of all requests as ID.
type RequestBatch struct {
	ID types.Hash32
	// depends on fetch config `BatchSize` which defaults to 10, more than 100 seems unlikely
	Requests []RequestMessage `scale:"max=100"`
}

// ResponseBatch is the response struct send for a RequestBatch. the ResponseBatch ID must be the same
// as stated in RequestBatch even if not all Data is present.
type ResponseBatch struct {
	ID types.Hash32
	// depends on fetch config `BatchSize` which defaults to 10, more than 100 seems unlikely
	Responses []ResponseMessage `scale:"max=100"`
}

// MeshHashRequest is used by ForkFinder to request the hashes of layers from
// a peer to find the layer at which a divergence occurred in the local mesh of
// the node.
//
// From and To define the beginning and end layer to request. By defines the
// increment between layers to limit the number of hashes in the response,
// e.g. 2 means only every other hash is requested. The number of hashes in
// the response is limited by `fetch.MaxHashesInReq`.
type MeshHashRequest struct {
	From, To types.LayerID
	Step     uint32
}

func NewMeshHashRequest(from, to types.LayerID) *MeshHashRequest {
	diff := to.Difference(from)
	delta := diff/uint32(MaxHashesInReq-1) + 1
	return &MeshHashRequest{
		From: from,
		To:   to,
		Step: delta,
	}
}

func (r *MeshHashRequest) Count() uint {
	diff := r.To.Difference(r.From)
	count := uint(diff/r.Step + 1)
	if diff%r.Step != 0 {
		// last layer is not a multiple of Step size, so we need to add it
		count++
	}
	return count
}

func (r *MeshHashRequest) Validate() error {
	if r.Step == 0 {
		return fmt.Errorf("%w: By must not be zero", errBadRequest)
	}

	if r.To.Before(r.From) {
		return fmt.Errorf("%w: To before From", errBadRequest)
	}

	if r.Count() > MaxHashesInReq {
		return fmt.Errorf("%w: number of layers requested exceeds maximum for one request", errBadRequest)
	}
	return nil
}

func (r *MeshHashRequest) MarshalLogObject(encoder log.ObjectEncoder) error {
	encoder.AddUint32("from", r.From.Uint32())
	encoder.AddUint32("to", r.To.Uint32())
	encoder.AddUint32("by", r.Step)
	return nil
}

type MeshHashes struct {
	// depends on syncer Config `MaxHashesInReq`, defaults to 100, 1000 is a safe upper bound
	Hashes []types.Hash32 `scale:"max=1000"`
}

type MaliciousIDs struct {
	NodeIDs []types.NodeID `scale:"max=100000"` // max. expected number of ATXs per epoch is 100_000
}

type EpochData struct {
	AtxIDs []types.ATXID `scale:"max=1000000"` // for epoch 13 > 800k ATXs are expected, added some safety margin
}

// LayerData is the data response for a given layer ID.
type LayerData struct {
	Ballots []types.BallotID `scale:"max=500"` // expected are 50 proposals per layer + safety margin
}

type OpinionRequest struct {
	Layer types.LayerID
	Block *types.BlockID
}

type LayerOpinion struct {
	PrevAggHash types.Hash32
	Certified   *types.BlockID

	peer p2p.Peer
}

// SetPeer ...
func (lo *LayerOpinion) SetPeer(p p2p.Peer) {
	lo.peer = p
}

// Peer ...
func (lo *LayerOpinion) Peer() p2p.Peer {
	return lo.peer
}

// MarshalLogObject implements logging encoder for LayerOpinion.
func (lo *LayerOpinion) MarshalLogObject(encoder log.ObjectEncoder) error {
	encoder.AddString("peer", lo.peer.String())
	encoder.AddString("prev hash", lo.PrevAggHash.String())
	encoder.AddBool("has cert", lo.Certified != nil)
	if lo.Certified != nil {
		encoder.AddString("cert block", lo.Certified.String())
	}
	return nil
}
