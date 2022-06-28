package hare

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spacemeshos/go-spacemesh/codec"
	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hash"
	"github.com/spacemeshos/go-spacemesh/signing"
)

func marshallUnmarshall(t *testing.T, msg *Message) *Message {
	buf, err := codec.Encode(&msg)
	require.NoError(t, err)

	m := &Message{}
	require.NoError(t, codec.Decode(buf, m))
	return m
}

func TestBuilder_TestBuild(t *testing.T) {
	b := newMessageBuilder()
	sgn := signing.NewEdSigner()
	msg := b.SetPubKey(sgn.PublicKey()).SetInstanceID(instanceID1).Sign(sgn).Build()

	m := marshallUnmarshall(t, msg.Message)
	assert.Equal(t, m, msg.Message)
}

func TestMessageBuilder_SetValues(t *testing.T) {
	s := NewSetFromValues(value5)
	msg := newMessageBuilder().SetValues(s).Build().Message

	m := marshallUnmarshall(t, msg)
	s1 := NewSet(m.InnerMsg.Values)
	s2 := NewSet(msg.InnerMsg.Values)
	assert.True(t, s1.Equals(s2))
}

func TestMessageBuilder_SetCertificate(t *testing.T) {
	s := NewSetFromValues(value5)
	tr := newCommitTracker(1, 1, s)
	tr.OnCommit(BuildCommitMsg(signing.NewEdSigner(), s))
	cert := tr.BuildCertificate()
	assert.NotNil(t, cert)
	c := newMessageBuilder().SetCertificate(cert).Build().Message
	cert2 := marshallUnmarshall(t, c).InnerMsg.Cert
	assert.Equal(t, cert.Values, cert2.Values)
}

func TestMessageBuilder_CommitCertificateSize(t *testing.T) {
	setSize := 1                  // # of proposal IDs in set (40 bytes per element in set)
	consensusThreshold := 5       // # of commits (112 bytes per included commit)
	expectedMessageBytes := 46980 // = (68 magical bytes) + 50 * (40bytes/element) + 401 * (112byte/commit)
	// Build new proposalID set
	s := NewEmptySet(setSize)
	for i := 0; i < setSize; i++ {
		uselessTX := hash.Sum([]byte(strconv.Itoa(i)))
		var txs []types.TransactionID
		txs = append(txs, uselessTX)
		pID := types.GenLayerProposal(types.NewLayerID(1), txs).ID()
		s.Add(pID)
	}
	// Create commit certificate from commits to the set
	tr := newCommitTracker(consensusThreshold, consensusThreshold, s)
	for i := 0; i < consensusThreshold; i++ {
		commitMsg := BuildCommitMsg(signing.NewEdSigner(), s)
		buf, _ := codec.Encode(commitMsg.Bytes())
		println(hex.EncodeToString(buf))
		println()
		tr.OnCommit(commitMsg)
	}
	cert := tr.BuildCertificate()
	// Put'em in a message and encode.
	msg := newMessageBuilder().SetType(notify).SetValues(s).SetCertificate(cert).Build()
	buf, err := codec.Encode(msg.Bytes())
	println(hex.EncodeToString(buf))
	require.NoError(t, err)
	assert.Equal(t, expectedMessageBytes, len(buf))
}
