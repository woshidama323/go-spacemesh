package hare

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/hare/mocks"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/spacemeshos/go-spacemesh/p2p/pubsub"
	"github.com/spacemeshos/go-spacemesh/signing"
	smocks "github.com/spacemeshos/go-spacemesh/system/mocks"
)

type HareWrapper struct {
	totalCP     uint32
	termination chan struct{}
	clock       *mockClock
	hare        []*Hare
	//lint:ignore U1000 pending https://github.com/spacemeshos/go-spacemesh/issues/4001
	initialSets []*Set
	outputs     map[types.LayerID][]*Set
}

func newHareWrapper(totalCp uint32) *HareWrapper {
	hs := new(HareWrapper)
	hs.clock = newMockClock()
	hs.totalCP = totalCp
	hs.termination = make(chan struct{})
	hs.outputs = make(map[types.LayerID][]*Set, 0)

	return hs
}

func (his *HareWrapper) waitForTermination() {
	for {
		count := 0
		for _, p := range his.hare {
			for i := types.GetEffectiveGenesis().Add(1); !i.After(types.GetEffectiveGenesis().Add(his.totalCP)); i = i.Add(1) {
				proposalIDs, _ := p.getResult(i)
				if len(proposalIDs) > 0 {
					count++
				}
			}
		}

		if count == int(his.totalCP)*len(his.hare) {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	for _, p := range his.hare {
		for i := types.GetEffectiveGenesis().Add(1); !i.After(types.GetEffectiveGenesis().Add(his.totalCP)); i = i.Add(1) {
			s := NewEmptySet(10)
			proposalIDs, _ := p.getResult(i)
			for _, p := range proposalIDs {
				s.Add(p)
			}
			his.outputs[i] = append(his.outputs[i], s)
		}
	}

	close(his.termination)
}

func (his *HareWrapper) WaitForTimedTermination(t *testing.T, timeout time.Duration) {
	timer := time.After(timeout)
	go his.waitForTermination()
	total := types.NewLayerID(his.totalCP)
	select {
	case <-timer:
		t.Fatal("Timeout")
		return
	case <-his.termination:
		for i := types.NewLayerID(1); !i.After(total); i = i.Add(1) {
			his.checkResult(t, i)
		}
		return
	}
}

func (his *HareWrapper) checkResult(t *testing.T, id types.LayerID) {
	// check consistency
	out := his.outputs[id]
	for i := 0; i < len(out)-1; i++ {
		if !out[i].Equals(out[i+1]) {
			t.Errorf("Consistency check failed: Expected: %v Actual: %v", out[i], out[i+1])
		}
	}
}

type p2pManipulator struct {
	pubsub.PublishSubsciber
	stalledLayer types.LayerID
	err          error
}

// func (m *p2pManipulator) Register(protocol string, handler pubsub.GossipHandler) {
// 	m.PublishSubsciber.Register(
// 		protocol,
// 		func(ctx context.Context, id peer.ID, message []byte) pubsub.ValidationResult {
// 			msg, _ := MessageFromBuffer(message)
// 			println("msg.Round", msg.Round, "in layer", msg.Layer.Value)
// 			// drop messages for given layer and rounds
// 			if msg.Layer == m.stalledLayer && msg.Round < 8 && msg.Round !=
// 				preRound {
// 				// We need to return ValidationAccept here because Publish
// 				// calls the handler and if we don't return ValidationAccept
// 				// then the message will not be published to the network.
// 				return pubsub.ValidationAccept
// 			}
// 			return handler(ctx, id, message)
// 		},
// 	)
// }

func (m *p2pManipulator) Publish(ctx context.Context, protocol string, payload []byte) error {
	msg, _ := MessageFromBuffer(payload)
	if msg.Layer == m.stalledLayer && msg.Round < 8 && msg.Round != preRound {
		return m.err
	}

	if err := m.PublishSubsciber.Publish(ctx, protocol, payload); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}

	return nil
}

type hareWithMocks struct {
	*Hare
	mockRoracle *mocks.MockRolacle
}

func createTestHare(tb testing.TB, msh mesh, tcfg config.Config, clock *mockClock, p2p pubsub.PublishSubsciber, name string) *hareWithMocks {
	tb.Helper()
	signer, err := signing.NewEdSigner()
	require.NoError(tb, err)
	pub := signer.PublicKey()
	nodeID := types.BytesToNodeID(pub.Bytes())
	pke, err := signing.NewPubKeyExtractor()
	require.NoError(tb, err)

	ctrl := gomock.NewController(tb)
	patrol := mocks.NewMocklayerPatrol(ctrl)
	patrol.EXPECT().SetHareInCharge(gomock.Any()).AnyTimes()
	mockBeacons := smocks.NewMockBeaconGetter(ctrl)
	mockBeacons.EXPECT().GetBeacon(gomock.Any()).Return(types.EmptyBeacon, nil).AnyTimes()
	mockStateQ := mocks.NewMockstateQuerier(ctrl)
	mockStateQ.EXPECT().IsIdentityActiveOnConsensusView(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mockSyncS := smocks.NewMockSyncStateProvider(ctrl)
	mockSyncS.EXPECT().IsSynced(gomock.Any()).Return(true).AnyTimes()
	mockSyncS.EXPECT().IsBeaconSynced(gomock.Any()).Return(true).AnyTimes()

	mockRoracle := mocks.NewMockRolacle(ctrl)

	hare := New(
		nil,
		tcfg,
		p2p,
		signer,
		pke,
		nodeID,
		make(chan LayerOutput, 100),
		mockSyncS,
		mockBeacons,
		mockRoracle,
		patrol,
		mockStateQ,
		clock,
		logtest.New(tb).WithName(name+"_"+signer.PublicKey().ShortString()),
		withMesh(msh),
	)
	p2p.Register(pubsub.HareProtocol, hare.GetHareMsgHandler())

	return &hareWithMocks{
		Hare:        hare,
		mockRoracle: mockRoracle,
	}
}

type mockClock struct {
	channels     map[types.LayerID]chan struct{}
	layerTime    map[types.LayerID]time.Time
	currentLayer types.LayerID
	m            sync.RWMutex
}

func newMockClock() *mockClock {
	return &mockClock{
		channels:     make(map[types.LayerID]chan struct{}),
		layerTime:    map[types.LayerID]time.Time{types.GetEffectiveGenesis(): time.Now()},
		currentLayer: types.GetEffectiveGenesis().Add(1),
	}
}

func (m *mockClock) LayerToTime(layer types.LayerID) time.Time {
	m.m.RLock()
	defer m.m.RUnlock()

	return m.layerTime[layer]
}

func (m *mockClock) AwaitLayer(layer types.LayerID) chan struct{} {
	m.m.Lock()
	defer m.m.Unlock()

	if _, ok := m.layerTime[layer]; ok {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if ch, ok := m.channels[layer]; ok {
		return ch
	}
	ch := make(chan struct{})
	m.channels[layer] = ch
	return ch
}

func (m *mockClock) CurrentLayer() types.LayerID {
	m.m.RLock()
	defer m.m.RUnlock()

	return m.currentLayer
}

func (m *mockClock) advanceLayer() {
	m.m.Lock()
	defer m.m.Unlock()

	m.layerTime[m.currentLayer] = time.Now()
	if ch, ok := m.channels[m.currentLayer]; ok {
		close(ch)
		delete(m.channels, m.currentLayer)
	}
	m.currentLayer = m.currentLayer.Add(1)
}

// Test - run multiple CPs simultaneously.
func Test_multipleCPs(t *testing.T) {
	logtest.SetupGlobal(t)

	r := require.New(t)
	totalCp := uint32(3)
	finalLyr := types.GetEffectiveGenesis().Add(totalCp)
	test := newHareWrapper(totalCp)
	totalNodes := 10
	// RoundDuration is not used because we override the newRoundClock
	// function, wakeupDelta controls whether a consensus process will skip a
	// layer, if the layer tick arrives after wakeup delta then the process
	// skips the layer.
	cfg := config.Config{N: totalNodes, WakeupDelta: 2 * time.Second, RoundDuration: 0, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100, Hdist: 20}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mesh, err := mocknet.FullMeshLinked(totalNodes)
	require.NoError(t, err)

	test.initialSets = make([]*Set, totalNodes)

	pList := make(map[types.LayerID][]*types.Proposal)
	for j := types.GetEffectiveGenesis().Add(1); !j.After(finalLyr); j = j.Add(1) {
		for i := uint64(0); i < 20; i++ {
			p := genLayerProposal(j, []types.TransactionID{})
			p.EpochData = &types.EpochData{
				Beacon: types.EmptyBeacon,
			}
			pList[j] = append(pList[j], p)
		}
	}
	meshes := make([]*mocks.Mockmesh, 0, totalNodes)
	ctrl, ctx := gomock.WithContext(ctx, t)
	for i := 0; i < totalNodes; i++ {
		mockMesh := mocks.NewMockmesh(ctrl)
		mockMesh.EXPECT().GetEpochAtx(gomock.Any(), gomock.Any()).Return(&types.ActivationTxHeader{BaseTickHeight: 11, TickCount: 1}, nil).AnyTimes()
		mockMesh.EXPECT().VRFNonce(gomock.Any(), gomock.Any()).Return(types.VRFPostIndex(0), nil).AnyTimes()
		mockMesh.EXPECT().GetMalfeasanceProof(gomock.Any()).AnyTimes()
		mockMesh.EXPECT().SetWeakCoin(gomock.Any(), gomock.Any()).AnyTimes()
		for lid := types.GetEffectiveGenesis().Add(1); !lid.After(finalLyr); lid = lid.Add(1) {
			mockMesh.EXPECT().Proposals(lid).Return(pList[lid], nil)
			for _, p := range pList[lid] {
				mockMesh.EXPECT().GetAtxHeader(p.AtxID).Return(&types.ActivationTxHeader{BaseTickHeight: 11, TickCount: 1}, nil).AnyTimes()
				mockMesh.EXPECT().Ballot(p.Ballot.ID()).Return(&p.Ballot, nil).AnyTimes()
			}
		}
		mockMesh.EXPECT().Proposals(gomock.Any()).Return([]*types.Proposal{}, nil).AnyTimes()
		meshes = append(meshes, mockMesh)
	}

	// setup roundClocks to progress a layer only when all nodes have received messages from all nodes.
	roundClocks := newSharedRoundClocks(totalNodes*totalNodes, 50*time.Millisecond)
	var pubsubs []*pubsub.PubSub
	outputs := make([]map[types.LayerID]LayerOutput, totalNodes)
	var outputsWaitGroup sync.WaitGroup
	for i := 0; i < totalNodes; i++ {
		host := mesh.Hosts()[i]
		ps, err := pubsub.New(ctx, logtest.New(t), host, pubsub.DefaultConfig())
		require.NoError(t, err)
		pubsubs = append(pubsubs, ps)
		// We wrap the pubsub system to notify round clocks whenever a message
		// is received.
		testPs := &testPublisherSubscriber{
			PublishSubsciber: ps,
			notify: func(msg []byte) {
				layer, eligibilityCount := extractInstanceID(msg)
				roundClocks.clock(layer).IncMessages(int(eligibilityCount), 0)
			},
		}
		h := createTestHare(t, meshes[i], cfg, test.clock, testPs, t.Name())
		// override the round clocks method to use our shared round clocks
		h.newRoundClock = roundClocks.roundClock
		h.mockRoracle.EXPECT().IsIdentityActiveOnConsensusView(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		h.mockRoracle.EXPECT().Proof(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(make([]byte, 80), nil).AnyTimes()
		h.mockRoracle.EXPECT().CalcEligibility(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint16(1), nil).AnyTimes()
		h.mockRoracle.EXPECT().Validate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		outputsWaitGroup.Add(1)
		go func(idx int) {
			defer outputsWaitGroup.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case out, ok := <-h.blockGenCh:
					if !ok {
						return
					}
					if outputs[idx] == nil {
						outputs[idx] = make(map[types.LayerID]LayerOutput)
					}
					outputs[idx][out.Layer] = out
				}
			}
		}(i)
		test.hare = append(test.hare, h.Hare)
		e := h.Start(ctx)
		r.NoError(e)
	}
	require.NoError(t, mesh.ConnectAllButSelf())
	require.Eventually(t, func() bool {
		for _, ps := range pubsubs {
			if len(ps.ProtocolPeers(pubsub.HareProtocol)) != len(mesh.Hosts())-1 {
				return false
			}
		}
		return true
	}, 5*time.Second, 10*time.Millisecond)
	log := logtest.New(t)
	for j := types.GetEffectiveGenesis().Add(1); !j.After(finalLyr); j = j.Add(1) {
		test.clock.advanceLayer()
		time.Sleep(10 * time.Millisecond)
		log.Warning("advancing to layer %d", j.Uint32())
	}

	// There are 5 rounds per layer and totalCPs layers and we double for good measure.
	test.WaitForTimedTermination(t, time.Minute)
	for _, h := range test.hare {
		close(h.blockGenCh)
	}
	outputsWaitGroup.Wait()
	for _, out := range outputs {
		for lid := types.GetEffectiveGenesis().Add(1); !lid.After(finalLyr); lid = lid.Add(1) {
			require.NotNil(t, out[lid])
			require.ElementsMatch(t, types.ToProposalIDs(pList[lid]), out[lid].Proposals)
		}
	}
	t.Cleanup(func() {
		for _, h := range test.hare {
			h.Close()
		}
	})
}

// Test - run multiple CPs where one of them runs more than one iteration.
func Test_multipleCPsAndIterations(t *testing.T) {
	logtest.SetupGlobal(t)

	r := require.New(t)
	totalCp := uint32(1)
	finalLyr := types.GetEffectiveGenesis().Add(totalCp)
	test := newHareWrapper(totalCp)
	totalNodes := 10
	// RoundDuration is not used because we override the newRoundClock
	// function, wakeupDelta controls whether a consensus process will skip a
	// layer, if the layer tick arrives after wakeup delta then the process
	// skips the layer.
	cfg := config.Config{N: totalNodes, WakeupDelta: 2 * time.Second, RoundDuration: 0, ExpectedLeaders: 5, LimitIterations: 1000, LimitConcurrent: 100, Hdist: 20}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mesh, err := mocknet.FullMeshLinked(totalNodes)
	require.NoError(t, err)

	test.initialSets = make([]*Set, totalNodes)

	pList := make(map[types.LayerID][]*types.Proposal)
	for j := types.GetEffectiveGenesis().Add(1); !j.After(finalLyr); j = j.Add(1) {
		for i := uint64(0); i < 20; i++ {
			p := genLayerProposal(j, []types.TransactionID{})
			p.EpochData = &types.EpochData{
				Beacon: types.EmptyBeacon,
			}
			pList[j] = append(pList[j], p)
		}
	}

	meshes := make([]*mocks.Mockmesh, 0, totalNodes)
	ctrl, ctx := gomock.WithContext(ctx, t)
	for i := 0; i < totalNodes; i++ {
		mockMesh := mocks.NewMockmesh(ctrl)
		mockMesh.EXPECT().GetEpochAtx(gomock.Any(), gomock.Any()).Return(&types.ActivationTxHeader{BaseTickHeight: 11, TickCount: 1}, nil).AnyTimes()
		mockMesh.EXPECT().VRFNonce(gomock.Any(), gomock.Any()).Return(types.VRFPostIndex(0), nil).AnyTimes()
		mockMesh.EXPECT().GetMalfeasanceProof(gomock.Any()).AnyTimes()
		mockMesh.EXPECT().SetWeakCoin(gomock.Any(), gomock.Any()).AnyTimes()
		for lid := types.GetEffectiveGenesis().Add(1); !lid.After(finalLyr); lid = lid.Add(1) {
			mockMesh.EXPECT().Proposals(lid).Return(pList[lid], nil)
			for _, p := range pList[lid] {
				mockMesh.EXPECT().GetAtxHeader(p.AtxID).Return(&types.ActivationTxHeader{BaseTickHeight: 11, TickCount: 1}, nil).AnyTimes()
				mockMesh.EXPECT().Ballot(p.Ballot.ID()).Return(&p.Ballot, nil).AnyTimes()
			}
		}
		mockMesh.EXPECT().Proposals(gomock.Any()).Return([]*types.Proposal{}, nil).AnyTimes()
		meshes = append(meshes, mockMesh)
	}

	// setup roundClocks to progress a layer only when all nodes have received messages from all nodes.
	roundClocks := newSharedRoundClocks(totalNodes*totalNodes, 50*time.Millisecond)
	stalledLayer := types.GetEffectiveGenesis().Add(1)

	// The stalled layer drops messages from rounds 0-7 so to ensure that those
	// rounds progress without requiring notification via
	// testPublisherSubscriber we "pre close" the rounds by providing a closed
	// channel.
	roundClocks.roundClock(stalledLayer)
	// stalledLayerRoundClock := roundClocks.clock(stalledLayer)
	// closedChan := make(chan struct{})
	// close(closedChan)
	// So this is tricky, ideally this would work but then layer 8 when advance round is called it tries to close the channel for layer 7 and that is already closed and we panic, so what to do?
	// Ok I could swithc the shared round clock to send values on channels but that is a bit tricky because I then have to have the right number of values for all the calls from all the cp instances.
	// A better approach could be to use my original approach and simply call AwaitEndOfRound before calling advacne round to make sure the value is inited. Then I don't need to sleep.
	// for r := 0; r < 8; r++ {
	// 	stalledLayerRoundClock.rounds[uint32(r)] = closedChan
	// }
	var pubsubs []*pubsub.PubSub
	outputs := make([]map[types.LayerID]LayerOutput, totalNodes)
	var outputsWaitGroup sync.WaitGroup
	var layer8Count int
	var notifyMutex sync.Mutex
	for i := 0; i < totalNodes; i++ {
		host := mesh.Hosts()[i]
		ps, err := pubsub.New(ctx, logtest.New(t), host, pubsub.DefaultConfig())
		require.NoError(t, err)

		// We wrap the pubsub system to notify round clocks whenever a message
		// is received.
		testPs := &testPublisherSubscriber{
			PublishSubsciber: ps,
			// notify: func(msg []byte) {
			// 	layer, eligibilityCount := extractInstanceID(msg)
			// 	roundClocks.clock(layer).IncMessages(int(eligibilityCount))
			// },
			notify: func(msg []byte) {
				notifyMutex.Lock()
				defer notifyMutex.Unlock()
				m, err := MessageFromBuffer(msg)
				if err != nil {
					panic(err)
				}

				roundClocks.clock(m.Layer).IncMessages(int(m.Eligibility.Count), m.Round)
				// Keep track of progress in the stalled layer
				if m.Layer == stalledLayer && m.Round == preRound {
					// println("layer8adding", m.Eligibility.Count)
					layer8Count += int(m.Eligibility.Count)
					// println("layer8count", layer8Count)
					if layer8Count == totalNodes*totalNodes {
						// Gotta sleep to make sure that advance round is called first by the round clock
						// time.Sleep(4 * roundClocks.processingDelay)
						<-roundClocks.clock(m.Layer).AwaitEndOfRound(preRound)
						println("status round completed")
						// time.Sleep(time.Second)
						roundClocks.clock(m.Layer).advanceToRound(8)
						// roundClocks.clock(m.Layer).m.Lock()
						// defer roundClocks.clock(m.Layer).m.Unlock()
						// for r := 0; r < 8; r++ {
						// 	println("aaaaaaaaaaaaaaaaaaaaaaaaaaaaadd")
						// 	// roundClocks.clock(m.Layer).AwaitEndOfRound(uint32(r))
						// 	time.Sleep(time.Millisecond * 200)
						// 	roundClocks.clock(m.Layer).advanceRound()
						// }
					}
				}
			},
		}
		// the manipulatior drops meessages for the given layer on the first iteration this will result in a second iteration s
		mp2p := &p2pManipulator{PublishSubsciber: testPs, stalledLayer: stalledLayer, err: errors.New("fake err")}
		h := createTestHare(t, meshes[i], cfg, test.clock, mp2p, t.Name())
		// override the round clocks method to use our shared round clocks
		h.newRoundClock = roundClocks.roundClock
		h.mockRoracle.EXPECT().IsIdentityActiveOnConsensusView(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		h.mockRoracle.EXPECT().Proof(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(make([]byte, 80), nil).AnyTimes()
		h.mockRoracle.EXPECT().CalcEligibility(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uint16(1), nil).AnyTimes()
		h.mockRoracle.EXPECT().Validate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
		outputsWaitGroup.Add(1)
		go func(idx int) {
			defer outputsWaitGroup.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case out, ok := <-h.blockGenCh:
					if !ok {
						return
					}
					if outputs[idx] == nil {
						outputs[idx] = make(map[types.LayerID]LayerOutput)
					}
					outputs[idx][out.Layer] = out
				}
			}
		}(i)
		test.hare = append(test.hare, h.Hare)
		e := h.Start(ctx)
		r.NoError(e)
	}
	require.NoError(t, mesh.ConnectAllButSelf())
	require.Eventually(t, func() bool {
		for _, ps := range pubsubs {
			if len(ps.ProtocolPeers(pubsub.HareProtocol)) != len(mesh.Hosts())-1 {
				return false
			}
		}
		return true
	}, 5*time.Second, 10*time.Millisecond)
	layerDuration := 250 * time.Millisecond
	log := logtest.New(t)
	go func() {
		for j := types.GetEffectiveGenesis().Add(1); !j.After(finalLyr); j = j.Add(1) {
			test.clock.advanceLayer()
			log.Warning("advancing to layer %d", j.Uint32())
			time.Sleep(layerDuration)
		}
	}()

	// There are 5 rounds per layer and totalCPs layers and we double to allow
	// for the for good measure. Also one layer in this test will run 2
	// iterations so we increase the layer count by 1.
	test.WaitForTimedTermination(t, time.Minute)
	for _, h := range test.hare {
		close(h.blockGenCh)
	}
	outputsWaitGroup.Wait()
	for _, out := range outputs {
		for lid := types.GetEffectiveGenesis().Add(1); !lid.After(finalLyr); lid = lid.Add(1) {
			require.NotNil(t, out[lid])
			require.ElementsMatch(t, types.ToProposalIDs(pList[lid]), out[lid].Proposals)
		}
	}
	t.Cleanup(func() {
		for _, h := range test.hare {
			h.Close()
		}
	})
}

// The SharedRoundClock acts as a synchronization mechanism to allow for rounds
// to progress only when some threshold of messages have been received. This
// allows for reliable tests because it removes the possibility of tests
// failing due to late arrival of messages. It also allows for fast tests
// because as soon as the required number of messages have been received the
// next round is started, this avoids any downtime that may have occurred in
// situations where we set a fixed round time. However there is a caveat,
// because this is hooking into the PubSub system there is still a delay
// between messages being delivered and hare actually processing those
// messages, that delay is accounted for by the processingDelay field. Setting
// this too low will result in tests failing due to late message delivery,
// increasing the value does however increase overall test time, so we don't
// want this to be huge. A better solution would be to extract message
// processing from consensus processes so that we could hook into actual
// message delivery to Hare, see - https://github.com/spacemeshos/go-spacemesh/issues/4248.
type SharedRoundClock struct {
	currentRound    uint32
	rounds          map[uint32]chan struct{}
	minCount        int
	processingDelay time.Duration
	messageCount    int
	m               sync.Mutex
	layer           types.LayerID
}

func (c *SharedRoundClock) AwaitWakeup() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (c *SharedRoundClock) AwaitEndOfRound(round uint32) <-chan struct{} {
	c.m.Lock()
	defer c.m.Unlock()
	return c.getRound(round)
}

func (c *SharedRoundClock) getRound(round uint32) chan struct{} {
	ch, ok := c.rounds[round]
	if !ok {
		ch = make(chan struct{})
		c.rounds[round] = ch
	}
	return ch
}

// RoundEnd is currently called only by metrics reporting code, so the returned
// value should not affect how components behave.
func (c *SharedRoundClock) RoundEnd(round uint32) time.Time {
	return time.Now()
}

func (c *SharedRoundClock) IncMessages(cnt int, round uint32) {
	c.m.Lock()
	defer c.m.Unlock()

	c.messageCount += cnt
	println("incmessages for round", c.currentRound, "message round", round, "for layer", c.layer.Value, "current count", c.messageCount)
	// if c.messageCount > c.minCount {
	// 	panic("violation")
	// }
	if c.messageCount >= c.minCount {
		time.AfterFunc(c.processingDelay, func() {
			c.m.Lock()
			defer c.m.Unlock()
			println("progressing to round", c.currentRound+1, "for layer", c.layer.Value)
			c.advanceRound()
		})
	}
}

func (c *SharedRoundClock) advanceRound() {
	c.messageCount = 0
	c.currentRound++
	if prevRound, ok := c.rounds[c.currentRound-1]; ok {
		println("closing old round", prevRound, c.currentRound-1)
		close(prevRound)
	}
}

// advanceToRound advances the clock to the given round.
func (c *SharedRoundClock) advanceToRound(round uint32) {
	println("about to acquire lock")
	c.m.Lock()
	println("acquired lock")
	defer c.m.Unlock()
	println("deferred unlock lock")
	println("advanceToRound current", c.currentRound, "target", round)

	for c.currentRound < round {
		println("advancing to round", c.currentRound+1, "closing")
		// Ensure that the channel exists before we close it
		c.getRound(c.currentRound)
		c.advanceRound()
	}
}

// sharedRoundClocks provides functionality to create shared round clocks that
// can be injected into Hare, it uses a map to enusre that all hare instances
// share the same round clock instance for each layer and provies methods to
// safely interact with the map from multiple goroutines.
type sharedRoundClocks struct {
	// The number of messages that need to be received before progressing to
	// the next round.
	roundMessageCount int
	// The time to wait between reaching the roundMessageCount and progressing
	// to the next round, this is to allow for messages received over pubsub to
	// be processed by hare before moving to the next round.
	processingDelay time.Duration
	clocks          map[types.LayerID]*SharedRoundClock
	m               sync.Mutex
}

func newSharedRoundClocks(roundMessageCount int, processingDelay time.Duration) *sharedRoundClocks {
	return &sharedRoundClocks{
		roundMessageCount: roundMessageCount,
		processingDelay:   processingDelay,
		clocks:            make(map[types.LayerID]*SharedRoundClock),
	}
}

// roundClock returns the shared round clock for the given layer, if the clock doesn't exist it will be created.
func (rc *sharedRoundClocks) roundClock(layer types.LayerID) RoundClock {
	rc.m.Lock()
	defer rc.m.Unlock()
	c, exist := rc.clocks[layer]
	if !exist {
		c = &SharedRoundClock{
			currentRound:    preRound,
			rounds:          make(map[uint32]chan struct{}),
			minCount:        rc.roundMessageCount,
			processingDelay: rc.processingDelay,
			messageCount:    0,
			layer:           layer,
		}
		rc.clocks[layer] = c
	}
	return c
}

// clock returns the clock for the given layer, if the clock does not exist the
// returned value will be nil.
func (rc *sharedRoundClocks) clock(layer types.LayerID) *SharedRoundClock {
	rc.m.Lock()
	defer rc.m.Unlock()
	return rc.clocks[layer]
}

func extractInstanceID(payload []byte) (types.LayerID, uint16) {
	m, err := MessageFromBuffer(payload)
	if err != nil {
		panic(err)
	}
	return m.Layer, m.Eligibility.Count
}

// testPublisherSubscriber wraps a PublisherSubscriber and hooks into the
// message handler to be able to be notified of received messages.
type testPublisherSubscriber struct {
	pubsub.PublishSubsciber
	notify func([]byte)
}

func (ps *testPublisherSubscriber) Register(protocol string, handler pubsub.GossipHandler) {
	ps.PublishSubsciber.Register(
		protocol,
		func(ctx context.Context, id peer.ID, msg []byte) pubsub.ValidationResult {
			res := handler(ctx, id, msg)
			ps.notify(msg)
			return res
		},
	)
}
