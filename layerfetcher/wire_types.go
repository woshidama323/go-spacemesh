package layerfetcher

import "github.com/spacemeshos/go-spacemesh/common/types"

// layerBlocks is the response for layer hash
type layerBlocks struct {
	Blocks          []types.BlockID
	LatestBlocks    []types.BlockID // LatestBlocks are the blocks received in the last 30 seconds from gossip
	VerifyingVector []types.BlockID // VerifyingVector is the input vector for verifying tortoise
}

// layerBlocks is the response for layer hash
type LayerHash struct {
	Hash    types.Hash32
	LayerID types.LayerID
}

// layerBlocks is the response for layer blocks exchange
type LayerBlocks struct {
	LayerID       types.LayerID
	Blocks        []types.BlockID
	LayerHash     types.Hash32
	AggregatedHash types.Hash32
}
