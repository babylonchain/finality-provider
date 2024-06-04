package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sync/atomic"

	bbnclient "github.com/babylonchain/babylon/client/client"
	bbncfg "github.com/babylonchain/babylon/client/config"
	"github.com/babylonchain/babylon/crypto/eots"
	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
	eotsservice "github.com/babylonchain/eots-aggregator/service"
	eotsclient "github.com/babylonchain/eots-aggregator/service/client"
	"github.com/babylonchain/finality-provider/eots-verifier/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lightningnetwork/lnd/signal"

	"github.com/babylonchain/finality-provider/eots-verifier/proto"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server is the main RPC server for the EOTS verifier daemon.
// It handles gRPC incoming requests.
type Server struct {
	started  int32
	shutdown signal.Interceptor
	quit     chan struct{}

	proto.UnimplementedEOTSVerifierServer

	cfg    *config.Config
	logger *zap.Logger

	// Need to ensure that the consumer is an Ethereum client
	consumerClient *ethclient.Client
	bbnClient      *bbnclient.Client
	eotsAggClient  *eotsclient.EotsAggregatorGRpcClient
}

// NewServer creates a new RPC sever for EOTS verifier
func NewServer(ctx context.Context, logger *zap.Logger, cfg *config.Config, shutdown signal.Interceptor) (*Server, error) {
	logger.Info("New EOTS verifier server")

	consumerRPC := cfg.ConsumerRPC
	bbnRPC := cfg.BabylonRPC
	eotsAggRPC := cfg.EotsAggRPC

	consumerClient, err := ethclient.DialContext(ctx, consumerRPC)
	if err != nil {
		logger.Error("Failed to dial consumer RPC", zap.Error(err))
		return nil, err
	}
	chainID, err := consumerClient.ChainID(ctx)
	if err != nil {
		logger.Error("Failed to fetch consumer ChainID", zap.Error(err))
		return nil, err
	}
	chainIDStr := chainID.Text(10)
	logger.Info("Consumer", zap.String("ChainID", chainIDStr))
	cfg.ChainID = chainIDStr

	bbnConfig := bbncfg.DefaultBabylonConfig()
	bbnConfig.RPCAddr = bbnRPC
	// bbnClient don't share context
	bbnClient, err := bbnclient.New(&bbnConfig, logger)
	if err != nil {
		logger.Error("Failed to dial Babylon RPC", zap.Error(err))
		return nil, err
	}
	// This is only for status checking
	bbnStatus, err := bbnClient.RPCClient.Status(ctx)
	if err != nil {
		logger.Error("Failed to fetch Babylonchain status", zap.Error(err))
		return nil, err
	}
	logger.Info("Babylonchain", zap.Int64("LatestBlockHeight", bbnStatus.SyncInfo.LatestBlockHeight))

	eotsAggClient, err := eotsclient.NewEotsAggregatorGRpcClient(ctx, eotsAggRPC)
	if err != nil {
		logger.Error("Failed to dial EOTS aggregator RPC", zap.Error(err))
		return nil, err
	}

	return &Server{
		shutdown: shutdown,
		quit:     make(chan struct{}, 1),

		cfg:    cfg,
		logger: logger,

		consumerClient: consumerClient,
		bbnClient:      bbnClient,
		eotsAggClient:  eotsAggClient,
	}, nil
}

// Run runs the main EOTS verifier server loop until a signal is
// received to shut down the process.
func (s *Server) Run() error {
	if atomic.AddInt32(&s.started, 1) != 1 {
		return nil
	}

	defer func() {
		if s.consumerClient != nil {
			s.consumerClient.Close()
		}
		if s.eotsAggClient != nil {
			s.eotsAggClient.Close()
		}
		s.logger.Info("Shutdown complete")
	}()

	listenAddr := s.cfg.RpcListener
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	defer grpcServer.GracefulStop()

	// Register the verifier RPC server.
	proto.RegisterEOTSVerifierServer(grpcServer, s)

	go func() {
		s.logger.Info("gRPC server listening", zap.String("address", listener.Addr().String()))
		if err := grpcServer.Serve(listener); err != nil {
			s.logger.Error("failed to serve gRPC server", zap.Error(err))
		}
	}()

	s.logger.Info("EOTS Verifier Daemon is fully active!")

	// Block until a shutdown signal is received.
	<-s.shutdown.ShutdownChannel()

	return nil
}

// VerifyFinalitySig verifies that the finality signature is valid
func (s *Server) VerifyFinalitySig(ctx context.Context, req *proto.VerifyFinalitySigRequest) (
	*proto.VerifyFinalitySigResponse, error) {
	txHash := common.HexToHash(req.TxHash)
	txReceipt, err := s.consumerClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}
	l2BlockNumber := txReceipt.BlockNumber
	l2Block, err := s.consumerClient.BlockByNumber(ctx, l2BlockNumber)
	if err != nil {
		return nil, err
	}

	parentHash := l2Block.ParentHash()
	transactions := l2Block.Transactions()
	var txList []byte
	for _, tx := range transactions {
		txBytes, err := tx.MarshalBinary()
		if err != nil {
			s.logger.Error("failed to encode the transaction.")
			continue
		}
		txList = append(txList, txBytes...)
	}
	// sign (parent_hash ++ transaction_list)
	sigMsg := append(parentHash.Bytes(), txList...)

	consumerID := fmt.Sprintf("op-stack-l2-%s", s.cfg.ChainID)
	eotsInfos, err := s.eotsAggClient.GetEOTSInfos(ctx, l2BlockNumber.Uint64(), consumerID)
	if err != nil {
		return nil, err
	}
	eotsInfoMap := make(map[string]*eotsservice.EOTSInfoResponse)
	for _, info := range eotsInfos {
		// Convert the [32]byte key to a hex string for the map
		key := hex.EncodeToString(info.FpBtcPk[:])
		eotsInfoMap[key] = info
	}

	fpList, totalPower, err := s.getFinalityProvidersWithPower(ctx, consumerID)
	if err != nil {
		return nil, err
	}

	validPower, err := s.calculateTotalPowerFromEotsVerifiedFps(ctx, sigMsg, fpList, eotsInfoMap)
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

func (s *Server) getFinalityProvidersWithPower(ctx context.Context, consumerID string) ([]*bsctypes.FinalityProviderResponse, uint64, error) {
	bbnRes, err := s.bbnClient.QueryClient.QueryConsumerFinalityProviders(consumerID, &query.PageRequest{})
	if err != nil {
		return nil, uint64(0), err
	}

	totalPower := uint64(0)
	for _, fp := range bbnRes.FinalityProviders {
		totalPower += fp.VotingPower
	}

	return bbnRes.FinalityProviders, totalPower, nil
}

func (s *Server) calculateTotalPowerFromEotsVerifiedFps(
	ctx context.Context,
	msg []byte,
	fpList []*bsctypes.FinalityProviderResponse,
	eotsInfoMap map[string]*eotsservice.EOTSInfoResponse,
) (uint64, error) {
	validPower := uint64(0)
	for _, fp := range fpList {
		fpBtcPkStr := fp.BtcPk.MarshalHex()
		if info, exists := eotsInfoMap[fpBtcPkStr]; exists {
			fpBTCPK, err := fp.BtcPk.ToBTCPK()
			if err != nil {
				return uint64(0), err
			}
			var pubRand *secp256k1.FieldVal
			// TODO: fetch the public randomness from the verifier contract
			// if pOverflow := pubRand.SetBytes(); pOverflow != 0 {
			// 	return uint64(0), err
			// }

			// finalitySig is the `s` part of the Schnorr signature.
			var finalitySig *btcec.ModNScalar
			if sOverflow := finalitySig.SetBytes(&info.FinalitySig); sOverflow != 0 {
				return uint64(0), err
			}

			if err := eots.Verify(fpBTCPK, pubRand, msg, finalitySig); err != nil {
				return uint64(0), err
			}
			validPower += fp.VotingPower
		}
	}
	return validPower, nil
}
