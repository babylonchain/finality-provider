package e2etest

import (
	"encoding/hex"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonchain/babylon/btcstaking"
	txformat "github.com/babylonchain/babylon/btctxformatter"
	asig "github.com/babylonchain/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonchain/babylon/testutil/datagen"
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	fpcc "github.com/babylonchain/finality-provider/clientcontroller"
	"github.com/babylonchain/finality-provider/eotsmanager/client"
	eotsconfig "github.com/babylonchain/finality-provider/eotsmanager/config"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/finality-provider/service"
	"github.com/babylonchain/finality-provider/types"
)

var (
	eventuallyWaitTimeOut = 1 * time.Minute
	eventuallyPollTime    = 500 * time.Millisecond
	btcNetworkParams      = &chaincfg.SimNetParams

	fpNamePrefix  = "test-fp-"
	monikerPrefix = "moniker-"
	chainID       = "chain-test"
	passphrase    = "testpass"
	hdPath        = ""
	simnetParams  = &chaincfg.SimNetParams
)

type TestManager struct {
	Wg                sync.WaitGroup
	BabylonHandler    *BabylonNodeHandler
	EOTSServerHandler *EOTSServerHandler
	FpConfig          *fpcfg.Config
	EOTSConfig        *eotsconfig.Config
	Fpa               *service.FinalityProviderApp
	EOTSClient        *client.EOTSManagerGRpcClient
	BBNClient         *fpcc.BabylonController
	BBNConsumerClient *fpcc.BabylonConsumerController
	StakingParams     *types.StakingParams
	CovenantPrivKeys  []*btcec.PrivateKey
	baseDir           string
}

type TestDelegationData struct {
	DelegatorPrivKey        *btcec.PrivateKey
	DelegatorKey            *btcec.PublicKey
	DelegatorBabylonPrivKey *secp256k1.PrivKey
	DelegatorBabylonKey     *secp256k1.PubKey
	SlashingTx              *bstypes.BTCSlashingTx
	StakingTx               *wire.MsgTx
	StakingTxInfo           *btcctypes.TransactionInfo
	DelegatorSig            *bbntypes.BIP340Signature
	FpPks                   []*btcec.PublicKey

	SlashingAddr  string
	ChangeAddr    string
	StakingTime   uint16
	StakingAmount int64
}

func StartManager(t *testing.T) *TestManager {
	testDir, err := tempDirWithName("fpe2etest")
	require.NoError(t, err)

	logger := zap.NewNop()

	// 1. generate covenant committee
	covenantQuorum := 2
	numCovenants := 3
	covenantPrivKeys, covenantPubKeys := generateCovenantCommittee(numCovenants, t)

	// 2. prepare Babylon node
	bh := NewBabylonNodeHandler(t, covenantQuorum, covenantPubKeys)
	err = bh.Start()
	require.NoError(t, err)
	fpHomeDir := filepath.Join(testDir, "fp-home")
	cfg := defaultFpConfig(bh.GetNodeDataDir(), fpHomeDir)
	bc, err := fpcc.NewBabylonController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)
	bcc, err := fpcc.NewBabylonConsumerController(cfg.BabylonConfig, &cfg.BTCNetParams, logger)
	require.NoError(t, err)

	// 3. prepare EOTS manager
	eotsHomeDir := filepath.Join(testDir, "eots-home")
	eotsCfg := eotsconfig.DefaultConfigWithHomePath(eotsHomeDir)
	eh := NewEOTSServerHandler(t, eotsCfg, eotsHomeDir)
	eh.Start()
	eotsCli, err := client.NewEOTSManagerGRpcClient(cfg.EOTSManagerAddress)
	require.NoError(t, err)

	// 4. prepare finality-provider
	fpdb, err := cfg.DatabaseConfig.GetDbBackend()
	require.NoError(t, err)
	fpApp, err := service.NewFinalityProviderApp(cfg, bc, bcc, eotsCli, fpdb, logger)
	require.NoError(t, err)
	err = fpApp.Start()
	require.NoError(t, err)

	tm := &TestManager{
		BabylonHandler:    bh,
		EOTSServerHandler: eh,
		FpConfig:          cfg,
		EOTSConfig:        eotsCfg,
		Fpa:               fpApp,
		EOTSClient:        eotsCli,
		BBNClient:         bc,
		BBNConsumerClient: bcc,
		CovenantPrivKeys:  covenantPrivKeys,
		baseDir:           testDir,
	}

	tm.WaitForServicesStart(t)

	return tm
}

func (tm *TestManager) WaitForServicesStart(t *testing.T) {
	// wait for Babylon node starts
	require.Eventually(t, func() bool {
		params, err := tm.BBNClient.QueryStakingParams()
		if err != nil {
			return false
		}
		tm.StakingParams = params
		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("Babylon node is started")
}

func StartManagerWithFinalityProvider(t *testing.T, n int) (*TestManager, []*service.FinalityProviderInstance, uint64) {
	tm := StartManager(t)
	app := tm.Fpa

	// register all finality providers
	registeredEpoch := uint64(0)
	fpPKs := make([]*bbntypes.BIP340PubKey, 0, n)
	for i := 0; i < n; i++ {
		fpName := fpNamePrefix + strconv.Itoa(i)
		moniker := monikerPrefix + strconv.Itoa(i)
		commission := sdkmath.LegacyZeroDec()
		desc := newDescription(moniker)
		cfg := app.GetConfig()
		_, err := service.CreateChainKey(cfg.BabylonConfig.KeyDirectory, cfg.BabylonConfig.ChainID, fpName, keyring.BackendTest, passphrase, hdPath, "")
		require.NoError(t, err)
		res, err := app.CreateFinalityProvider(fpName, chainID, passphrase, hdPath, desc, &commission)
		require.NoError(t, err)
		fpPk, err := bbntypes.NewBIP340PubKeyFromHex(res.FpInfo.BtcPkHex)
		require.NoError(t, err)
		fpPKs = append(fpPKs, fpPk)
		resp, err := app.RegisterFinalityProvider(fpPk.MarshalHex())
		require.NoError(t, err)
		registeredEpoch = resp.RegisteredEpoch // last registered epoch
	}

	// wait until the last registered epoch is finalised
	tm.FinalizeUntilEpoch(t, registeredEpoch)

	for i := 0; i < n; i++ {
		// start
		err := app.StartHandlingFinalityProvider(fpPKs[i], passphrase)
		require.NoError(t, err)
		fpIns, err := app.GetFinalityProviderInstance(fpPKs[i])
		require.NoError(t, err)
		require.True(t, fpIns.IsRunning())
		require.NoError(t, err)
	}

	// check finality providers on Babylon side
	require.Eventually(t, func() bool {
		fps, err := tm.BBNClient.QueryFinalityProviders()
		if err != nil {
			t.Logf("failed to query finality providers from Babylon %s", err.Error())
			return false
		}

		if len(fps) != n {
			return false
		}

		for _, fp := range fps {
			if !strings.Contains(fp.Description.Moniker, monikerPrefix) {
				return false
			}
			if !fp.Commission.Equal(sdkmath.LegacyZeroDec()) {
				return false
			}
		}

		return true
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	fpInsList := app.ListFinalityProviderInstances()
	require.Equal(t, n, len(fpInsList))

	t.Logf("the test manager is running with %v finality-provider(s)", len(fpInsList))

	return tm, fpInsList, registeredEpoch
}

func (tm *TestManager) Stop(t *testing.T) {
	err := tm.Fpa.Stop()
	require.NoError(t, err)
	err = tm.BabylonHandler.Stop()
	require.NoError(t, err)
	err = os.RemoveAll(tm.baseDir)
	require.NoError(t, err)
	tm.EOTSServerHandler.Stop()
}

func (tm *TestManager) WaitForFpRegistered(t *testing.T, bbnPk *secp256k1.PubKey) {
	require.Eventually(t, func() bool {
		queriedFps, err := tm.BBNClient.QueryFinalityProviders()
		if err != nil {
			return false
		}
		return len(queriedFps) == 1 && queriedFps[0].BabylonPk.Equals(bbnPk)
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the finality-provider is successfully registered")
}

func (tm *TestManager) WaitForNPendingDels(t *testing.T, n int) []*bstypes.BTCDelegationResponse {
	var (
		dels []*bstypes.BTCDelegationResponse
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BBNClient.QueryPendingDelegations(
			100,
		)
		if err != nil {
			return false
		}
		return len(dels) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("delegations are pending")

	return dels
}

func (tm *TestManager) WaitForNActiveDels(t *testing.T, n int) []*bstypes.BTCDelegationResponse {
	var (
		dels []*bstypes.BTCDelegationResponse
		err  error
	)
	require.Eventually(t, func() bool {
		dels, err = tm.BBNClient.QueryActiveDelegations(
			100,
		)
		if err != nil {
			return false
		}
		return len(dels) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("delegations are active")

	return dels
}

func generateCovenantCommittee(numCovenants int, t *testing.T) ([]*btcec.PrivateKey, []*bbntypes.BIP340PubKey) {
	var (
		covenantPrivKeys []*btcec.PrivateKey
		covenantPubKeys  []*bbntypes.BIP340PubKey
	)

	for i := 0; i < numCovenants; i++ {
		privKey, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		covenantPrivKeys = append(covenantPrivKeys, privKey)
		pubKey := bbntypes.NewBIP340PubKeyFromBTCPK(privKey.PubKey())
		covenantPubKeys = append(covenantPubKeys, pubKey)
	}

	return covenantPrivKeys, covenantPubKeys
}

func (tm *TestManager) CheckBlockFinalization(t *testing.T, height uint64, num int) {
	// we need to ensure votes are collected at the given height
	require.Eventually(t, func() bool {
		votes, err := tm.BBNClient.QueryVotesAtHeight(height)
		if err != nil {
			t.Logf("failed to get the votes at height %v: %s", height, err.Error())
			return false
		}
		return len(votes) == num
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	// as the votes have been collected, the block should be finalized
	require.Eventually(t, func() bool {
		finalized, err := tm.BBNConsumerClient.QueryIsBlockFinalized(height)
		if err != nil {
			t.Logf("failed to query block at height %v: %s", height, err.Error())
			return false
		}
		return finalized
	}, eventuallyWaitTimeOut, eventuallyPollTime)
}

func (tm *TestManager) WaitForFpVoteCast(t *testing.T, fpIns *service.FinalityProviderInstance) uint64 {
	var lastVotedHeight uint64
	require.Eventually(t, func() bool {
		if fpIns.GetLastVotedHeight() > 0 {
			lastVotedHeight = fpIns.GetLastVotedHeight()
			return true
		} else {
			return false
		}
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	return lastVotedHeight
}

func (tm *TestManager) WaitForNFinalizedBlocks(t *testing.T, n int) []*types.BlockInfo {
	var (
		blocks []*types.BlockInfo
		err    error
	)
	require.Eventually(t, func() bool {
		blocks, err = tm.BBNConsumerClient.QueryLatestFinalizedBlocks(uint64(n))
		if err != nil {
			t.Logf("failed to get the latest finalized block: %s", err.Error())
			return false
		}
		return len(blocks) == n
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the block is finalized at %v", blocks[0].Height)

	return blocks
}

func (tm *TestManager) WaitForFpShutDown(t *testing.T, pk *bbntypes.BIP340PubKey) {
	require.Eventually(t, func() bool {
		_, err := tm.Fpa.GetFinalityProviderInstance(pk)
		return err != nil
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Logf("the finality-provider instance %s is shutdown", pk.MarshalHex())
}

func (tm *TestManager) StopAndRestartFpAfterNBlocks(t *testing.T, n int, fpIns *service.FinalityProviderInstance) {
	blockBeforeStopHeight, err := tm.BBNConsumerClient.QueryLatestBlockHeight()
	require.NoError(t, err)
	err = fpIns.Stop()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		headerAfterStopHeight, err := tm.BBNConsumerClient.QueryLatestBlockHeight()
		if err != nil {
			return false
		}

		return headerAfterStopHeight >= uint64(n)+blockBeforeStopHeight
	}, eventuallyWaitTimeOut, eventuallyPollTime)

	t.Log("restarting the finality-provider instance")

	tm.FpConfig.PollerConfig.AutoChainScanningMode = true
	err = fpIns.Start()
	require.NoError(t, err)
}

func (tm *TestManager) GetFpPrivKey(t *testing.T, fpPk []byte) *btcec.PrivateKey {
	record, err := tm.EOTSClient.KeyRecord(fpPk, passphrase)
	require.NoError(t, err)
	return record.PrivKey
}

func (tm *TestManager) InsertCovenantSigForDelegation(t *testing.T, btcDel *bstypes.BTCDelegation) {
	slashingTx := btcDel.SlashingTx
	stakingTx := btcDel.StakingTx
	stakingMsgTx, err := bbntypes.NewBTCTxFromBytes(stakingTx)
	require.NoError(t, err)

	params := tm.StakingParams

	stakingInfo, err := btcstaking.BuildStakingInfo(
		btcDel.BtcPk.MustToBTCPK(),
		// TODO: Handle multiple providers
		[]*btcec.PublicKey{btcDel.FpBtcPkList[0].MustToBTCPK()},
		params.CovenantPks,
		params.CovenantQuorum,
		btcDel.GetStakingTime(),
		btcutil.Amount(btcDel.TotalSat),
		simnetParams,
	)
	require.NoError(t, err)
	stakingTxUnbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	idx, err := bbntypes.GetOutputIdxInBTCTx(stakingMsgTx, stakingInfo.StakingOutput)
	require.NoError(t, err)

	require.NoError(t, err)
	slashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	// get covenant private key from the keyring
	valEncKey, err := asig.NewEncryptionKeyFromBTCPK(btcDel.FpBtcPkList[0].MustToBTCPK())
	require.NoError(t, err)

	unbondingMsgTx, err := bbntypes.NewBTCTxFromBytes(btcDel.BtcUndelegation.UnbondingTx)
	require.NoError(t, err)
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		btcDel.BtcPk.MustToBTCPK(),
		[]*btcec.PublicKey{btcDel.FpBtcPkList[0].MustToBTCPK()},
		params.CovenantPks,
		params.CovenantQuorum,
		uint16(btcDel.UnbondingTime),
		btcutil.Amount(unbondingMsgTx.TxOut[0].Value),
		simnetParams,
	)
	require.NoError(t, err)

	// Covenant 0 signatures
	covenantAdaptorStakingSlashing1, err := slashingTx.EncSign(
		stakingMsgTx,
		idx,
		slashingPathInfo.RevealedLeaf.Script,
		tm.CovenantPrivKeys[0],
		valEncKey,
	)
	require.NoError(t, err)
	covenantUnbondingSig1, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
		unbondingMsgTx,
		stakingInfo.StakingOutput,
		tm.CovenantPrivKeys[0],
		stakingTxUnbondingPathInfo.RevealedLeaf,
	)
	require.NoError(t, err)

	// slashing unbonding tx sig
	unbondingTxSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)
	covenantAdaptorUnbondingSlashing1, err := btcDel.BtcUndelegation.SlashingTx.EncSign(
		unbondingMsgTx,
		0,
		unbondingTxSlashingPathInfo.RevealedLeaf.Script,
		tm.CovenantPrivKeys[0],
		valEncKey,
	)
	require.NoError(t, err)

	_, err = tm.BBNClient.SubmitCovenantSigs(
		tm.CovenantPrivKeys[0].PubKey(),
		stakingMsgTx.TxHash().String(),
		[][]byte{covenantAdaptorStakingSlashing1.MustMarshal()},
		covenantUnbondingSig1,
		[][]byte{covenantAdaptorUnbondingSlashing1.MustMarshal()},
	)
	require.NoError(t, err)

	// Covenant 1 signatures
	covenantAdaptorStakingSlashing2, err := slashingTx.EncSign(
		stakingMsgTx,
		idx,
		slashingPathInfo.RevealedLeaf.Script,
		tm.CovenantPrivKeys[1],
		valEncKey,
	)
	require.NoError(t, err)
	covenantUnbondingSig2, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
		unbondingMsgTx,
		stakingInfo.StakingOutput,
		tm.CovenantPrivKeys[1],
		stakingTxUnbondingPathInfo.RevealedLeaf,
	)
	require.NoError(t, err)

	// slashing unbonding tx sig

	covenantAdaptorUnbondingSlashing2, err := btcDel.BtcUndelegation.SlashingTx.EncSign(
		unbondingMsgTx,
		0,
		unbondingTxSlashingPathInfo.RevealedLeaf.Script,
		tm.CovenantPrivKeys[1],
		valEncKey,
	)

	require.NoError(t, err)
	_, err = tm.BBNClient.SubmitCovenantSigs(
		tm.CovenantPrivKeys[1].PubKey(),
		stakingMsgTx.TxHash().String(),
		[][]byte{covenantAdaptorStakingSlashing2.MustMarshal()},
		covenantUnbondingSig2,
		[][]byte{covenantAdaptorUnbondingSlashing2.MustMarshal()},
	)
	require.NoError(t, err)
}

func (tm *TestManager) InsertBTCDelegation(t *testing.T, fpPks []*btcec.PublicKey, stakingTime uint16, stakingAmount int64) *TestDelegationData {
	params := tm.StakingParams
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// delegator BTC key pairs, staking tx and slashing tx
	delBtcPrivKey, delBtcPubKey, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	unbondingTime := uint16(tm.StakingParams.MinimumUnbondingTime()) + 1
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingAmount,
		params.SlashingAddress.String(),
		params.SlashingRate,
		unbondingTime,
	)

	// delegator Babylon key pairs
	delBabylonPrivKey, delBabylonPubKey, err := datagen.GenRandomSecp256k1KeyPair(r)
	require.NoError(t, err)

	// proof-of-possession
	pop, err := bstypes.NewPoP(delBabylonPrivKey, delBtcPrivKey)
	require.NoError(t, err)

	// create and insert BTC headers which include the staking tx to get staking tx info
	btcTipHeaderResp, err := tm.BBNClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(btcTipHeaderResp.HeaderHex)
	require.NoError(t, err)
	blockWithStakingTx := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), testStakingInfo.StakingTx)
	accumulatedWork := btclctypes.CalcWork(&blockWithStakingTx.HeaderBytes)
	accumulatedWork = btclctypes.CumulativeWork(accumulatedWork, btcTipHeaderResp.Work)
	parentBlockHeaderInfo := &btclctypes.BTCHeaderInfo{
		Header: &blockWithStakingTx.HeaderBytes,
		Hash:   blockWithStakingTx.HeaderBytes.Hash(),
		Height: btcTipHeaderResp.Height + 1,
		Work:   &accumulatedWork,
	}
	headers := make([]bbntypes.BTCHeaderBytes, 0)
	headers = append(headers, blockWithStakingTx.HeaderBytes)
	for i := 0; i < int(params.ComfirmationTimeBlocks); i++ {
		headerInfo := datagen.GenRandomValidBTCHeaderInfoWithParent(r, *parentBlockHeaderInfo)
		headers = append(headers, *headerInfo.Header)
		parentBlockHeaderInfo = headerInfo
	}
	_, err = tm.BBNClient.InsertBtcBlockHeaders(headers)
	require.NoError(t, err)
	btcHeader := blockWithStakingTx.HeaderBytes
	serializedStakingTx, err := bbntypes.SerializeBTCTx(testStakingInfo.StakingTx)
	require.NoError(t, err)
	txInfo := btcctypes.NewTransactionInfo(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, serializedStakingTx, blockWithStakingTx.SpvProof.MerkleNodes)

	slashignSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		0,
		slashignSpendInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	unbondingValue := stakingAmount - 1000
	stakingTxHash := testStakingInfo.StakingTx.TxHash()

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNetworkParams,
		delBtcPrivKey,
		fpPks,
		params.CovenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stakingTxHash, 0),
		unbondingTime,
		unbondingValue,
		params.SlashingAddress.String(),
		params.SlashingRate,
		unbondingTime,
	)

	unbondingTxMsg := testUnbondingInfo.UnbondingTx

	unbondingSlashingPathInfo, err := testUnbondingInfo.UnbondingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	unbondingSig, err := testUnbondingInfo.SlashingTx.Sign(
		unbondingTxMsg,
		0,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		delBtcPrivKey,
	)
	require.NoError(t, err)

	serializedUnbondingTx, err := bbntypes.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)

	// submit the BTC delegation to Babylon
	_, err = tm.BBNClient.CreateBTCDelegation(
		delBabylonPubKey.(*secp256k1.PubKey),
		bbntypes.NewBIP340PubKeyFromBTCPK(delBtcPubKey),
		fpPks,
		pop,
		uint32(stakingTime),
		stakingAmount,
		txInfo,
		testStakingInfo.SlashingTx,
		delegatorSig,
		serializedUnbondingTx,
		uint32(unbondingTime),
		unbondingValue,
		testUnbondingInfo.SlashingTx,
		unbondingSig)
	require.NoError(t, err)

	t.Log("successfully submitted a BTC delegation")

	return &TestDelegationData{
		DelegatorPrivKey:        delBtcPrivKey,
		DelegatorKey:            delBtcPubKey,
		DelegatorBabylonPrivKey: delBabylonPrivKey.(*secp256k1.PrivKey),
		DelegatorBabylonKey:     delBabylonPubKey.(*secp256k1.PubKey),
		FpPks:                   fpPks,
		StakingTx:               testStakingInfo.StakingTx,
		SlashingTx:              testStakingInfo.SlashingTx,
		StakingTxInfo:           txInfo,
		DelegatorSig:            delegatorSig,
		SlashingAddr:            params.SlashingAddress.String(),
		StakingTime:             stakingTime,
		StakingAmount:           stakingAmount,
	}
}

func defaultFpConfig(keyringDir, homeDir string) *fpcfg.Config {
	cfg := fpcfg.DefaultConfigWithHome(homeDir)

	cfg.BitcoinNetwork = "simnet"
	cfg.BTCNetParams = chaincfg.SimNetParams

	cfg.PollerConfig.AutoChainScanningMode = false
	// babylon configs for sending transactions
	cfg.BabylonConfig.KeyDirectory = keyringDir
	// need to use this one to send otherwise we will have account sequence mismatch
	// errors
	cfg.BabylonConfig.Key = "test-spending-key"
	// Big adjustment to make sure we have enough gas in our transactions
	cfg.BabylonConfig.GasAdjustment = 20

	return &cfg
}

func tempDirWithName(name string) (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, name)
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}

func newDescription(moniker string) *stakingtypes.Description {
	dec := stakingtypes.NewDescription(moniker, "", "", "", "")
	return &dec
}

// ParseRespBTCDelToBTCDel parses an BTC delegation response to BTC Delegation
// adapted from
// https://github.com/babylonchain/babylon/blob/1a3c50da64885452c8d669fcea2a2fad78c8a028/test/e2e/btc_staking_e2e_test.go#L548
func ParseRespBTCDelToBTCDel(resp *bstypes.BTCDelegationResponse) (btcDel *bstypes.BTCDelegation, err error) {
	stakingTx, err := hex.DecodeString(resp.StakingTxHex)
	if err != nil {
		return nil, err
	}

	delSig, err := bbntypes.NewBIP340SignatureFromHex(resp.DelegatorSlashSigHex)
	if err != nil {
		return nil, err
	}

	slashingTx, err := bstypes.NewBTCSlashingTxFromHex(resp.SlashingTxHex)
	if err != nil {
		return nil, err
	}

	btcDel = &bstypes.BTCDelegation{
		// missing BabylonPk, Pop
		// these fields are not sent out to the client on BTCDelegationResponse
		BtcPk:            resp.BtcPk,
		FpBtcPkList:      resp.FpBtcPkList,
		StartHeight:      resp.StartHeight,
		EndHeight:        resp.EndHeight,
		TotalSat:         resp.TotalSat,
		StakingTx:        stakingTx,
		DelegatorSig:     delSig,
		StakingOutputIdx: resp.StakingOutputIdx,
		CovenantSigs:     resp.CovenantSigs,
		UnbondingTime:    resp.UnbondingTime,
		SlashingTx:       slashingTx,
	}

	if resp.UndelegationResponse != nil {
		ud := resp.UndelegationResponse
		unbondTx, err := hex.DecodeString(ud.UnbondingTxHex)
		if err != nil {
			return nil, err
		}

		slashTx, err := bstypes.NewBTCSlashingTxFromHex(ud.SlashingTxHex)
		if err != nil {
			return nil, err
		}

		delSlashingSig, err := bbntypes.NewBIP340SignatureFromHex(ud.DelegatorSlashingSigHex)
		if err != nil {
			return nil, err
		}

		btcDel.BtcUndelegation = &bstypes.BTCUndelegation{
			UnbondingTx:              unbondTx,
			CovenantUnbondingSigList: ud.CovenantUnbondingSigList,
			CovenantSlashingSigs:     ud.CovenantSlashingSigs,
			SlashingTx:               slashTx,
			DelegatorSlashingSig:     delSlashingSig,
		}

		if len(ud.DelegatorUnbondingSigHex) > 0 {
			delUnbondingSig, err := bbntypes.NewBIP340SignatureFromHex(ud.DelegatorUnbondingSigHex)
			if err != nil {
				return nil, err
			}
			btcDel.BtcUndelegation.DelegatorUnbondingSig = delUnbondingSig
		}
	}

	return btcDel, nil
}

func (tm *TestManager) InsertWBTCHeaders(t *testing.T, r *rand.Rand) {
	params, err := tm.BBNClient.QueryStakingParams()
	require.NoError(t, err)
	btcTipResp, err := tm.BBNClient.QueryBtcLightClientTip()
	require.NoError(t, err)
	tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(btcTipResp.HeaderHex)
	require.NoError(t, err)
	kHeaders := datagen.NewBTCHeaderChainFromParentInfo(r, &btclctypes.BTCHeaderInfo{
		Header: &tipHeader,
		Hash:   tipHeader.Hash(),
		Height: btcTipResp.Height,
		Work:   &btcTipResp.Work,
	}, uint32(params.FinalizationTimeoutBlocks))
	_, err = tm.BBNClient.InsertBtcBlockHeaders(kHeaders.ChainToBytes())
	require.NoError(t, err)
}

func (tm *TestManager) FinalizeUntilEpoch(t *testing.T, epoch uint64) {
	bbnClient := tm.BBNClient.GetBBNClient()

	// wait until the checkpoint of this epoch is sealed
	require.Eventually(t, func() bool {
		lastSealedCkpt, err := bbnClient.LatestEpochFromStatus(ckpttypes.Sealed)
		if err != nil {
			return false
		}
		return epoch <= lastSealedCkpt.RawCheckpoint.EpochNum
	}, eventuallyWaitTimeOut, 1*time.Second)

	t.Logf("start finalizing epochs till %d", epoch)
	// Random source for the generation of BTC data
	r := rand.New(rand.NewSource(time.Now().Unix()))

	// get all checkpoints of these epochs
	pagination := &sdkquerytypes.PageRequest{
		Key:   ckpttypes.CkptsObjectKey(1),
		Limit: epoch,
	}
	resp, err := bbnClient.RawCheckpoints(pagination)
	require.NoError(t, err)
	require.Equal(t, int(epoch), len(resp.RawCheckpoints))

	submitter := tm.BBNClient.GetKeyAddress()

	for _, checkpoint := range resp.RawCheckpoints {
		currentBtcTipResp, err := tm.BBNClient.QueryBtcLightClientTip()
		require.NoError(t, err)
		tipHeader, err := bbntypes.NewBTCHeaderBytesFromHex(currentBtcTipResp.HeaderHex)
		require.NoError(t, err)

		rawCheckpoint, err := checkpoint.Ckpt.ToRawCheckpoint()
		require.NoError(t, err)

		btcCheckpoint, err := ckpttypes.FromRawCkptToBTCCkpt(rawCheckpoint, submitter)
		require.NoError(t, err)

		babylonTagBytes, err := hex.DecodeString("01020304")
		require.NoError(t, err)

		p1, p2, err := txformat.EncodeCheckpointData(
			babylonTagBytes,
			txformat.CurrentVersion,
			btcCheckpoint,
		)
		require.NoError(t, err)

		tx1 := datagen.CreatOpReturnTransaction(r, p1)

		opReturn1 := datagen.CreateBlockWithTransaction(r, tipHeader.ToBlockHeader(), tx1)
		tx2 := datagen.CreatOpReturnTransaction(r, p2)
		opReturn2 := datagen.CreateBlockWithTransaction(r, opReturn1.HeaderBytes.ToBlockHeader(), tx2)

		// insert headers and proofs
		_, err = tm.BBNClient.InsertBtcBlockHeaders([]bbntypes.BTCHeaderBytes{
			opReturn1.HeaderBytes,
			opReturn2.HeaderBytes,
		})
		require.NoError(t, err)

		_, err = tm.BBNClient.InsertSpvProofs(submitter.String(), []*btcctypes.BTCSpvProof{
			opReturn1.SpvProof,
			opReturn2.SpvProof,
		})
		require.NoError(t, err)

		// wait until this checkpoint is submitted
		require.Eventually(t, func() bool {
			ckpt, err := bbnClient.RawCheckpoint(checkpoint.Ckpt.EpochNum)
			require.NoError(t, err)
			return ckpt.RawCheckpoint.Status == ckpttypes.Submitted
		}, eventuallyWaitTimeOut, eventuallyPollTime)
	}

	// insert w BTC headers
	tm.InsertWBTCHeaders(t, r)

	// wait until the checkpoint of this epoch is finalised
	require.Eventually(t, func() bool {
		lastFinalizedCkpt, err := bbnClient.LatestEpochFromStatus(ckpttypes.Finalized)
		if err != nil {
			t.Logf("failed to get last finalized epoch: %v", err)
			return false
		}
		return epoch <= lastFinalizedCkpt.RawCheckpoint.EpochNum
	}, eventuallyWaitTimeOut, 1*time.Second)

	t.Logf("epoch %d is finalised", epoch)
}
