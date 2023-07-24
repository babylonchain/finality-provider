// Code generated by MockGen. DO NOT EDIT.
// Source: bbnclient/interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	types "github.com/babylonchain/babylon/types"
	types0 "github.com/babylonchain/babylon/x/btcstaking/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	secp256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	gomock "github.com/golang/mock/gomock"
)

// MockBabylonClient is a mock of BabylonClient interface.
type MockBabylonClient struct {
	ctrl     *gomock.Controller
	recorder *MockBabylonClientMockRecorder
}

// MockBabylonClientMockRecorder is the mock recorder for MockBabylonClient.
type MockBabylonClientMockRecorder struct {
	mock *MockBabylonClient
}

// NewMockBabylonClient creates a new mock instance.
func NewMockBabylonClient(ctrl *gomock.Controller) *MockBabylonClient {
	mock := &MockBabylonClient{ctrl: ctrl}
	mock.recorder = &MockBabylonClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBabylonClient) EXPECT() *MockBabylonClientMockRecorder {
	return m.recorder
}

// CommitPubRandList mocks base method.
func (m *MockBabylonClient) CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitPubRandList", btcPubKey, startHeight, pubRandList, sig)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitPubRandList indicates an expected call of CommitPubRandList.
func (mr *MockBabylonClientMockRecorder) CommitPubRandList(btcPubKey, startHeight, pubRandList, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitPubRandList", reflect.TypeOf((*MockBabylonClient)(nil).CommitPubRandList), btcPubKey, startHeight, pubRandList, sig)
}

// QueryHeader mocks base method.
func (m *MockBabylonClient) QueryHeader(height int64) (*coretypes.ResultHeader, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryHeader", height)
	ret0, _ := ret[0].(*coretypes.ResultHeader)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryHeader indicates an expected call of QueryHeader.
func (mr *MockBabylonClientMockRecorder) QueryHeader(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryHeader", reflect.TypeOf((*MockBabylonClient)(nil).QueryHeader), height)
}

// QueryHeightWithLastPubRand mocks base method.
func (m *MockBabylonClient) QueryHeightWithLastPubRand(btcPubKey *types.BIP340PubKey) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryHeightWithLastPubRand", btcPubKey)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryHeightWithLastPubRand indicates an expected call of QueryHeightWithLastPubRand.
func (mr *MockBabylonClientMockRecorder) QueryHeightWithLastPubRand(btcPubKey interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryHeightWithLastPubRand", reflect.TypeOf((*MockBabylonClient)(nil).QueryHeightWithLastPubRand), btcPubKey)
}

// QueryNodeStatus mocks base method.
func (m *MockBabylonClient) QueryNodeStatus() (*coretypes.ResultStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryNodeStatus")
	ret0, _ := ret[0].(*coretypes.ResultStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryNodeStatus indicates an expected call of QueryNodeStatus.
func (mr *MockBabylonClientMockRecorder) QueryNodeStatus() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryNodeStatus", reflect.TypeOf((*MockBabylonClient)(nil).QueryNodeStatus))
}

// QueryShouldSubmitJurySigs mocks base method.
func (m *MockBabylonClient) QueryShouldSubmitJurySigs(btcPubKey *types.BIP340PubKey) (bool, []*types.BIP340PubKey, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryShouldSubmitJurySigs", btcPubKey)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].([]*types.BIP340PubKey)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// QueryShouldSubmitJurySigs indicates an expected call of QueryShouldSubmitJurySigs.
func (mr *MockBabylonClientMockRecorder) QueryShouldSubmitJurySigs(btcPubKey interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryShouldSubmitJurySigs", reflect.TypeOf((*MockBabylonClient)(nil).QueryShouldSubmitJurySigs), btcPubKey)
}

// QueryShouldValidatorVote mocks base method.
func (m *MockBabylonClient) QueryShouldValidatorVote(btcPubKey *types.BIP340PubKey, blockHeight uint64) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryShouldValidatorVote", btcPubKey, blockHeight)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryShouldValidatorVote indicates an expected call of QueryShouldValidatorVote.
func (mr *MockBabylonClientMockRecorder) QueryShouldValidatorVote(btcPubKey, blockHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryShouldValidatorVote", reflect.TypeOf((*MockBabylonClient)(nil).QueryShouldValidatorVote), btcPubKey, blockHeight)
}

// RegisterValidator mocks base method.
func (m *MockBabylonClient) RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *types0.ProofOfPossession) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterValidator", bbnPubKey, btcPubKey, pop)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterValidator indicates an expected call of RegisterValidator.
func (mr *MockBabylonClientMockRecorder) RegisterValidator(bbnPubKey, btcPubKey, pop interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterValidator", reflect.TypeOf((*MockBabylonClient)(nil).RegisterValidator), bbnPubKey, btcPubKey, pop)
}

// SubmitFinalitySig mocks base method.
func (m *MockBabylonClient) SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitFinalitySig", btcPubKey, blockHeight, blockHash, sig)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitFinalitySig indicates an expected call of SubmitFinalitySig.
func (mr *MockBabylonClientMockRecorder) SubmitFinalitySig(btcPubKey, blockHeight, blockHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitFinalitySig", reflect.TypeOf((*MockBabylonClient)(nil).SubmitFinalitySig), btcPubKey, blockHeight, blockHash, sig)
}

// SubmitJurySig mocks base method.
func (m *MockBabylonClient) SubmitJurySig(btcPubKey, delPubKey *types.BIP340PubKey, sig *types.BIP340Signature) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitJurySig", btcPubKey, delPubKey, sig)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitJurySig indicates an expected call of SubmitJurySig.
func (mr *MockBabylonClientMockRecorder) SubmitJurySig(btcPubKey, delPubKey, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitJurySig", reflect.TypeOf((*MockBabylonClient)(nil).SubmitJurySig), btcPubKey, delPubKey, sig)
}
