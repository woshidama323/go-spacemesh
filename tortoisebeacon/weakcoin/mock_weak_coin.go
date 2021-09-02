// Code generated by mockery v1.0.0. DO NOT EDIT.

package weakcoin

import (
	context "context"

	service "github.com/spacemeshos/go-spacemesh/p2p/service"
	mock "github.com/stretchr/testify/mock"

	types "github.com/spacemeshos/go-spacemesh/common/types"
)

// MockWeakCoin is an autogenerated mock type for the WeakCoin type
type MockWeakCoin struct {
	mock.Mock
}

// Get provides a mock function with given fields: epoch, round
func (_m *MockWeakCoin) Get(epoch types.EpochID, round types.RoundID) bool {
	ret := _m.Called(epoch, round)

	var r0 bool
	if rf, ok := ret.Get(0).(func(types.EpochID, types.RoundID) bool); ok {
		r0 = rf(epoch, round)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// HandleSerializedMessage provides a mock function with given fields: ctx, data, sync
func (_m *MockWeakCoin) HandleSerializedMessage(ctx context.Context, data service.GossipMessage, sync service.Fetcher) {
	_m.Called(ctx, data, sync)
}

// OnRoundFinished provides a mock function with given fields: epoch, round
func (_m *MockWeakCoin) OnRoundFinished(epoch types.EpochID, round types.RoundID) {
	_m.Called(epoch, round)
}

// OnRoundStarted provides a mock function with given fields: epoch, round
func (_m *MockWeakCoin) OnRoundStarted(epoch types.EpochID, round types.RoundID) {
	_m.Called(epoch, round)
}

// PublishProposal provides a mock function with given fields: ctx, epoch, round
func (_m *MockWeakCoin) PublishProposal(ctx context.Context, epoch types.EpochID, round types.RoundID) error {
	ret := _m.Called(ctx, epoch, round)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.EpochID, types.RoundID) error); ok {
		r0 = rf(ctx, epoch, round)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
