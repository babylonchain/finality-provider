//go:build e2e_op
// +build e2e_op

package e2etest_op

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmdtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	bbnclient "github.com/babylonchain/babylon/client/client"
	bbncfg "github.com/babylonchain/babylon/client/config"
	bbntypes "github.com/babylonchain/babylon/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	bbncc "github.com/babylonchain/finality-provider/clientcontroller/babylon"
	"github.com/babylonchain/finality-provider/clientcontroller/opstackl2"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	e2eutils "github.com/babylonchain/finality-provider/itest"
	"github.com/babylonchain/finality-provider/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cometbft/cometbft/crypto/merkle"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdkquery "github.com/cosmos/cosmos-sdk/types/query"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const (
	opFinalityGadgetContractPath = "../bytecode/op_finality_gadget_f149c8b.wasm"
	opConsumerId                 = "op-stack-l2-12345"
)

type OpL2ConsumerTestManager struct {
	BabylonHandler    *e2eutils.BabylonNodeHandler
	BBNClient         *bbncc.BabylonController
	EOTSClient        *client.EOTSManagerGRpcClient
	EOTSConfig        *eotsconfig.Config
	EOTSServerHandler *e2eutils.EOTSServerHandler
	FpApp             *service.FinalityProviderApp
	FpConfig          *fpcfg.Config
	OpL2ConsumerCtrl  *opstackl2.OPStackL2ConsumerController
	StakingParams     *types.StakingParams
	CovenantPrivKeys  []*btcec.PrivateKey
	BaseDir           string
}

func StartOpL2ConsumerManager(t *testing.T) *OpL2ConsumerTestManager {
	// Setup consumer test manager
	testDir, err := e2eutils.BaseDir("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := e2eutils.GenerateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := e2eutils.NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := e2eutils.DefaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := bbncc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. register consumer to Babylon
	txRes, err := bc.RegisterConsumerChain(opConsumerId, opConsumerId, opConsumerId)
	require.NoError(t, err)
	t.Logf("Register consumer %s to Babylon %s", opConsumerId, txRes.TxHash)

	// 4. new op consumer controller
	opcc, err := opstackl2.NewOPStackL2ConsumerController(mockOpL2ConsumerCtrlConfig(bh.GetNodeDataDir()), logger)
	require.NoError(t, err)

	// 5. store op-finality-gadget contract
	err = storeWasmCode(opcc, opFinalityGadgetContractPath)
	require.NoError(t, err)

	opFinalityGadgetContractWasmId, err := getLatestCodeId(opcc)
	require.NoError(t, err)
	require.Equal(t, uint64(1), opFinalityGadgetContractWasmId, "first deployed contract code_id should be 1")

	// 6. instantiate op contract
	opFinalityGadgetInitMsg := map[string]interface{}{
		"admin":            opcc.CwClient.MustGetAddr(),
		"consumer_id":      opConsumerId,
		"activated_height": 0,
	}
	opFinalityGadgetInitMsgBytes, err := json.Marshal(opFinalityGadgetInitMsg)
	require.NoError(t, err)
	err = instantiateWasmContract(opcc, opFinalityGadgetContractWasmId, opFinalityGadgetInitMsgBytes)
	require.NoError(t, err)

	// get op contract address
	resp, err := opcc.CwClient.ListContractsByCode(opFinalityGadgetContractWasmId, &sdkquerytypes.PageRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Contracts, 1)

	// update the contract address in config to replace a placeholder address
	// previously used to bypass the validation
	opcc.Cfg.OPFinalityGadgetAddress = resp.Contracts[0]
	t.Logf("Deployed op finality contract address: %s", resp.Contracts[0])

	// 7. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := e2eutils.NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 8. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, opcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	ctm := &OpL2ConsumerTestManager{
		BabylonHandler:    bh,
		BBNClient:         bc,
		EOTSClient:        eotsCli,
		EOTSConfig:        eotsCfg,
		EOTSServerHandler: eh,
		FpApp:             fpApp,
		FpConfig:          cfg,
		OpL2ConsumerCtrl:  opcc,
		CovenantPrivKeys:  covenantPrivKeys,
		BaseDir:           testDir,
	}

	ctm.WaitForServicesStart(t)
	return ctm
}

func mockOpL2ConsumerCtrlConfig(nodeDataDir string) *fpcfg.OPStackL2Config {
	dc := bbncfg.DefaultBabylonConfig()

	// fill up the config from dc config
	return &fpcfg.OPStackL2Config{
		// it has to be a valid EVM RPC which doesn't timeout
		OPStackL2RPCAddress: "https://rpc.ankr.com/eth",
		// it has to be a valid addr that can be passed into `sdktypes.AccAddressFromBech32()`
		OPFinalityGadgetAddress: "bbn1ghd753shjuwexxywmgs4xz7x2q732vcnkm6h2pyv9s6ah3hylvrqxxvh0f",
		Key:                     dc.Key,
		ChainID:                 dc.ChainID,
		RPCAddr:                 dc.RPCAddr,
		GRPCAddr:                dc.GRPCAddr,
		AccountPrefix:           dc.AccountPrefix,
		KeyringBackend:          dc.KeyringBackend,
		KeyDirectory:            nodeDataDir,
		GasAdjustment:           1.5,
		GasPrices:               "0.002ubbn",
		Debug:                   dc.Debug,
		Timeout:                 dc.Timeout,
		// Setting this to relatively low value, out currnet babylon client (lens) will
		// block for this amout of time to wait for transaction inclusion in block
		BlockTimeout: 1 * time.Minute,
		OutputFormat: dc.OutputFormat,
		SignModeStr:  dc.SignModeStr,
	}
}

func (ctm *OpL2ConsumerTestManager) WaitForServicesStart(t *testing.T) {
	require.Eventually(t, func() bool {
		params, err := ctm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		ctm.StakingParams = params
		return true
	}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	t.Logf("Babylon node has started")
}

func (ctm *OpL2ConsumerTestManager) StartFinalityProvider(t *testing.T, n int) []*service.FinalityProviderInstance {
	app := ctm.FpApp

	for i := 0; i < n; i++ {
		fpName := e2eutils.FpNamePrefix + strconv.Itoa(i)
		moniker := e2eutils.MonikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := e2eutils.NewDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, e2eutils.Passphrase, e2eutils.HdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, opConsumerId, e2eutils.Passphrase, e2eutils.HdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		regRes, err := app.RegisterFinalityProvider(fpPk.MarshalHex())
		t.Logf("Registered Finality Provider %s", regRes.TxHash)
		require.NoError(t, err)
		err = app.StartHandlingFinalityProvider(fpPk, e2eutils.Passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPk)
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)

		// check finality providers on Babylon side
		require.Eventually(t, func() bool {
			fps, err := ctm.QueryFinalityProviders(opConsumerId)
			if err != nil {
				t.Logf("failed to query finality providers from Babylon %s", err.Error())
				return false
			}

			if len(fps) != i+1 {
				return false
			}

			for _, fp := range fps {
				if !strings.Contains(fp.Description.Moniker, e2eutils.MonikerPrefix) {
					return false
				}
				if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
					return false
				}
			}

			return true
		}, e2eutils.EventuallyWaitTimeOut, e2eutils.EventuallyPollTime)
	}

	fpInsList := app.ListFinalityProviderInstances()
	require.Equal(t, n, len(fpInsList))

	t.Logf("The test manager is running with %v finality-provider(s)", len(fpInsList))

	for i, fp := range fpInsList {
		t.Logf("The num %d finality-provider pubkeyhex %s", i, fp.GetBtcPkHex())
	}

	return fpInsList
}

type PubRandListInfo struct {
	PubRandList []*secp256k1.FieldVal
	Commitment  []byte
	ProofList   []*merkle.Proof
}

func (ctm *OpL2ConsumerTestManager) GenerateCommitPubRandListMsg(fpPk *bbntypes.BIP340PubKey, startHeight uint64, numPubRand uint64) (*PubRandListInfo, *ftypes.MsgCommitPubRandList, error) {
	// generate a list of Schnorr randomness pairs
	fp, err := ctm.FpApp.GetFinalityProviderInstance(fpPk)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get fp instance: %w", err)
	}
	pubRandList, err := fp.GetPubRandList(startHeight, numPubRand)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate randomness: %w", err)
	}
	// generate commitment and proof for each public randomness
	commitment, proofList := types.GetPubRandCommitAndProofs(pubRandList)

	// store them to database
	if err := ctm.FpApp.GetPubRandProofStore().AddPubRandProofList(pubRandList, proofList); err != nil {
		return nil, nil, fmt.Errorf("failed to save public randomness to DB: %w", err)
	}

	// sign the commitment
	schnorrSig, err := fp.SignPubRandCommit(startHeight, numPubRand, commitment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign the Schnorr signature: %w", err)
	}

	pubRandListInfo := &PubRandListInfo{
		PubRandList: pubRandList,
		Commitment:  commitment,
		ProofList:   proofList,
	}

	msg := &ftypes.MsgCommitPubRandList{
		Signer:      ctm.BBNClient.MustGetTxSigner(),
		FpBtcPk:     fpPk,
		StartHeight: startHeight,
		NumPubRand:  numPubRand,
		Commitment:  commitment,
		Sig:         bbntypes.NewBIP340SignatureFromBTCSig(schnorrSig),
	}
	return pubRandListInfo, msg, nil
}

func (ctm *OpL2ConsumerTestManager) CommitPubRandList(t *testing.T, fpPk *bbntypes.BIP340PubKey) (*PubRandListInfo, *ftypes.MsgCommitPubRandList) {
	// generate randomness data
	pubRandListInfo, msgPub, err := ctm.GenerateCommitPubRandListMsg(fpPk, 1, 100)
	require.NoError(t, err)

	commitRes, err := ctm.OpL2ConsumerCtrl.CommitPubRandList(
		msgPub.FpBtcPk.MustToBTCPK(),
		msgPub.StartHeight,
		msgPub.NumPubRand,
		msgPub.Commitment,
		msgPub.Sig.MustToBTCSig(),
	)
	require.NoError(t, err)
	t.Logf("Commit PubRandList to op finality contract %s", commitRes.TxHash)
	return pubRandListInfo, msgPub
}

func (ctm *OpL2ConsumerTestManager) WaitForFpPubRandCommitted(t *testing.T, fpPk *bbntypes.BIP340PubKey) {
	require.Eventually(t, func() bool {
		// query pub rand
		committedPubRandMap, err := ctm.OpL2ConsumerCtrl.QueryLastCommittedPublicRand(fpPk.MustToBTCPK(), 1)
		if err != nil {
			return false
		}
		for k, v := range committedPubRandMap {
			require.Equal(t, uint64(1), k)
			require.Equal(t, uint64(100), v.NumPubRand)
			break
		}
		return true
	}, e2etest.EventuallyWaitTimeOut, e2etest.EventuallyPollTime)
	t.Logf("Public randomness is successfully committed")
}

func (ctm *OpL2ConsumerTestManager) QueryFinalityProviders(consumerId string) ([]*bsctypes.FinalityProviderResponse, error) {
	var fps []*bsctypes.FinalityProviderResponse
	pagination := &sdkquery.PageRequest{
		Limit: 100,
	}

	bbnConfig := fpcfg.BBNConfigToBabylonConfig(ctm.FpConfig.BabylonConfig)
	logger := zap.NewNop()
	bc, err := bbnclient.New(
		&bbnConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	for {
		res, err := bc.QueryConsumerFinalityProviders(consumerId, pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to query finality providers: %v", err)
		}
		fps = append(fps, res.FinalityProviders...)
		if res.Pagination == nil || res.Pagination.NextKey == nil {
			break
		}

		pagination.Key = res.Pagination.NextKey
	}

	return fps, nil
}

func storeWasmCode(opcc *opstackl2.OPStackL2ConsumerController, wasmFile string) error {
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return err
	}
	if strings.HasSuffix(wasmFile, "wasm") { // compress for gas limit
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err = gz.Write(wasmCode)
		if err != nil {
			return err
		}
		err = gz.Close()
		if err != nil {
			return err
		}
		wasmCode = buf.Bytes()
	}

	storeMsg := &wasmdtypes.MsgStoreCode{
		Sender:       opcc.CwClient.MustGetAddr(),
		WASMByteCode: wasmCode,
	}
	_, err = opcc.ReliablySendMsg(storeMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

func instantiateWasmContract(opcc *opstackl2.OPStackL2ConsumerController, codeID uint64, initMsg []byte) error {
	instantiateMsg := &wasmdtypes.MsgInstantiateContract{
		Sender: opcc.CwClient.MustGetAddr(),
		Admin:  opcc.CwClient.MustGetAddr(),
		CodeID: codeID,
		Label:  "op-test",
		Msg:    initMsg,
		Funds:  nil,
	}

	_, err := opcc.ReliablySendMsg(instantiateMsg, nil, nil)
	if err != nil {
		return err
	}

	return nil
}

// returns the latest wasm code id.
func getLatestCodeId(opcc *opstackl2.OPStackL2ConsumerController) (uint64, error) {
	pagination := &sdkquery.PageRequest{
		Limit:   1,
		Reverse: true,
	}
	resp, err := opcc.CwClient.ListCodes(pagination)
	if err != nil {
		return 0, err
	}

	if len(resp.CodeInfos) == 0 {
		return 0, fmt.Errorf("no codes found")
	}

	return resp.CodeInfos[0].CodeID, nil
}

func (ctm *OpL2ConsumerTestManager) Stop(t *testing.T) {
	var err error
	// FpApp has to stop first or you will get "rpc error: desc = account xxx not found: key not found" error
	// b/c when Babylon daemon is stopped, FP won't be able to find the keyring backend
	err = ctm.FpApp.Stop()
	require.NoError(t, err)
	err = ctm.BabylonHandler.Stop()
	require.NoError(t, err)
	ctm.EOTSServerHandler.Stop()
	err = os.RemoveAll(ctm.BaseDir)
	require.NoError(t, err)
}
