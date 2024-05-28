package service

import (
	"context"
	"fmt"
	"os"

	bbnclient "github.com/babylonchain/babylon/client/client"
	bbncfg "github.com/babylonchain/babylon/client/config"
	"github.com/babylonchain/babylon/crypto/eots"
	bbntypes "github.com/babylonchain/babylon/types"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	eotsservice "github.com/babylonchain/eots-aggregator/service"
	eotsclient "github.com/babylonchain/eots-aggregator/service/client"
	"github.com/babylonchain/finality-provider/eotsmanager"
	"github.com/babylonchain/finality-provider/eotsmanager/config"
	"github.com/babylonchain/finality-provider/eotsmanager/proto"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum-optimism/optimism/op-service/dial"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
	"github.com/ethereum/go-ethereum/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// rpcServer is the main RPC server for the EOTS daemon that handles
// gRPC incoming requests.
type rpcServer struct {
	proto.UnimplementedEOTSManagerServer

	em eotsmanager.EOTSManager
}

// newRPCServer creates a new RPC sever from the set of input dependencies.
func newRPCServer(
	em eotsmanager.EOTSManager,
) *rpcServer {

	return &rpcServer{
		em: em,
	}
}

// RegisterWithGrpcServer registers the rpcServer with the passed root gRPC
// server.
func (r *rpcServer) RegisterWithGrpcServer(grpcServer *grpc.Server) error {
	// Register the main RPC server.
	proto.RegisterEOTSManagerServer(grpcServer, r)
	return nil
}

func (r *rpcServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	return &proto.PingResponse{}, nil
}

// CreateKey generates and saves an EOTS key
func (r *rpcServer) CreateKey(ctx context.Context, req *proto.CreateKeyRequest) (
	*proto.CreateKeyResponse, error) {

	pk, err := r.em.CreateKey(req.Name, req.Passphrase, req.HdPath)

	if err != nil {
		return nil, err
	}

	return &proto.CreateKeyResponse{Pk: pk}, nil
}

// CreateMasterRandPair returns a list of Schnorr randomness pairs
func (r *rpcServer) CreateMasterRandPair(ctx context.Context, req *proto.CreateMasterRandPairRequest) (*proto.CreateMasterRandPairResponse, error) {

	mpr, err := r.em.CreateMasterRandPair(req.Uid, req.ChainId, req.Passphrase)
	if err != nil {
		return nil, err
	}

	return &proto.CreateMasterRandPairResponse{
		MasterPubRand: mpr,
	}, nil
}

// KeyRecord returns the key record
func (r *rpcServer) KeyRecord(ctx context.Context, req *proto.KeyRecordRequest) (
	*proto.KeyRecordResponse, error) {

	record, err := r.em.KeyRecord(req.Uid, req.Passphrase)
	if err != nil {
		return nil, err
	}

	res := &proto.KeyRecordResponse{
		Name:       record.Name,
		PrivateKey: record.PrivKey.Serialize(),
	}

	return res, nil
}

// SignEOTS signs an EOTS with the EOTS private key and the relevant randomness
func (r *rpcServer) SignEOTS(ctx context.Context, req *proto.SignEOTSRequest) (
	*proto.SignEOTSResponse, error) {

	sig, err := r.em.SignEOTS(req.Uid, req.ChainId, req.Msg, req.Height, req.Passphrase)
	if err != nil {
		return nil, err
	}

	sigBytes := sig.Bytes()

	return &proto.SignEOTSResponse{Sig: sigBytes[:]}, nil
}

// SignSchnorrSig signs a Schnorr sig with the EOTS private key
func (r *rpcServer) SignSchnorrSig(ctx context.Context, req *proto.SignSchnorrSigRequest) (
	*proto.SignSchnorrSigResponse, error) {

	sig, err := r.em.SignSchnorrSig(req.Uid, req.Msg, req.Passphrase)
	if err != nil {
		return nil, err
	}

	return &proto.SignSchnorrSigResponse{Sig: sig.Serialize()}, nil
}

// verifierRpcServer is the main RPC server for the EOTS daemon that handles
// gRPC incoming requests.
type verifierRpcServer struct {
	proto.UnimplementedEOTSVerifierServer
	cfg           *config.Config
	log           log.Logger
	rollupClient  dial.RollupClientInterface
	bbnClient     *bbnclient.Client
	eotsAggClient *eotsclient.EotsAggregatorGRpcClient
}

// NewVerifierRPCServer creates a new RPC sever for EOTS verifier
func NewVerifierRPCServer(cfg *config.Config, zapLog *zap.Logger) (*verifierRpcServer, error) {
	rollupRPC := cfg.Verifier.RollupRPC
	bbnRPC := cfg.Verifier.BabylonRPC
	eotsAggRPC := cfg.Verifier.EotsAggRPC
	zapLog.Info("Running EOTS verifier", zap.String("RollupRPC", rollupRPC), zap.String("BabylonRPC", bbnRPC), zap.String("EotsAggRPC", eotsAggRPC))

	ctx := context.Background()
	log := oplog.NewLogger(os.Stderr, oplog.DefaultCLIConfig())
	rollupProvider, err := dial.NewStaticL2RollupProvider(ctx, log, rollupRPC)
	if err != nil {
		log.Error("eots-verifier unable to get rollup provider", "err", err)
		return nil, err
	}
	rollupClient, err := rollupProvider.RollupClient(ctx)
	if err != nil {
		log.Error("eots-verifier unable to get rollup client", "err", err)
		return nil, err
	}

	bbnConfig := bbncfg.DefaultBabylonConfig()
	bbnConfig.RPCAddr = bbnRPC
	// bbnClient don't share context
	bbnClient, err := bbnclient.New(&bbnConfig, zapLog)
	if err != nil {
		log.Error("eots-verifier failed to create Babylon client", "err", err)
		return nil, err
	}

	eotsAggClient, err := eotsclient.NewEotsAggregatorGRpcClient(ctx, eotsAggRPC)
	if err != nil {
		log.Error("eots-verifier failed to create EOTS aggregator client", "err", err)
		return nil, err
	}

	return &verifierRpcServer{
		cfg:           cfg,
		log:           log,
		rollupClient:  rollupClient,
		bbnClient:     bbnClient,
		eotsAggClient: eotsAggClient,
	}, nil
}

// RegisterWithGrpcServer registers the rpcServer with the passed root gRPC
// server.
func (rpc *verifierRpcServer) RegisterWithGrpcServer(grpcServer *grpc.Server) error {
	// Register the verifier RPC server.
	proto.RegisterEOTSVerifierServer(grpcServer, rpc)
	return nil
}

// VerifyFinalitySig verifies that the finality signature is valid
func (rpc *verifierRpcServer) VerifyFinalitySig(ctx context.Context, req *proto.VerifyFinalitySigRequest) (
	*proto.VerifyFinalitySigResponse, error) {
	height := req.BlockNumber
	output, err := rpc.rollupClient.OutputAtBlock(ctx, height)
	if err != nil {
		rpc.log.Error("eots-verifier unable to OutputAtBlock", "blockNumber", height, "err", err)
		return nil, err
	}

	blockHash := output.BlockRef.Hash

	rollupCfg, err := rpc.rollupClient.RollupConfig(ctx)
	if err != nil {
		rpc.log.Error("eots-verifier failed to fetch rollup config", "err", err)
		return nil, err
	}
	consumerID := fmt.Sprintf("op-stack-l2-%s", rollupCfg.L2ChainID.String())
	eotsInfos, err := rpc.eotsAggClient.GetEOTSInfos(ctx, int64(height), consumerID)
	if err != nil {
		rpc.log.Error("eots-verifier failed to fetch EOTS info", "err", err)
		return nil, err
	}

	fpList, totalPower, err := rpc.getFinalityProvidersWithPower(ctx, consumerID)
	if err != nil {
		return nil, err
	}

	validPower, err := rpc.calculateValidPower(ctx, blockHash.Bytes(), fpList, eotsInfos)
	if err != nil {
		return nil, err
	}
	// Ensure that the valid power is more than 2/3 of the total power.
	if validPower*3 <= totalPower*2 {
		return &proto.VerifyFinalitySigResponse{
			Result: false,
		}, fmt.Errorf("insufficient voting power of valid finality signatures")
	}

	return &proto.VerifyFinalitySigResponse{
		Result: true,
	}, nil
}

func (rpc *verifierRpcServer) getFinalityProvidersWithPower(ctx context.Context, consumerID string) ([]*bsctypes.FinalityProviderResponse, uint64, error) {
	bbnRes, err := rpc.bbnClient.QueryClient.QueryConsumerFinalityProviders(consumerID, &query.PageRequest{})
	if err != nil {
		rpc.log.Error("eots-verifier failed to query consumer finality provoiders", "err", err)
		return nil, uint64(0), err
	}

	totalPower := uint64(0)
	for _, fp := range bbnRes.FinalityProviders {
		totalPower += fp.VotingPower
	}

	return bbnRes.FinalityProviders, totalPower, nil
}

func (rpc *verifierRpcServer) calculateValidPower(ctx context.Context, msg []byte, fpList []*bsctypes.FinalityProviderResponse, eotsInfos []*eotsservice.EOTSInfoResponse) (uint64, error) {
	validPower := uint64(0)
	for _, fp := range fpList {
		for _, info := range eotsInfos {
			dbFpBtcPk, err := bbntypes.NewBIP340PubKey(info.FpBtcPk[:])
			if err != nil {
				rpc.log.Error("eots-verifier invalid FpBtcPk", "err", err)
				return uint64(0), err
			}
			if dbFpBtcPk.Equals(fp.BtcPk) {
				fpBTCPK, err := fp.BtcPk.ToBTCPK()
				if err != nil {
					return uint64(0), err
				}
				var pubRand *secp256k1.FieldVal
				if overflow := pubRand.SetBytes(&info.PubRand); overflow != 0 {
					rpc.log.Error("eots-verifier invalid PubRand", "err", err)
					return uint64(0), err
				}
				finalitySig, err := bbntypes.NewSchnorrEOTSSig(info.FinalitySig[:])
				if err != nil {
					rpc.log.Error("eots-verifier invalid FinalitySig", "err", err)
					return uint64(0), err
				}
				if err := eots.Verify(fpBTCPK, pubRand, msg, finalitySig.ToModNScalar()); err != nil {
					return uint64(0), err
				}
				validPower += fp.VotingPower
			}
		}
	}
	return validPower, nil
}
