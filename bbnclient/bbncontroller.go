package babylonclient

import (
	"context"
	"fmt"
	"strconv"
	"time"

	bbnapp "github.com/babylonchain/babylon/app"
	"github.com/babylonchain/babylon/types"
	btcstakingtypes "github.com/babylonchain/babylon/x/btcstaking/types"
	finalitytypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/babylonchain/btc-validator/valcfg"
	"github.com/babylonchain/rpc-client/client"
	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/sirupsen/logrus"
	lensquery "github.com/strangelove-ventures/lens/client/query"
	"google.golang.org/grpc/metadata"
)

var _ BabylonClient = &BabylonController{}

func (bc *BabylonController) GetTxSigner() string {
	signer := bc.rpcClient.MustGetAddr()
	prefix := bc.rpcClient.GetConfig().AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

type BabylonController struct {
	rpcClient *client.Client
	logger    *logrus.Logger
	timeout   time.Duration
}

func NewBabylonController(
	cfg *valcfg.BBNConfig,
	logger *logrus.Logger,
) (*BabylonController, error) {
	babylonConfig := valcfg.BBNConfigToBabylonConfig(cfg)

	// TODO should be validated earlier
	if err := babylonConfig.Validate(); err != nil {
		return nil, err
	}
	// create a Tendermint/Cosmos client for Babylon
	rpcClient, err := client.New(&babylonConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create Babylon rpc client: %w", err)
	}

	// HACK: replace the modules in public rpc-client to add BTC staking / finality modules
	// so that it recognises their message formats
	// TODO: fix this either by fixing rpc-client side
	var moduleBasics []module.AppModuleBasic
	for _, mbasic := range bbnapp.ModuleBasics {
		moduleBasics = append(moduleBasics, mbasic)
	}
	rpcClient.Config.Modules = moduleBasics

	return &BabylonController{
		rpcClient,
		logger,
		cfg.Timeout,
	}, nil
}

func (bc *BabylonController) GetTxSigner() string {
	signer := bc.rpcClient.MustGetAddr()
	prefix := bc.rpcClient.GetConfig().AccountPrefix
	return sdk.MustBech32ifyAddressBytes(prefix, signer)
}

// RegisterValidator registers a BTC validator via a MsgCreateBTCValidator to Babylon
// it returns tx hash and error
func (bc *BabylonController) RegisterValidator(bbnPubKey *secp256k1.PubKey, btcPubKey *types.BIP340PubKey, pop *btcstakingtypes.ProofOfPossession) ([]byte, error) {
	registerMsg := &btcstakingtypes.MsgCreateBTCValidator{
		Signer:    bc.GetTxSigner(),
		BabylonPk: bbnPubKey,
		BtcPk:     btcPubKey,
		Pop:       pop,
	}

	res, err := bc.rpcClient.SendMsg(context.Background(), registerMsg, "")
	if err != nil {
		return nil, err
	}

	return []byte(res.TxHash), nil
}

// CommitPubRandList commits a list of Schnorr public randomness via a MsgCommitPubRand to Babylon
// it returns tx hash and error
func (bc *BabylonController) CommitPubRandList(btcPubKey *types.BIP340PubKey, startHeight uint64, pubRandList []types.SchnorrPubRand, sig *types.BIP340Signature) ([]byte, error) {
	msg := &finalitytypes.MsgCommitPubRandList{
		Signer:      bc.GetTxSigner(),
		ValBtcPk:    btcPubKey,
		StartHeight: startHeight,
		PubRandList: pubRandList,
		Sig:         sig,
	}

	res, err := bc.rpcClient.SendMsg(context.Background(), msg, "")
	if err != nil {
		return nil, err
	}

	return []byte(res.TxHash), nil
}

// SubmitJurySig submits the Jury signature via a MsgAddJurySig to Babylon if the daemon runs in Jury mode
// it returns tx hash and error
func (bc *BabylonController) SubmitJurySig(btcPubKey *types.BIP340PubKey, delPubKey *types.BIP340PubKey, sig *types.BIP340Signature) ([]byte, error) {
	panic("implement me")
}

// SubmitFinalitySig submits the finality signature via a MsgAddVote to Babylon
func (bc *BabylonController) SubmitFinalitySig(btcPubKey *types.BIP340PubKey, blockHeight uint64, blockHash []byte, sig *types.SchnorrEOTSSig) ([]byte, error) {
	panic("implement me")
}

// Note: the following queries are only for PoC
// QueryHeightWithLastPubRand queries the height of the last block with public randomness
func (bc *BabylonController) QueryHeightWithLastPubRand(btcPubKey *types.BIP340PubKey) (uint64, error) {
	panic("implement me")
}

// QueryShouldSubmitJurySigs queries if there's a list of delegations that the Jury should submit Jury sigs to
// it is only used when the program is running in Jury mode
// it returns a list of public keys used for delegations
func (bc *BabylonController) QueryShouldSubmitJurySigs(btcPubKey *types.BIP340PubKey) (bool, []*types.BIP340PubKey, error) {
	panic("implement me")
}

// QueryShouldValidatorVote asks Babylon if the validator should submit a finality sig for the given block height
func (bc *BabylonController) QueryShouldValidatorVote(btcPubKey *types.BIP340PubKey, blockHeight uint64) (bool, error) {
	panic("implement me")
}

func (bc *BabylonController) QueryNodeStatus() (*ctypes.ResultStatus, error) {
	status, err := bc.rpcClient.QueryClient.GetStatus()
	if err != nil {
		return nil, err
	}

	return status, nil
}

func getQueryContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	defaultOptions := lensquery.DefaultOptions()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	strHeight := strconv.Itoa(int(defaultOptions.Height))
	ctx = metadata.AppendToOutgoingContext(ctx, grpctypes.GRPCBlockHeightHeader, strHeight)
	return ctx, cancel
}

func (bc *BabylonController) QueryHeader(height int64) (*ctypes.ResultHeader, error) {
	ctx, cancel := getQueryContext(bc.timeout)
	headerResp, err := bc.rpcClient.ChainClient.RPCClient.Header(ctx, &height)
	defer cancel()

	if err != nil {
		return nil, err
	}

	// Returning response directly, if header with specified number did not exist
	// at request will contain nill header
	return headerResp, nil
}
