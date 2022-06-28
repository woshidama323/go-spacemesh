package hare

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/hare/mocks"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
	"github.com/spacemeshos/go-spacemesh/sql/ballots"
	"github.com/spacemeshos/go-spacemesh/sql/proposals"
	smocks "github.com/spacemeshos/go-spacemesh/system/mocks"
	"github.com/stretchr/testify/require"
)

type messageChecker = func([]byte)

func runWithAssertions(t testing.TB, msgChecker messageChecker) {
	t.Helper()
	logtest.SetupGlobal(t)

	types.SetLayersPerEpoch(4)
	r := require.New(t)
	totalCp := uint32(3)
	test := newHareWrapper(totalCp)
	totalNodes := 20
	cfg := config.Config{N: totalNodes, F: totalNodes/2 - 1, RoundDuration: 5, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mesh, err := mocknet.FullMeshLinked(ctx, totalNodes)
	require.NoError(t, err)

	test.initialSets = make([]*Set, totalNodes)

	dbs := make([]*sql.Database, 0, totalNodes)
	for i := 0; i < totalNodes; i++ {
		dbs = append(dbs, sql.InMemory())
	}
	pList := make(map[types.LayerID][]*types.Proposal)
	blocks := make(map[types.LayerID]*types.Block)
	for j := types.GetEffectiveGenesis().Add(1); !j.After(types.GetEffectiveGenesis().Add(totalCp)); j = j.Add(1) {
		for i := uint64(0); i < 200; i++ {
			p := types.GenLayerProposal(j, []types.TransactionID{})
			p.EpochData = &types.EpochData{
				Beacon: types.EmptyBeacon,
			}
			for x := 0; x < totalNodes; x++ {
				require.NoError(t, ballots.Add(dbs[x], &p.Ballot))
				require.NoError(t, proposals.Add(dbs[x], p))
			}
			pList[j] = append(pList[j], p)
		}
		blocks[j] = types.GenLayerBlock(j, types.RandomTXSet(199))
	}
	var pubsubs []*pubsub.PubSub
	scMap := NewSharedClock(totalNodes, totalCp, time.Duration(50*int(totalCp)*totalNodes)*time.Millisecond)
	for i := 0; i < totalNodes; i++ {
		host := mesh.Hosts()[i]
		ps, err := pubsub.New(ctx, logtest.New(t), host, pubsub.DefaultConfig())
		require.NoError(t, err)
		src := NewSimRoundClock(ps, scMap)
		pubsubs = append(pubsubs, ps)

		ed := signing.NewEdSigner()
		pub := ed.PublicKey()
		nodeID := types.BytesToNodeID(pub.Bytes())
		ctrl := gomock.NewController(t)
		patrol := mocks.NewMocklayerPatrol(ctrl)
		patrol.EXPECT().SetHareInCharge(gomock.Any()).AnyTimes()
		patrol.EXPECT().CompleteHare(gomock.Any()).AnyTimes()
		mockBeacons := smocks.NewMockBeaconGetter(ctrl)
		mockBeacons.EXPECT().GetBeacon(gomock.Any()).Return(types.EmptyBeacon, nil).AnyTimes()
		mockStateQ := mocks.NewMockstateQuerier(ctrl)
		mockStateQ.EXPECT().IsIdentityActiveOnConsensusView(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		mockSyncS := smocks.NewMockSyncStateProvider(ctrl)
		mockSyncS.EXPECT().IsSynced(gomock.Any()).Return(true).AnyTimes()

		mockRoracle := mocks.NewMockRolacle(ctrl)
		mockBlockGen := mocks.NewMockblockGenerator(ctrl)
		mockMeshDB := mocks.NewMockmeshProvider(ctrl)
		mockFetcher := smocks.NewMockProposalFetcher(ctrl)

		hare := New(db, tcfg, pid, p2p, ed, nodeID, mockBlockGen, mockSyncS, mockMeshDB, mockBeacons, mockFetcher, mockRoracle, patrol, 10,
			mockStateQ, clock, logtest.New(t).WithName(name+"_"+ed.PublicKey().ShortString()))

		msgHandler := hare.GetHareMsgHandler()
		msgPrinter := func(ctx context.Context, pid peer.ID, msgBytes []byte) pubsub.ValidationResult {
			m := &Message{}
			codec.Decode(msgBytes, m)
			fmt.Printf(" %s %d", m.InnerMsg.Type.String(), len(msgBytes))
			return msgHandler(ctx, pid, msgBytes)
		}
		p2p.Register(ProtoName, msgPrinter)

		return &hareWithMocks{
			Hare:         hare,
			mockRoracle:  mockRoracle,
			mockMeshDB:   mockMeshDB,
			mockBlockGen: mockBlockGen,
			mockFetcher:  mockFetcher,
		}

		h.mockRoracle.EXPECT().IsIdentityActiveOnConsensusView(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		h.mockRoracle.EXPECT().Proof(gomock.Any(), gomock.Any(), gomock.Any()).Return(make([]byte, 100), nil).AnyTimes()
		h.mockRoracle.EXPECT().CalcEligibility(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint16(1), nil).AnyTimes()
		h.mockRoracle.EXPECT().Validate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		h.mockFetcher.EXPECT().GetProposals(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		h.mockBlockGen.EXPECT().GenerateBlock(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, layerID types.LayerID, got []*types.Proposal) (*types.Block, error) {
				require.ElementsMatch(t, pList[layerID], got)
				return blocks[layerID], nil
			}).AnyTimes()
		h.mockMeshDB.EXPECT().AddBlockWithTXs(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, block *types.Block) error {
				require.Equal(t, blocks[block.LayerIndex], block)
				return nil
			}).AnyTimes()
		h.mockMeshDB.EXPECT().ProcessLayerPerHareOutput(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		h.newRoundClock = src.NewRoundClock
		test.hare = append(test.hare, h.Hare)
		e := h.Start(context.TODO())
		r.NoError(e)
	}
	require.NoError(t, mesh.ConnectAllButSelf())
	require.Eventually(t, func() bool {
		for _, ps := range pubsubs {
			if len(ps.ProtocolPeers(ProtoName)) != len(mesh.Hosts())-1 {
				return false
			}
		}
		return true
	}, 5*time.Second, 10*time.Millisecond)
	go func() {
		for j := types.GetEffectiveGenesis().Add(1); !j.After(types.GetEffectiveGenesis().Add(totalCp)); j = j.Add(1) {
			test.clock.advanceLayer()
			time.Sleep(250 * time.Millisecond)
		}
	}()

	test.WaitForTimedTermination(t, 80*time.Second)
}
