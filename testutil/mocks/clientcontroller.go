// Code generated by MockGen. DO NOT EDIT.
// Source: clientcontroller/interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	math "cosmossdk.io/math"
	types "github.com/babylonchain/babylon/x/finality/types"
	types0 "github.com/babylonchain/finality-provider/types"
	btcec "github.com/btcsuite/btcd/btcec/v2"
	schnorr "github.com/btcsuite/btcd/btcec/v2/schnorr"
	gomock "github.com/golang/mock/gomock"
)

// MockClientController is a mock of ClientController interface.
type MockClientController struct {
	ctrl     *gomock.Controller
	recorder *MockClientControllerMockRecorder
}

// MockClientControllerMockRecorder is the mock recorder for MockClientController.
type MockClientControllerMockRecorder struct {
	mock *MockClientController
}

// NewMockClientController creates a new mock instance.
func NewMockClientController(ctrl *gomock.Controller) *MockClientController {
	mock := &MockClientController{ctrl: ctrl}
	mock.recorder = &MockClientControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientController) EXPECT() *MockClientControllerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockClientController) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockClientControllerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockClientController)(nil).Close))
}

// QueryFinalityProviderSlashed mocks base method.
func (m *MockClientController) QueryFinalityProviderSlashed(fpPk *btcec.PublicKey) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryFinalityProviderSlashed", fpPk)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryFinalityProviderSlashed indicates an expected call of QueryFinalityProviderSlashed.
func (mr *MockClientControllerMockRecorder) QueryFinalityProviderSlashed(fpPk interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryFinalityProviderSlashed", reflect.TypeOf((*MockClientController)(nil).QueryFinalityProviderSlashed), fpPk)
}

// RegisterFinalityProvider mocks base method.
func (m *MockClientController) RegisterFinalityProvider(chainID string, chainPk []byte, fpPk *btcec.PublicKey, pop []byte, commission *math.LegacyDec, description []byte) (*types0.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterFinalityProvider", chainID, chainPk, fpPk, pop, commission, description)
	ret0, _ := ret[0].(*types0.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterFinalityProvider indicates an expected call of RegisterFinalityProvider.
func (mr *MockClientControllerMockRecorder) RegisterFinalityProvider(chainID, chainPk, fpPk, pop, commission, description interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterFinalityProvider", reflect.TypeOf((*MockClientController)(nil).RegisterFinalityProvider), chainID, chainPk, fpPk, pop, commission, description)
}

// MockConsumerController is a mock of ConsumerController interface.
type MockConsumerController struct {
	ctrl     *gomock.Controller
	recorder *MockConsumerControllerMockRecorder
}

// MockConsumerControllerMockRecorder is the mock recorder for MockConsumerController.
type MockConsumerControllerMockRecorder struct {
	mock *MockConsumerController
}

// NewMockConsumerController creates a new mock instance.
func NewMockConsumerController(ctrl *gomock.Controller) *MockConsumerController {
	mock := &MockConsumerController{ctrl: ctrl}
	mock.recorder = &MockConsumerControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockConsumerController) EXPECT() *MockConsumerControllerMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockConsumerController) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockConsumerControllerMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockConsumerController)(nil).Close))
}

// CommitPubRandList mocks base method.
func (m *MockConsumerController) CommitPubRandList(fpPk *btcec.PublicKey, startHeight, numPubRand uint64, commitment []byte, sig *schnorr.Signature) (*types0.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitPubRandList", fpPk, startHeight, numPubRand, commitment, sig)
	ret0, _ := ret[0].(*types0.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitPubRandList indicates an expected call of CommitPubRandList.
func (mr *MockConsumerControllerMockRecorder) CommitPubRandList(fpPk, startHeight, numPubRand, commitment, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitPubRandList", reflect.TypeOf((*MockConsumerController)(nil).CommitPubRandList), fpPk, startHeight, numPubRand, commitment, sig)
}

// QueryActivatedHeight mocks base method.
func (m *MockConsumerController) QueryActivatedHeight() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryActivatedHeight")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryActivatedHeight indicates an expected call of QueryActivatedHeight.
func (mr *MockConsumerControllerMockRecorder) QueryActivatedHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryActivatedHeight", reflect.TypeOf((*MockConsumerController)(nil).QueryActivatedHeight))
}

// QueryBlock mocks base method.
func (m *MockConsumerController) QueryBlock(height uint64) (*types0.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlock", height)
	ret0, _ := ret[0].(*types0.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlock indicates an expected call of QueryBlock.
func (mr *MockConsumerControllerMockRecorder) QueryBlock(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlock", reflect.TypeOf((*MockConsumerController)(nil).QueryBlock), height)
}

// QueryBlocks mocks base method.
func (m *MockConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types0.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlocks", startHeight, endHeight, limit)
	ret0, _ := ret[0].([]*types0.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlocks indicates an expected call of QueryBlocks.
func (mr *MockConsumerControllerMockRecorder) QueryBlocks(startHeight, endHeight, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlocks", reflect.TypeOf((*MockConsumerController)(nil).QueryBlocks), startHeight, endHeight, limit)
}

// QueryFinalityProviderVotingPower mocks base method.
func (m *MockConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryFinalityProviderVotingPower", fpPk, blockHeight)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryFinalityProviderVotingPower indicates an expected call of QueryFinalityProviderVotingPower.
func (mr *MockConsumerControllerMockRecorder) QueryFinalityProviderVotingPower(fpPk, blockHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryFinalityProviderVotingPower", reflect.TypeOf((*MockConsumerController)(nil).QueryFinalityProviderVotingPower), fpPk, blockHeight)
}

// QueryIsBlockFinalized mocks base method.
func (m *MockConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryIsBlockFinalized", height)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryIsBlockFinalized indicates an expected call of QueryIsBlockFinalized.
func (mr *MockConsumerControllerMockRecorder) QueryIsBlockFinalized(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryIsBlockFinalized", reflect.TypeOf((*MockConsumerController)(nil).QueryIsBlockFinalized), height)
}

// QueryLastCommittedPublicRand mocks base method.
func (m *MockConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*types.PubRandCommitResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryLastCommittedPublicRand", fpPk, count)
	ret0, _ := ret[0].(map[uint64]*types.PubRandCommitResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryLastCommittedPublicRand indicates an expected call of QueryLastCommittedPublicRand.
func (mr *MockConsumerControllerMockRecorder) QueryLastCommittedPublicRand(fpPk, count interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryLastCommittedPublicRand", reflect.TypeOf((*MockConsumerController)(nil).QueryLastCommittedPublicRand), fpPk, count)
}

// QueryLatestBlockHeight mocks base method.
func (m *MockConsumerController) QueryLatestBlockHeight() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryLatestBlockHeight")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryLatestBlockHeight indicates an expected call of QueryLatestBlockHeight.
func (mr *MockConsumerControllerMockRecorder) QueryLatestBlockHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryLatestBlockHeight", reflect.TypeOf((*MockConsumerController)(nil).QueryLatestBlockHeight))
}

// QueryLatestFinalizedBlock mocks base method.
func (m *MockConsumerController) QueryLatestFinalizedBlock() (*types0.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryLatestFinalizedBlock")
	ret0, _ := ret[0].(*types0.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryLatestFinalizedBlock indicates an expected call of QueryLatestFinalizedBlock.
func (mr *MockConsumerControllerMockRecorder) QueryLatestFinalizedBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryLatestFinalizedBlock", reflect.TypeOf((*MockConsumerController)(nil).QueryLatestFinalizedBlock))
}

// SubmitBatchFinalitySigs mocks base method.
func (m *MockConsumerController) SubmitBatchFinalitySigs(fpPk *btcec.PublicKey, blocks []*types0.BlockInfo, pubRandList []*btcec.FieldVal, proofList [][]byte, sigs []*btcec.ModNScalar) (*types0.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitBatchFinalitySigs", fpPk, blocks, pubRandList, proofList, sigs)
	ret0, _ := ret[0].(*types0.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitBatchFinalitySigs indicates an expected call of SubmitBatchFinalitySigs.
func (mr *MockConsumerControllerMockRecorder) SubmitBatchFinalitySigs(fpPk, blocks, pubRandList, proofList, sigs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitBatchFinalitySigs", reflect.TypeOf((*MockConsumerController)(nil).SubmitBatchFinalitySigs), fpPk, blocks, pubRandList, proofList, sigs)
}

// SubmitFinalitySig mocks base method.
func (m *MockConsumerController) SubmitFinalitySig(fpPk *btcec.PublicKey, block *types0.BlockInfo, pubRand *btcec.FieldVal, proof []byte, sig *btcec.ModNScalar) (*types0.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitFinalitySig", fpPk, block, pubRand, proof, sig)
	ret0, _ := ret[0].(*types0.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitFinalitySig indicates an expected call of SubmitFinalitySig.
func (mr *MockConsumerControllerMockRecorder) SubmitFinalitySig(fpPk, block, pubRand, proof, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitFinalitySig", reflect.TypeOf((*MockConsumerController)(nil).SubmitFinalitySig), fpPk, block, pubRand, proof, sig)
}
