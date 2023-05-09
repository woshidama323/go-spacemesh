package p2p_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	varint "github.com/multiformats/go-varint"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/cmd/node"
	"github.com/spacemeshos/go-spacemesh/config"
	"github.com/spacemeshos/go-spacemesh/events"
	"github.com/spacemeshos/go-spacemesh/log"
	ps "github.com/spacemeshos/go-spacemesh/p2p/pubsub"
)

func TestBlacklist(t *testing.T) {
	// conf := &config.Config{
	// 	BaseConfig: config.BaseConfig{
	// 		LayerDuration: time.Minute,
	// 		DataDirParent: t.TempDir(),
	// 	},
	// 	Tortoise: tortoise.Config{
	// 		Zdist: 1,
	// 	},
	// 	Genesis: &config.GenesisConfig{
	// 		GenesisTime: time.Now().Format(time.RFC3339),
	// 	},
	// }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf1 := config.DefaultTestConfig()
	conf1.DataDirParent = t.TempDir()
	conf1.P2P.Listen = "/ip4/0.0.0.0/tcp/0"
	app1, err := NewApp(&conf1)
	require.NoError(t, err)
	conf2 := config.DefaultTestConfig()
	conf2.DataDirParent = t.TempDir()
	conf2.P2P.Listen = "/ip4/0.0.0.0/tcp/0"
	app2, err := NewApp(&conf2)
	require.NoError(t, err)
	g := errgroup.Group{}
	defer func() {
		app1.Cleanup(ctx)
		app2.Cleanup(ctx)
	}()
	g.Go(func() error {
		return app1.Start(ctx)
	})
	<-app1.Started()
	g.Go(func() error {
		return app2.Start(ctx)
	})
	<-app2.Started()

	// app.Host().Peerstore().AddAddr(, addr multiaddr.Multiaddr, ttl time.Duration)
	// app.Host().Connect(, pi peer.AddrInfo)
	// h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	// require.NoError(t, err)
	// fmt.Printf("addrs %+v\n", h.Addrs())
	fmt.Printf("addrs %+v\n", app1.Host().Addrs())
	fmt.Printf("ids %v %v\n", app1.Host().ID(), app2.Host().ID())

	time.Sleep(time.Second)
	printcons(app1, app2)
	println("newsconnect")
	err = app2.Host().Connect(context.Background(), peer.AddrInfo{
		ID:    app1.Host().ID(),
		Addrs: app1.Host().Addrs(),
	})
	require.NoError(t, err)

	time.Sleep(time.Second)

	// printcons(app1, app2)
	println("newstream")
	s, err := app2.Host().NewStream(ctx, app1.Host().ID(), pubsub.GossipSubID_v11)
	require.NoError(t, err)
	println(s)

	time.Sleep(time.Second)
	// k := app2.Host().Peerstore().PrivKey(app2.Host().ID())

	printcons(app1, app2)
	require.Greater(t, len(app2.Host().Network().ConnsToPeer(app1.Host().ID())), 0)
	// printcons(app1, app2)
	println("send")
	msg := makeMessage(make([]byte, 20), app2.Host().ID(), 100000)
	// err = signMessage(app2.Host().ID(), k, msg)
	// require.NoError(t, err)
	err = writeRpc(rpcWithMessages(msg), s)
	require.NoError(t, err)

	// for {
	// 	time.Sleep(time.Second)
	// 	printcons(app1, app2)

	// }

	require.Eventually(t, func() bool {
		printcons(app1, app2)
		return len(app2.Host().Network().ConnsToPeer(app1.Host().ID())) == 0
	}, time.Second*15, time.Millisecond*200)

	// fmt.Printf("conns to peer %v\n", len(app2.Host().Network().ConnsToPeer(app1.Host().ID())))
	// for {
	// 	time.Sleep(time.Second)
	// 	printcons(app1, app2)

	// }
	// err = app2.Host().Publish(ctx, pubsub.AtxProtocol, make([]byte, 20))
	// require.NoError(t, err)
	// l, err := s.Write(make([]byte, 10000))
	// require.NoError(t, err)
	// println(l)

	// s, err := h.NewStream(context.Background(), app.Host().ID())
	// require.NoError(t, err)
	// n, err := s.Write(make([]byte, 1))
	// require.NoError(t, err)
	// println(n)
	cancel()
	require.NoError(t, g.Wait())
}

func printcons(app1, app2 *node.App) {
	fmt.Printf("conns to peer %v\n", len(app2.Host().Network().ConnsToPeer(app1.Host().ID())))
	fmt.Printf("conns to back %v\n", len(app1.Host().Network().ConnsToPeer(app2.Host().ID())))
}

// func signMessage(pid peer.ID, key crypto.PrivKey, m *pb.Message) error {
// 	bytes, err := m.Marshal()
// 	if err != nil {
// 		return err
// 	}

// 	bytes = withSignPrefix(bytes)

// 	sig, err := key.Sign(bytes)
// 	if err != nil {
// 		return err
// 	}

// 	m.Signature = sig

// 	pk, _ := pid.ExtractPublicKey()
// 	if pk == nil {
// 		pubk, err := crypto.MarshalPublicKey(key.GetPublic())
// 		if err != nil {
// 			return err
// 		}
// 		m.Key = pubk
// 	}

// 	return nil
// }

// func withSignPrefix(bytes []byte) []byte {
// 	return append([]byte(pubsub.SignPrefix), bytes...)
// }

func writeRpc(rpc *pubsub.RPC, s network.Stream) error {
	size := uint64(rpc.Size())

	buf := make([]byte, varint.UvarintSize(size)+int(size))

	n := binary.PutUvarint(buf, size)
	_, err := rpc.MarshalTo(buf[n:])
	if err != nil {
		return err
	}

	_, err = s.Write(buf)
	return err
}

func makeMessage(data []byte, id peer.ID, sequence uint64) *pb.Message {
	protocol := ps.AtxProtocol
	seqno := make([]byte, 8)
	binary.BigEndian.PutUint64(seqno, sequence)
	m := &pb.Message{
		Data:  data,
		Topic: &protocol,
	}
	return m
}

func rpcWithMessages(msgs ...*pb.Message) *pubsub.RPC {
	return &pubsub.RPC{RPC: pb.RPC{Publish: msgs}}
}

func NewApp(conf *config.Config) (*node.App, error) {
	app := node.New(
		node.WithConfig(conf),
		node.WithLog(log.RegisterHooks(
			log.NewWithLevel("", zap.NewAtomicLevelAt(zapcore.DebugLevel)),
			events.EventHook())),
	)

	err := app.Initialize()
	return app, err
}
