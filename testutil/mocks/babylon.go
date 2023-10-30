// Code generated by MockGen. DO NOT EDIT.
// Source: clientcontroller/interface.go

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	types "github.com/babylonchain/btc-validator/types"
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

// CommitPubRandList mocks base method.
func (m *MockClientController) CommitPubRandList(valPk *btcec.PublicKey, startHeight uint64, pubRandList []*btcec.FieldVal, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitPubRandList", valPk, startHeight, pubRandList, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitPubRandList indicates an expected call of CommitPubRandList.
func (mr *MockClientControllerMockRecorder) CommitPubRandList(valPk, startHeight, pubRandList, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitPubRandList", reflect.TypeOf((*MockClientController)(nil).CommitPubRandList), valPk, startHeight, pubRandList, sig)
}

// QueryActivatedHeight mocks base method.
func (m *MockClientController) QueryActivatedHeight() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryActivatedHeight")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryActivatedHeight indicates an expected call of QueryActivatedHeight.
func (mr *MockClientControllerMockRecorder) QueryActivatedHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryActivatedHeight", reflect.TypeOf((*MockClientController)(nil).QueryActivatedHeight))
}

// QueryBestBlock mocks base method.
func (m *MockClientController) QueryBestBlock() (*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBestBlock")
	ret0, _ := ret[0].(*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBestBlock indicates an expected call of QueryBestBlock.
func (mr *MockClientControllerMockRecorder) QueryBestBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBestBlock", reflect.TypeOf((*MockClientController)(nil).QueryBestBlock))
}

// QueryBlock mocks base method.
func (m *MockClientController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlock", height)
	ret0, _ := ret[0].(*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlock indicates an expected call of QueryBlock.
func (mr *MockClientControllerMockRecorder) QueryBlock(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlock", reflect.TypeOf((*MockClientController)(nil).QueryBlock), height)
}

// QueryBlocks mocks base method.
func (m *MockClientController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlocks", startHeight, endHeight, limit)
	ret0, _ := ret[0].([]*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlocks indicates an expected call of QueryBlocks.
func (mr *MockClientControllerMockRecorder) QueryBlocks(startHeight, endHeight, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlocks", reflect.TypeOf((*MockClientController)(nil).QueryBlocks), startHeight, endHeight, limit)
}

// QueryLatestFinalizedBlocks mocks base method.
func (m *MockClientController) QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryLatestFinalizedBlocks", count)
	ret0, _ := ret[0].([]*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryLatestFinalizedBlocks indicates an expected call of QueryLatestFinalizedBlocks.
func (mr *MockClientControllerMockRecorder) QueryLatestFinalizedBlocks(count interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryLatestFinalizedBlocks", reflect.TypeOf((*MockClientController)(nil).QueryLatestFinalizedBlocks), count)
}

// QueryPendingDelegations mocks base method.
func (m *MockClientController) QueryPendingDelegations(limit uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryPendingDelegations", limit)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryPendingDelegations indicates an expected call of QueryPendingDelegations.
func (mr *MockClientControllerMockRecorder) QueryPendingDelegations(limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryPendingDelegations", reflect.TypeOf((*MockClientController)(nil).QueryPendingDelegations), limit)
}

// QueryUnbondingDelegations mocks base method.
func (m *MockClientController) QueryUnbondingDelegations(limit uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryUnbondingDelegations", limit)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryUnbondingDelegations indicates an expected call of QueryUnbondingDelegations.
func (mr *MockClientControllerMockRecorder) QueryUnbondingDelegations(limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryUnbondingDelegations", reflect.TypeOf((*MockClientController)(nil).QueryUnbondingDelegations), limit)
}

// QueryValidatorSlashed mocks base method.
func (m *MockClientController) QueryValidatorSlashed(valPk *btcec.PublicKey) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorSlashed", valPk)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorSlashed indicates an expected call of QueryValidatorSlashed.
func (mr *MockClientControllerMockRecorder) QueryValidatorSlashed(valPk interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorSlashed", reflect.TypeOf((*MockClientController)(nil).QueryValidatorSlashed), valPk)
}

// QueryValidatorUnbondingDelegations mocks base method.
func (m *MockClientController) QueryValidatorUnbondingDelegations(valPk *btcec.PublicKey, max uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorUnbondingDelegations", valPk, max)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorUnbondingDelegations indicates an expected call of QueryValidatorUnbondingDelegations.
func (mr *MockClientControllerMockRecorder) QueryValidatorUnbondingDelegations(valPk, max interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorUnbondingDelegations", reflect.TypeOf((*MockClientController)(nil).QueryValidatorUnbondingDelegations), valPk, max)
}

// QueryValidatorVotingPower mocks base method.
func (m *MockClientController) QueryValidatorVotingPower(valPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorVotingPower", valPk, blockHeight)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorVotingPower indicates an expected call of QueryValidatorVotingPower.
func (mr *MockClientControllerMockRecorder) QueryValidatorVotingPower(valPk, blockHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorVotingPower", reflect.TypeOf((*MockClientController)(nil).QueryValidatorVotingPower), valPk, blockHeight)
}

// RegisterValidator mocks base method.
func (m *MockClientController) RegisterValidator(chainPk []byte, valPk *btcec.PublicKey, pop []byte, commission, description string) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterValidator", chainPk, valPk, pop, commission, description)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterValidator indicates an expected call of RegisterValidator.
func (mr *MockClientControllerMockRecorder) RegisterValidator(chainPk, valPk, pop, commission, description interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterValidator", reflect.TypeOf((*MockClientController)(nil).RegisterValidator), chainPk, valPk, pop, commission, description)
}

// SubmitBatchFinalitySigs mocks base method.
func (m *MockClientController) SubmitBatchFinalitySigs(valPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitBatchFinalitySigs", valPk, blocks, sigs)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitBatchFinalitySigs indicates an expected call of SubmitBatchFinalitySigs.
func (mr *MockClientControllerMockRecorder) SubmitBatchFinalitySigs(valPk, blocks, sigs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitBatchFinalitySigs", reflect.TypeOf((*MockClientController)(nil).SubmitBatchFinalitySigs), valPk, blocks, sigs)
}

// SubmitFinalitySig mocks base method.
func (m *MockClientController) SubmitFinalitySig(valPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitFinalitySig", valPk, blockHeight, blockHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitFinalitySig indicates an expected call of SubmitFinalitySig.
func (mr *MockClientControllerMockRecorder) SubmitFinalitySig(valPk, blockHeight, blockHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitFinalitySig", reflect.TypeOf((*MockClientController)(nil).SubmitFinalitySig), valPk, blockHeight, blockHash, sig)
}

// SubmitJurySig mocks base method.
func (m *MockClientController) SubmitJurySig(valPk, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitJurySig", valPk, delPk, stakingTxHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitJurySig indicates an expected call of SubmitJurySig.
func (mr *MockClientControllerMockRecorder) SubmitJurySig(valPk, delPk, stakingTxHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitJurySig", reflect.TypeOf((*MockClientController)(nil).SubmitJurySig), valPk, delPk, stakingTxHash, sig)
}

// SubmitJuryUnbondingSigs mocks base method.
func (m *MockClientController) SubmitJuryUnbondingSigs(valPk, delPk *btcec.PublicKey, stakingTxHash string, unbondingSig, slashUnbondingSig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitJuryUnbondingSigs", valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitJuryUnbondingSigs indicates an expected call of SubmitJuryUnbondingSigs.
func (mr *MockClientControllerMockRecorder) SubmitJuryUnbondingSigs(valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitJuryUnbondingSigs", reflect.TypeOf((*MockClientController)(nil).SubmitJuryUnbondingSigs), valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig)
}

// SubmitValidatorUnbondingSig mocks base method.
func (m *MockClientController) SubmitValidatorUnbondingSig(valPk, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitValidatorUnbondingSig", valPk, delPk, stakingTxHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitValidatorUnbondingSig indicates an expected call of SubmitValidatorUnbondingSig.
func (mr *MockClientControllerMockRecorder) SubmitValidatorUnbondingSig(valPk, delPk, stakingTxHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitValidatorUnbondingSig", reflect.TypeOf((*MockClientController)(nil).SubmitValidatorUnbondingSig), valPk, delPk, stakingTxHash, sig)
}

// MockValidatorController is a mock of ValidatorController interface.
type MockValidatorController struct {
	ctrl     *gomock.Controller
	recorder *MockValidatorControllerMockRecorder
}

// MockValidatorControllerMockRecorder is the mock recorder for MockValidatorController.
type MockValidatorControllerMockRecorder struct {
	mock *MockValidatorController
}

// NewMockValidatorController creates a new mock instance.
func NewMockValidatorController(ctrl *gomock.Controller) *MockValidatorController {
	mock := &MockValidatorController{ctrl: ctrl}
	mock.recorder = &MockValidatorControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockValidatorController) EXPECT() *MockValidatorControllerMockRecorder {
	return m.recorder
}

// CommitPubRandList mocks base method.
func (m *MockValidatorController) CommitPubRandList(valPk *btcec.PublicKey, startHeight uint64, pubRandList []*btcec.FieldVal, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CommitPubRandList", valPk, startHeight, pubRandList, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CommitPubRandList indicates an expected call of CommitPubRandList.
func (mr *MockValidatorControllerMockRecorder) CommitPubRandList(valPk, startHeight, pubRandList, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CommitPubRandList", reflect.TypeOf((*MockValidatorController)(nil).CommitPubRandList), valPk, startHeight, pubRandList, sig)
}

// QueryActivatedHeight mocks base method.
func (m *MockValidatorController) QueryActivatedHeight() (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryActivatedHeight")
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryActivatedHeight indicates an expected call of QueryActivatedHeight.
func (mr *MockValidatorControllerMockRecorder) QueryActivatedHeight() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryActivatedHeight", reflect.TypeOf((*MockValidatorController)(nil).QueryActivatedHeight))
}

// QueryBestBlock mocks base method.
func (m *MockValidatorController) QueryBestBlock() (*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBestBlock")
	ret0, _ := ret[0].(*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBestBlock indicates an expected call of QueryBestBlock.
func (mr *MockValidatorControllerMockRecorder) QueryBestBlock() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBestBlock", reflect.TypeOf((*MockValidatorController)(nil).QueryBestBlock))
}

// QueryBlock mocks base method.
func (m *MockValidatorController) QueryBlock(height uint64) (*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlock", height)
	ret0, _ := ret[0].(*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlock indicates an expected call of QueryBlock.
func (mr *MockValidatorControllerMockRecorder) QueryBlock(height interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlock", reflect.TypeOf((*MockValidatorController)(nil).QueryBlock), height)
}

// QueryBlocks mocks base method.
func (m *MockValidatorController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryBlocks", startHeight, endHeight, limit)
	ret0, _ := ret[0].([]*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryBlocks indicates an expected call of QueryBlocks.
func (mr *MockValidatorControllerMockRecorder) QueryBlocks(startHeight, endHeight, limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryBlocks", reflect.TypeOf((*MockValidatorController)(nil).QueryBlocks), startHeight, endHeight, limit)
}

// QueryLatestFinalizedBlocks mocks base method.
func (m *MockValidatorController) QueryLatestFinalizedBlocks(count uint64) ([]*types.BlockInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryLatestFinalizedBlocks", count)
	ret0, _ := ret[0].([]*types.BlockInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryLatestFinalizedBlocks indicates an expected call of QueryLatestFinalizedBlocks.
func (mr *MockValidatorControllerMockRecorder) QueryLatestFinalizedBlocks(count interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryLatestFinalizedBlocks", reflect.TypeOf((*MockValidatorController)(nil).QueryLatestFinalizedBlocks), count)
}

// QueryValidatorSlashed mocks base method.
func (m *MockValidatorController) QueryValidatorSlashed(valPk *btcec.PublicKey) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorSlashed", valPk)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorSlashed indicates an expected call of QueryValidatorSlashed.
func (mr *MockValidatorControllerMockRecorder) QueryValidatorSlashed(valPk interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorSlashed", reflect.TypeOf((*MockValidatorController)(nil).QueryValidatorSlashed), valPk)
}

// QueryValidatorUnbondingDelegations mocks base method.
func (m *MockValidatorController) QueryValidatorUnbondingDelegations(valPk *btcec.PublicKey, max uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorUnbondingDelegations", valPk, max)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorUnbondingDelegations indicates an expected call of QueryValidatorUnbondingDelegations.
func (mr *MockValidatorControllerMockRecorder) QueryValidatorUnbondingDelegations(valPk, max interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorUnbondingDelegations", reflect.TypeOf((*MockValidatorController)(nil).QueryValidatorUnbondingDelegations), valPk, max)
}

// QueryValidatorVotingPower mocks base method.
func (m *MockValidatorController) QueryValidatorVotingPower(valPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryValidatorVotingPower", valPk, blockHeight)
	ret0, _ := ret[0].(uint64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryValidatorVotingPower indicates an expected call of QueryValidatorVotingPower.
func (mr *MockValidatorControllerMockRecorder) QueryValidatorVotingPower(valPk, blockHeight interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryValidatorVotingPower", reflect.TypeOf((*MockValidatorController)(nil).QueryValidatorVotingPower), valPk, blockHeight)
}

// RegisterValidator mocks base method.
func (m *MockValidatorController) RegisterValidator(chainPk []byte, valPk *btcec.PublicKey, pop []byte, commission, description string) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RegisterValidator", chainPk, valPk, pop, commission, description)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RegisterValidator indicates an expected call of RegisterValidator.
func (mr *MockValidatorControllerMockRecorder) RegisterValidator(chainPk, valPk, pop, commission, description interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RegisterValidator", reflect.TypeOf((*MockValidatorController)(nil).RegisterValidator), chainPk, valPk, pop, commission, description)
}

// SubmitBatchFinalitySigs mocks base method.
func (m *MockValidatorController) SubmitBatchFinalitySigs(valPk *btcec.PublicKey, blocks []*types.BlockInfo, sigs []*btcec.ModNScalar) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitBatchFinalitySigs", valPk, blocks, sigs)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitBatchFinalitySigs indicates an expected call of SubmitBatchFinalitySigs.
func (mr *MockValidatorControllerMockRecorder) SubmitBatchFinalitySigs(valPk, blocks, sigs interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitBatchFinalitySigs", reflect.TypeOf((*MockValidatorController)(nil).SubmitBatchFinalitySigs), valPk, blocks, sigs)
}

// SubmitFinalitySig mocks base method.
func (m *MockValidatorController) SubmitFinalitySig(valPk *btcec.PublicKey, blockHeight uint64, blockHash []byte, sig *btcec.ModNScalar) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitFinalitySig", valPk, blockHeight, blockHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitFinalitySig indicates an expected call of SubmitFinalitySig.
func (mr *MockValidatorControllerMockRecorder) SubmitFinalitySig(valPk, blockHeight, blockHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitFinalitySig", reflect.TypeOf((*MockValidatorController)(nil).SubmitFinalitySig), valPk, blockHeight, blockHash, sig)
}

// SubmitValidatorUnbondingSig mocks base method.
func (m *MockValidatorController) SubmitValidatorUnbondingSig(valPk, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitValidatorUnbondingSig", valPk, delPk, stakingTxHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitValidatorUnbondingSig indicates an expected call of SubmitValidatorUnbondingSig.
func (mr *MockValidatorControllerMockRecorder) SubmitValidatorUnbondingSig(valPk, delPk, stakingTxHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitValidatorUnbondingSig", reflect.TypeOf((*MockValidatorController)(nil).SubmitValidatorUnbondingSig), valPk, delPk, stakingTxHash, sig)
}

// MockJuryController is a mock of JuryController interface.
type MockJuryController struct {
	ctrl     *gomock.Controller
	recorder *MockJuryControllerMockRecorder
}

// MockJuryControllerMockRecorder is the mock recorder for MockJuryController.
type MockJuryControllerMockRecorder struct {
	mock *MockJuryController
}

// NewMockJuryController creates a new mock instance.
func NewMockJuryController(ctrl *gomock.Controller) *MockJuryController {
	mock := &MockJuryController{ctrl: ctrl}
	mock.recorder = &MockJuryControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockJuryController) EXPECT() *MockJuryControllerMockRecorder {
	return m.recorder
}

// QueryPendingDelegations mocks base method.
func (m *MockJuryController) QueryPendingDelegations(limit uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryPendingDelegations", limit)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryPendingDelegations indicates an expected call of QueryPendingDelegations.
func (mr *MockJuryControllerMockRecorder) QueryPendingDelegations(limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryPendingDelegations", reflect.TypeOf((*MockJuryController)(nil).QueryPendingDelegations), limit)
}

// QueryUnbondingDelegations mocks base method.
func (m *MockJuryController) QueryUnbondingDelegations(limit uint64) ([]*types.Delegation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "QueryUnbondingDelegations", limit)
	ret0, _ := ret[0].([]*types.Delegation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// QueryUnbondingDelegations indicates an expected call of QueryUnbondingDelegations.
func (mr *MockJuryControllerMockRecorder) QueryUnbondingDelegations(limit interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "QueryUnbondingDelegations", reflect.TypeOf((*MockJuryController)(nil).QueryUnbondingDelegations), limit)
}

// SubmitJurySig mocks base method.
func (m *MockJuryController) SubmitJurySig(valPk, delPk *btcec.PublicKey, stakingTxHash string, sig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitJurySig", valPk, delPk, stakingTxHash, sig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitJurySig indicates an expected call of SubmitJurySig.
func (mr *MockJuryControllerMockRecorder) SubmitJurySig(valPk, delPk, stakingTxHash, sig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitJurySig", reflect.TypeOf((*MockJuryController)(nil).SubmitJurySig), valPk, delPk, stakingTxHash, sig)
}

// SubmitJuryUnbondingSigs mocks base method.
func (m *MockJuryController) SubmitJuryUnbondingSigs(valPk, delPk *btcec.PublicKey, stakingTxHash string, unbondingSig, slashUnbondingSig *schnorr.Signature) (*types.TxResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubmitJuryUnbondingSigs", valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig)
	ret0, _ := ret[0].(*types.TxResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubmitJuryUnbondingSigs indicates an expected call of SubmitJuryUnbondingSigs.
func (mr *MockJuryControllerMockRecorder) SubmitJuryUnbondingSigs(valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubmitJuryUnbondingSigs", reflect.TypeOf((*MockJuryController)(nil).SubmitJuryUnbondingSigs), valPk, delPk, stakingTxHash, unbondingSig, slashUnbondingSig)
}
