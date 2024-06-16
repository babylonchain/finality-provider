package clientcontroller

import (
	"encoding/json"
	"fmt"

	wasmdparams "github.com/CosmWasm/wasmd/app/params"
	appparams "github.com/babylonchain/babylon/app/params"
	bbnclient "github.com/babylonchain/babylon/client/client"
	bbntypes "github.com/babylonchain/babylon/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	cosmwasmclient "github.com/babylonchain/finality-provider/cosmwasmclient/client"
	cwcfg "github.com/babylonchain/finality-provider/cosmwasmclient/config"
	cwtypes "github.com/babylonchain/finality-provider/cosmwasmclient/types"
	fpcfg "github.com/babylonchain/finality-provider/finality-provider/config"
	"github.com/babylonchain/finality-provider/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"go.uber.org/zap"
)

// TODO: rename the file name, class name and etc
// This is not a simple EVM chain. It's a OP Stack L2 chain, which has many
// implications. So we should rename to sth like e.g. OPStackL2Consumer
// This helps distinguish from pure EVM sidechains e.g. Binance Chain
var _ ConsumerController = &EVMConsumerController{}

type EVMConsumerController struct {
	cwClient *cosmwasmclient.Client
	cfg      *fpcfg.EVMConfig
	logger   *zap.Logger
}

func NewEVMConsumerController(
	bbnCfg *fpcfg.BBNConfig,
	evmCfg *fpcfg.EVMConfig,
	logger *zap.Logger,
) (*EVMConsumerController, error) {
	bbnConfig := fpcfg.BBNConfigToBabylonConfig(bbnCfg)
	if err := bbnConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon client: %w", err)
	}
	bbnClient, err := bbnclient.New(
		&bbnConfig,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon client: %w", err)
	}

	keyRec, err := bbnClient.GetKeyring().Key(bbnConfig.Key)
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}
	// submitterAddress retrieves address based on key name which is configured in
	// cfg *stakercfg.BBNConfig.
	submitterAddress, err := keyRec.GetAddress()
	if err != nil {
		panic(fmt.Sprintf("Failed to get key address: %s", err))
	}

	cwConfig := cwcfg.CosmwasmConfig{
		Key:              bbnCfg.Key,
		ChainID:          bbnCfg.ChainID,
		RPCAddr:          bbnCfg.RPCAddr,
		GRPCAddr:         evmCfg.GRPCAddress,
		AccountPrefix:    bbnCfg.AccountPrefix,
		KeyringBackend:   bbnCfg.KeyringBackend,
		GasAdjustment:    bbnCfg.GasAdjustment,
		GasPrices:        bbnCfg.GasPrices,
		KeyDirectory:     bbnCfg.KeyDirectory,
		Debug:            bbnCfg.Debug,
		Timeout:          bbnCfg.Timeout,
		BlockTimeout:     bbnCfg.BlockTimeout,
		OutputFormat:     bbnCfg.OutputFormat,
		SignModeStr:      bbnCfg.SignModeStr,
		SubmitterAddress: submitterAddress.String(),
	}

	if err := cwConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for Babylon Wasm client: %w", err)
	}

	encCfg := appparams.DefaultEncodingConfig()
	cwClient, err := cosmwasmclient.New(
		&cwConfig,
		BabylonConsumerChainName,
		wasmdparams.EncodingConfig{
			InterfaceRegistry: encCfg.InterfaceRegistry,
			Codec:             encCfg.Codec,
			TxConfig:          encCfg.TxConfig,
			Amino:             encCfg.Amino,
		},
		logger,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create Babylon Wasm client: %w", err)
	}

	return &EVMConsumerController{
		cwClient,
		evmCfg,
		logger,
	}, nil
}

// CommitPubRandList commits a list of Schnorr public randomness to Babylon CosmWasm contract
// it returns tx hash and error
func (ec *EVMConsumerController) CommitPubRandList(
	fpPk *btcec.PublicKey,
	startHeight uint64,
	numPubRand uint64,
	commitment []byte,
	sig *schnorr.Signature,
) (*types.TxResponse, error) {
	contractAddress := ec.cfg.OPFinalityGadgetAddress
	contractInfo, err := ec.cwClient.QueryContractInfo(contractAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to query contract info %s: %w", contractAddress, err)
	}

	msg := cwtypes.CommitPublicRandomnessMsg{
		CommitPublicRandomness: cwtypes.CommitPublicRandomnessMsgParams{
			FpPubkeyHex: bbntypes.NewBIP340PubKeyFromBTCPK(fpPk).MarshalHex(),
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig.Serialize(),
		},
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	resp, err := ec.cwClient.ExecuteContract(contractAddress, contractInfo.CodeID, msgData, "")
	if err != nil {
		return nil, err
	}
	var data cwtypes.CommitPublicRandomnessResponse
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, err
	}
	// TODO: need to refactor
	return &types.TxResponse{TxHash: "", Events: nil}, nil
}

// SubmitFinalitySig submits the finality signature
// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (ec *EVMConsumerController) SubmitFinalitySig(
	fpPk *btcec.PublicKey,
	block *types.BlockInfo,
	pubRand *btcec.FieldVal,
	proof []byte, // TODO: have a type for proof
	sig *btcec.ModNScalar,
) (*types.TxResponse, error) {

	return &types.TxResponse{TxHash: "", Events: nil}, nil
}

// SubmitBatchFinalitySigs submits a batch of finality signatures to Babylon
func (ec *EVMConsumerController) SubmitBatchFinalitySigs(
	fpPk *btcec.PublicKey,
	blocks []*types.BlockInfo,
	pubRandList []*btcec.FieldVal,
	proofList [][]byte,
	sigs []*btcec.ModNScalar,
) (*types.TxResponse, error) {
	if len(blocks) != len(sigs) {
		return nil, fmt.Errorf("the number of blocks %v should match the number of finality signatures %v", len(blocks), len(sigs))
	}

	return &types.TxResponse{TxHash: "", Events: nil}, nil
}

// QueryFinalityProviderVotingPower queries the voting power of the finality provider at a given height
func (ec *EVMConsumerController) QueryFinalityProviderVotingPower(fpPk *btcec.PublicKey, blockHeight uint64) (uint64, error) {
	/* TODO: implement

	latest_committed_l2_height = read `latestBlockNumber()` from the L1 L2OutputOracle contract and return the result

	if blockHeight > latest_committed_l2_height:

		query the VP from the L1 oracle contract using "latest" as the block tag

	else:

		1. query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
		  to find the first event where the `_l2BlockNumber` >= blockHeight
		2. get the block.number from the event
		3. query the VP from the L1 oracle contract using `block.number` as the block tag

	*/

	return 0, nil
}

func (ec *EVMConsumerController) QueryLatestFinalizedBlock() (*types.BlockInfo, error) {
	return &types.BlockInfo{
		Height: 0,
		Hash:   nil,
	}, nil
}

func (ec *EVMConsumerController) QueryBlocks(startHeight, endHeight, limit uint64) ([]*types.BlockInfo, error) {

	return ec.queryLatestBlocks(sdk.Uint64ToBigEndian(startHeight), 0, finalitytypes.QueriedBlockStatus_ANY, false)
}

func (ec *EVMConsumerController) queryLatestBlocks(startKey []byte, count uint64, status finalitytypes.QueriedBlockStatus, reverse bool) ([]*types.BlockInfo, error) {
	var blocks []*types.BlockInfo

	return blocks, nil
}

func (ec *EVMConsumerController) QueryBlock(height uint64) (*types.BlockInfo, error) {

	return &types.BlockInfo{
		Height: height,
		Hash:   nil,
	}, nil
}

func (ec *EVMConsumerController) QueryIsBlockFinalized(height uint64) (bool, error) {
	/* TODO: implement
	1. get the latest finalized block number from `latestBlockNumber()` in the L1 L2OutputOracle contract
	2. compare the block number with `height`
	*/
	return false, nil
}

func (ec *EVMConsumerController) QueryActivatedHeight() (uint64, error) {
	/* TODO: implement

		oracle_event = query the event in the L1 oracle contract where the FP's voting power is firstly set

		l1_activated_height = get the L1 block number from the `oracle_event`

	  output_event = query the L1 event `emit OutputProposed(_outputRoot, nextOutputIndex(), _l2BlockNumber, block.timestamp, block.number);`
				to find the first event where the `block.number` >= l1_activated_height

		if output_event == nil:

				read `nextBlockNumber()` from the L1 L2OutputOracle contract and return the result

		else:

				return output_event._l2BlockNumber

	*/

	return 0, nil
}

func (ec *EVMConsumerController) QueryLatestBlockHeight() (uint64, error) {
	/* TODO: implement
	get the latest L2 block number from a RPC call
	*/

	return uint64(0), nil
}

// QueryLastCommittedPublicRand returns the last public randomness commitments
func (ec *EVMConsumerController) QueryLastCommittedPublicRand(fpPk *btcec.PublicKey, count uint64) (map[uint64]*finalitytypes.PubRandCommitResponse, error) {

	return nil, nil
}

func (ec *EVMConsumerController) Close() error {
	ec.cwClient.Stop()
	return nil
}
