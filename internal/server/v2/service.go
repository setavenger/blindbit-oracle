// Package v2 is the gRPC endpoint for blindbit Oracle
package v2

import (
	"context"
	"encoding/hex"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// OracleService implements the gRPC OracleService interface
type OracleService struct {
	db database.DB
	pb.UnimplementedOracleServiceServer
}

// NewOracleService creates a new OracleService instance
func NewOracleService(db database.DB) *OracleService {
	return &OracleService{
		db: db,
	}
}

// GetInfo returns oracle information
func (s *OracleService) GetInfo(
	ctx context.Context, _ *emptypb.Empty,
) (
	*pb.InfoResponse, error,
) {
	blockhash, height, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("failed pulling chain tip")
		return nil, err
	}

	_ = blockhash //todo: also add current blockhash to info

	return &pb.InfoResponse{
		Network:                        config.ChainToString(config.Chain),
		Height:                         uint64(height),
		TweaksOnly:                     config.TweaksOnly,
		TweaksFullBasic:                config.TweakIndexFullNoDust,
		TweaksFullWithDustFilter:       config.TweakIndexFullIncludingDust,
		TweaksCutThroughWithDustFilter: config.TweaksCutThroughWithDust,
	}, nil
}

// GetBestBlockHeight returns the current best block height
func (s *OracleService) GetBestBlockHeight(
	ctx context.Context, _ *emptypb.Empty,
) (*pb.BlockHeightResponse, error) {
	_, height, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("failed pulling chain tip")
		return nil, err
	}

	return &pb.BlockHeightResponse{
		BlockHeight: uint64(height),
	}, nil
}

// GetBlockHashByHeight returns the block hash for a given height
func (s *OracleService) GetBlockHashByHeight(
	ctx context.Context, req *pb.BlockHeightRequest,
) (*pb.BlockHashResponse, error) {
	blockhash, err := s.db.GetBlockHashByHeight(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).
			Uint64("height", req.BlockHeight).
			Msg("failed pulling blockhash for height")
		return nil, err
	}

	return &pb.BlockHashResponse{
		BlockHash: utils.ReverseBytesCopy(blockhash),
	}, nil
}

func (s *OracleService) StreamComputeIndexServer(
	req *pb.RangedBlockHeightRequestFiltered,
	stream pb.OracleService_StreamComputeIndexServer,
) error {
	for height := req.Start; height <= req.End; height++ {
		blockhash, err := s.db.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Err(err).
				Uint64("height", height).
				Msg("failed to blockash by height")
			return err
		}

		shortOuts, err := s.db.FetchComputeIndex(uint32(height))
		if err != nil {
			logging.L.Err(err).
				Uint64("height", height).
				Msg("failed to pull short outs")
			return err
		}

		// for i := range shortOuts {
		// 	for j := range shortOuts[i].OutputsShort {
		// 		shortOuts[i].OutputsShort = shortOuts[i].OutputsShort[:4]
		// 	}
		// }

		batch := &pb.ComputeIndexResponse{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   utils.ReverseBytesCopy(blockhash),
				BlockHeight: height,
			},
			Index: shortOuts,
		}

		fmt.Printf("return height: %d - %d\n", height, len(shortOuts))

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(
				codes.Internal,
				"failed to send block batch for height %d", height,
			)
		}
	}

	return nil
}

// GetFullBlock returns complete block data with all transaction details
func (s *OracleService) GetFullBlock(
	ctx context.Context, req *pb.BlockHeightRequest,
) (*pb.FullBlockResponse, error) {
	blockhash, err := s.db.GetBlockHashByHeight(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).
			Uint64("height", req.BlockHeight).
			Msg("could not fetch block hash")
		return nil, status.Errorf(codes.Internal, "could not fetch block hash: %v", err)
	}

	// Get chain tip for FetchOutputsAll
	_, syncTip, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("could not fetch chain tip")
		return nil, status.Errorf(codes.Internal, "could not fetch chain tip: %v", err)
	}

	// Fetch all the data we need for the full block
	outputs, err := s.db.FetchOutputsAll(blockhash, syncTip)
	if err != nil {
		logging.L.Err(err).Msg("error fetching outputs")
		return nil, status.Errorf(codes.Internal, "could not retrieve outputs from database: %v", err)
	}

	// Fetch tweaks with transaction IDs
	tweakRows, err := s.db.TweaksForBlockAll(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching tweak rows")
		return nil, status.Errorf(codes.Internal, "could not retrieve tweaks from database: %v", err)
	}

	// Group outputs by transaction ID
	txOutputs := make(map[string][]*database.Output)
	for _, output := range outputs {
		if output != nil {
			txid := hex.EncodeToString(output.Txid)
			txOutputs[txid] = append(txOutputs[txid], output)
		}
	}

	// Create a map of txid to tweak for quick lookup
	txidToTweak := make(map[string][33]byte)
	for _, tweakRow := range tweakRows {
		if tweakRow != nil {
			txid := hex.EncodeToString(tweakRow.Txid[:])
			txidToTweak[txid] = tweakRow.Tweak
		}
	}

	// Fetch all txid-outpoints mappings for this block
	txidOutpointsMap, err := s.db.FetchAllTxidOutpointsForBlock(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching txid-outpoints mappings")
		return nil, status.Errorf(codes.Internal, "could not retrieve txid-outpoints from database: %v", err)
	}

	// Build the full transaction items
	var fullTxItems []*pb.FullTxItem
	for txid, outputs := range txOutputs {
		txidBytes, err := hex.DecodeString(txid)
		if err != nil {
			logging.L.Err(err).Msg("error decoding txid")
			continue
		}

		var txidArray [32]byte
		copy(txidArray[:], txidBytes)

		// Get tweak for this transaction
		var tweak [33]byte
		if tweakBytes, exists := txidToTweak[txid]; exists {
			tweak = tweakBytes
		}

		// Get inputs (spent outpoints) for this transaction
		var inputs []byte
		if outpoints, exists := txidOutpointsMap[txidArray]; exists {
			for i := range outpoints {
				utils.ReverseBytes(outpoints[i][:32])
				inputs = append(inputs, outpoints[i][:]...)
			}
		}

		// Convert outputs to UTXO items
		var utxoItems []*pb.UTXOItemLight
		for _, output := range outputs {
			var pubkey [32]byte
			copy(pubkey[:], output.Pubkey)

			utxoItems = append(utxoItems, &pb.UTXOItemLight{
				Vout:   output.Vout,
				Amount: output.Amount,
				Pubkey: pubkey[:],
			})
		}

		fullTxItems = append(fullTxItems, &pb.FullTxItem{
			Txid:   utils.ReverseBytesCopy(txidArray[:]),
			Tweak:  tweak[:],
			Inputs: inputs,
			Utxos:  utxoItems,
		})
	}

	response := &pb.FullBlockResponse{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: req.BlockHeight,
		},
		Index: fullTxItems,
	}

	return response, nil
}
