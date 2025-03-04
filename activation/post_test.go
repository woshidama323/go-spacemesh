package activation

import (
	"context"
	"testing"
	"time"

	"github.com/spacemeshos/post/config"
	"github.com/spacemeshos/post/initialization"
	"github.com/spacemeshos/post/shared"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/datastore"
	"github.com/spacemeshos/go-spacemesh/log/logtest"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/spacemeshos/go-spacemesh/sql"
)

func TestPostSetupManager(t *testing.T) {
	mgr := newTestPostManager(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var eg errgroup.Group
	eg.Go(func() error {
		timer := time.NewTicker(50 * time.Millisecond)
		defer timer.Stop()

		lastStatus := &PostSetupStatus{}
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				status := mgr.Status()
				require.GreaterOrEqual(t, status.NumLabelsWritten, lastStatus.NumLabelsWritten)

				if status.NumLabelsWritten == uint64(mgr.opts.NumUnits)*mgr.cfg.LabelsPerUnit {
					return nil
				}
				require.Contains(t, []PostSetupState{PostSetupStatePrepared, PostSetupStateInProgress}, status.State)
				lastStatus = status
			}
		}
	})

	// Create data.
	require.NoError(t, mgr.PrepareInitializer(context.Background(), mgr.opts))
	require.NoError(t, mgr.StartSession(context.Background()))
	require.NoError(t, eg.Wait())
	require.Equal(t, PostSetupStateComplete, mgr.Status().State)

	// Create data (same opts).
	require.NoError(t, mgr.PrepareInitializer(context.Background(), mgr.opts))
	require.NoError(t, mgr.StartSession(context.Background()))

	// Cleanup.
	require.NoError(t, mgr.Reset())

	// Create data (same opts, after deletion).
	require.NoError(t, mgr.PrepareInitializer(context.Background(), mgr.opts))
	require.NoError(t, mgr.StartSession(context.Background()))
	require.Equal(t, PostSetupStateComplete, mgr.Status().State)
}

// Checks that PrepareInitializer returns an error when invalid opts are given.
// It's not exhaustive since this validation occurs in the post repo codebase
// and should be fully tested there but we check a few cases to be sure that
// PrepareInitializer will return errors when the opts don't validate.
func TestPostSetupManager_PrepareInitializer(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)

	// check no error with good options.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))

	defaultConfig := config.DefaultConfig()

	// Check that invalid options return errors
	opts := mgr.opts
	opts.ComputeBatchSize = 3
	req.Error(mgr.PrepareInitializer(context.Background(), opts))

	opts = mgr.opts
	opts.NumUnits = defaultConfig.MaxNumUnits + 1
	req.Error(mgr.PrepareInitializer(context.Background(), opts))

	opts = mgr.opts
	opts.NumUnits = defaultConfig.MinNumUnits - 1
	req.Error(mgr.PrepareInitializer(context.Background(), opts))

	opts = mgr.opts
	opts.Scrypt.N = 0
	req.Error(opts.Scrypt.Validate())
	req.Error(mgr.PrepareInitializer(context.Background(), opts))
}

func TestPostSetupManager_StartSession_WithoutProvider_Error(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)
	mgr.opts.ProviderID.value = nil

	// Create data.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts)) // prepare is fine without provider
	req.ErrorContains(mgr.StartSession(context.Background()), "no provider specified")

	req.Equal(PostSetupStateError, mgr.Status().State)
}

func TestPostSetupManager_StartSession_WithoutProviderAfterInit_OK(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Create data.
	req.NoError(mgr.PrepareInitializer(ctx, mgr.opts))
	req.NoError(mgr.StartSession(ctx))

	req.Equal(PostSetupStateComplete, mgr.Status().State)
	cancel()

	// start Initializer again, but with no provider set
	mgr.opts.ProviderID.value = nil

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	req.NoError(mgr.PrepareInitializer(ctx, mgr.opts))
	req.NoError(mgr.StartSession(ctx))

	req.Equal(PostSetupStateComplete, mgr.Status().State)
}

// Checks that the sequence of calls for initialization (first
// PrepareInitializer and then StartSession) is enforced.
func TestPostSetupManager_InitializationCallSequence(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Should fail since we have not prepared.
	req.Error(mgr.StartSession(ctx))

	req.NoError(mgr.PrepareInitializer(ctx, mgr.opts))

	// Should fail since we need to call StartSession after PrepareInitializer.
	req.Error(mgr.PrepareInitializer(ctx, mgr.opts))

	req.NoError(mgr.StartSession(ctx))

	// Should fail since it is required to call PrepareInitializer before each
	// call to StartSession.
	req.Error(mgr.StartSession(ctx))
}

func TestPostSetupManager_StateError(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)
	mgr.opts.NumUnits = 0
	req.Error(mgr.PrepareInitializer(context.Background(), mgr.opts))
	// Verify Status returns StateError
	req.Equal(PostSetupStateError, mgr.Status().State)
}

func TestPostSetupManager_InitialStatus(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)

	// Verify the initial status.
	status := mgr.Status()
	req.Equal(PostSetupStateNotStarted, status.State)
	req.Zero(status.NumLabelsWritten)

	// Create data.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))
	req.NoError(mgr.StartSession(context.Background()))
	req.Equal(PostSetupStateComplete, mgr.Status().State)

	// Re-instantiate `PostSetupManager`.
	mgr = newTestPostManager(t)

	// Verify the initial status.
	status = mgr.Status()
	req.Equal(PostSetupStateNotStarted, status.State)
	req.Zero(status.NumLabelsWritten)
}

func TestPostSetupManager_Stop(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)

	// Verify state.
	status := mgr.Status()
	req.Equal(PostSetupStateNotStarted, status.State)
	req.Zero(status.NumLabelsWritten)

	// Create data.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))
	req.NoError(mgr.StartSession(context.Background()))

	// Verify state.
	req.Equal(PostSetupStateComplete, mgr.Status().State)

	// Reset.
	req.NoError(mgr.Reset())

	// Verify state.
	req.Equal(PostSetupStateNotStarted, mgr.Status().State)

	// Create data again.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))
	req.NoError(mgr.StartSession(context.Background()))

	// Verify state.
	req.Equal(PostSetupStateComplete, mgr.Status().State)
}

func TestPostSetupManager_Stop_WhileInProgress(t *testing.T) {
	req := require.New(t)

	mgr := newTestPostManager(t)
	mgr.opts.MaxFileSize = 4096
	mgr.opts.NumUnits = mgr.cfg.MaxNumUnits

	// Create data.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))
	ctx, cancel := context.WithCancel(context.Background())
	var eg errgroup.Group
	eg.Go(func() error {
		return mgr.StartSession(ctx)
	})

	// Verify the intermediate status.
	req.Eventually(func() bool {
		return mgr.Status().State == PostSetupStateInProgress
	}, 5*time.Second, 10*time.Millisecond)

	// Stop initialization.
	cancel()

	req.ErrorIs(eg.Wait(), context.Canceled)

	// Verify status.
	status := mgr.Status()
	req.Equal(PostSetupStateStopped, status.State)
	req.LessOrEqual(status.NumLabelsWritten, uint64(mgr.opts.NumUnits)*mgr.cfg.LabelsPerUnit)

	// Continue to create data.
	req.NoError(mgr.PrepareInitializer(context.Background(), mgr.opts))
	req.NoError(mgr.StartSession(context.Background()))

	// Verify status.
	status = mgr.Status()
	req.Equal(PostSetupStateComplete, status.State)
	req.Equal(uint64(mgr.opts.NumUnits)*mgr.cfg.LabelsPerUnit, status.NumLabelsWritten)
}

func TestPostSetupManager_findCommitmentAtx_UsesLatestAtx(t *testing.T) {
	mgr := newTestPostManager(t)

	latestAtx := addPrevAtx(t, mgr.db, 1, mgr.signer)
	atx, err := mgr.findCommitmentAtx(context.Background())
	require.NoError(t, err)
	require.Equal(t, latestAtx.ID(), atx)
}

func TestPostSetupManager_findCommitmentAtx_DefaultsToGoldenAtx(t *testing.T) {
	mgr := newTestPostManager(t)

	atx, err := mgr.findCommitmentAtx(context.Background())
	require.NoError(t, err)
	require.Equal(t, mgr.goldenATXID, atx)
}

func TestPostSetupManager_getCommitmentAtx_getsCommitmentAtxFromPostMetadata(t *testing.T) {
	mgr := newTestPostManager(t)

	// write commitment atx to metadata
	commitmentAtx := types.RandomATXID()
	err := initialization.SaveMetadata(mgr.opts.DataDir, &shared.PostMetadata{
		CommitmentAtxId: commitmentAtx.Bytes(),
		NodeId:          mgr.signer.NodeID().Bytes(),
	})
	require.NoError(t, err)

	atxid, err := mgr.commitmentAtx(context.Background(), mgr.opts.DataDir)
	require.NoError(t, err)
	require.NotNil(t, atxid)
	require.Equal(t, commitmentAtx, atxid)
}

func TestPostSetupManager_getCommitmentAtx_getsCommitmentAtxFromInitialAtx(t *testing.T) {
	mgr := newTestPostManager(t)

	// add an atx by the same node
	commitmentAtx := types.RandomATXID()
	atx := types.NewActivationTx(types.NIPostChallenge{}, types.Address{}, nil, 1, nil)
	atx.CommitmentATX = &commitmentAtx
	addAtx(t, mgr.cdb, mgr.signer, atx)

	atxid, err := mgr.commitmentAtx(context.Background(), mgr.opts.DataDir)
	require.NoError(t, err)
	require.Equal(t, commitmentAtx, atxid)
}

type testPostManager struct {
	*PostSetupManager

	opts PostSetupOpts

	signer *signing.EdSigner
	cdb    *datastore.CachedDB
}

func newTestPostManager(tb testing.TB) *testPostManager {
	tb.Helper()

	sig, err := signing.NewEdSigner()
	require.NoError(tb, err)
	id := sig.NodeID()

	opts := DefaultPostSetupOpts()
	opts.DataDir = tb.TempDir()
	opts.ProviderID.SetUint32(initialization.CPUProviderID())
	opts.Scrypt.N = 2 // Speedup initialization in tests.

	goldenATXID := types.ATXID{2, 3, 4}

	syncer := NewMocksyncer(gomock.NewController(tb))
	synced := make(chan struct{})
	close(synced)
	syncer.EXPECT().RegisterForATXSynced().AnyTimes().Return(synced)

	cdb := datastore.NewCachedDB(sql.InMemory(), logtest.New(tb))
	mgr, err := NewPostSetupManager(id, DefaultPostConfig(), zaptest.NewLogger(tb), cdb, goldenATXID, syncer)
	require.NoError(tb, err)

	return &testPostManager{
		PostSetupManager: mgr,
		opts:             opts,
		signer:           sig,
		cdb:              cdb,
	}
}
