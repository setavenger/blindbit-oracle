// Package v2 is the gRPC endpoint for blindbit Oracle
package v2

import (
	"context"
	"sync"

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
func (s *OracleService) GetInfo(ctx context.Context, _ *emptypb.Empty) (*pb.InfoResponse, error) {
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

// GetTweakArray returns tweaks for a specific block height
func (s *OracleService) GetTweakArray(
	ctx context.Context, req *pb.BlockHeightRequest,
) (*pb.TweakArray, error) {
	blockhash, err := s.db.GetBlockHashByHeight(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).
			Uint32("height", uint32(req.BlockHeight)).
			Msg("failed to get blockhash for by height")
		return nil, status.Errorf(
			codes.Internal, "could not retrieve blockhash for height %d", req.BlockHeight,
		)
	}

	tweakIndex, err := s.db.TweaksForBlockAll(blockhash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not retrieve tweak data")
	}

	tweaks := make([][]byte, len(tweakIndex))
	for i := range tweakIndex {
		tweaks[i] = tweakIndex[i].Tweak[:]
	}

	return &pb.TweakArray{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: req.BlockHeight,
		},
		Tweaks: tweaks,
	}, nil
}

// GetTweakIndexArray returns tweak index data for a specific block height
func (s *OracleService) GetTweakIndexArray(
	ctx context.Context, req *pb.GetTweakIndexRequest,
) (*pb.TweakArray, error) {

	blockhash, err := s.db.GetBlockHashByHeight(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).
			Uint32("height", req.BlockHeight).
			Msg("failed to get blockhash for by height")
		return nil, status.Errorf(
			codes.Internal, "could not retrieve blockhash for height %d", req.BlockHeight,
		)
	}

	_, syncTip, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).
			Msg("failed to get chain tip")
		return nil, err
	}

	tweakIndex, err := s.db.TweaksForBlockCutThrough(blockhash, syncTip)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not retrieve dusted tweak data")
	}

	tweaks := make([][]byte, len(tweakIndex))
	for i := range tweakIndex {
		tweaks[i] = tweakIndex[i].Tweak[:]
	}

	return &pb.TweakArray{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint64(req.BlockHeight),
		},
		Tweaks: tweaks,
	}, nil
}

// GetUTXOArray returns UTXOs for a specific block height
func (s *OracleService) GetUTXOArray(
	ctx context.Context, req *pb.BlockHeightRequest,
) (*pb.UTXOArrayResponse, error) {

	blockhash, err := s.db.GetBlockHashByHeight(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).
			Uint64("height", req.BlockHeight).
			Msg("failed to get blockhash for by height")
		return nil, status.Errorf(
			codes.Internal, "could not retrieve blockhash for height %d", req.BlockHeight,
		)
	}

	// todo: should we move this into the actual call
	// at least make a wrapper which covers this
	_, syncTip, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).
			Msg("failed to get chain tip")
		return nil, err
	}

	// todo: change to dustlimit request
	outputs, err := s.db.FetchOutputsAll(blockhash, syncTip)
	if err != nil {
		logging.L.Err(err).Msg("failed pulling utxos")
		return nil, status.Errorf(
			codes.Internal, "failed to pull utxos at height %d", req.BlockHeight,
		)
	}

	// Convert internal UTXO types to protobuf types
	pbUtxos := make([]*pb.UTXO, len(outputs))
	for i, utxo := range outputs {
		// todo: g in and change scriptpubKey to at least Byte slice if not even array
		pbUtxos[i] = &pb.UTXO{
			Txid:         utils.ReverseBytesCopy(utxo.Txid[:]),
			Vout:         utxo.Vout,
			Value:        utxo.Amount,
			ScriptPubKey: utxo.Pubkey,
			BlockHeight:  req.BlockHeight,
			BlockHash:    blockhash,
		}
	}

	return &pb.UTXOArrayResponse{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   blockhash,
			BlockHeight: req.BlockHeight,
		},
		Utxos: pbUtxos,
	}, nil
}

// GetFilter returns filter data for a specific block height and type
// func (s *OracleService) GetFilter(
// 	ctx context.Context, req *pb.GetFilterRequest,
// ) (*pb.FilterRepsonse, error) {
// 	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
// 	if err != nil {
// 		logging.L.Err(err).Msg("error fetching block header inv")
// 		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
// 	}
//
// 	var filter types.Filter
// 	var err2 error
//
// 	switch req.FilterType {
// 	case pb.FilterType_FILTER_TYPE_SPENT:
// 		filter, err2 = dblevel.FetchByBlockHashSpentOutpointsFilter(headerInv.Hash)
// 	case pb.FilterType_FILTER_TYPE_NEW_UTXOS:
// 		filter, err2 = dblevel.FetchByBlockHashNewUTXOsFilter(headerInv.Hash)
// 	default:
// 		return nil, status.Errorf(codes.InvalidArgument, "invalid filter type")
// 	}
//
// 	if err2 != nil {
// 		logging.L.Err(err2).Msg("error fetching filter")
// 		return nil, status.Errorf(codes.Internal, "could not retrieve filter data")
// 	}
//
// 	return &pb.FilterRepsonse{
// 		BlockIdentifier: &pb.BlockIdentifier{
// 			BlockHash:   headerInv.Hash[:],
// 			BlockHeight: uint64(headerInv.Height),
// 		},
// 		FilterData: &pb.FilterData{
// 			FilterType: req.FilterType,
// 			Data:       filter.Data,
// 		},
// 	}, nil
// }

// GetSpentOutpointsIndex returns spent outpoints index for a specific block height
// func (s *OracleService) GetSpentOutpointsIndex(
// 	ctx context.Context, req *pb.BlockHeightRequest,
// ) (*pb.SpentOutpointsIndexResponse, error) {
// 	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
// 	if err != nil {
// 		logging.L.Err(err).Msg("error fetching block header inv")
// 		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
// 	}
//
// 	spentOutpoints, err := dblevel.FetchByBlockHashSpentOutpointIndex(headerInv.Hash)
// 	if err != nil {
// 		logging.L.Err(err).Msg("error fetching spent outpoints index")
// 		return nil, status.Errorf(codes.Internal, "could not retrieve spent outpoints data")
// 	}
//
// 	spentOutpointsSliced := make([][]byte, len(spentOutpoints.Data))
// 	for i := range spentOutpointsSliced {
// 		spentOutpointsSliced[i] = spentOutpoints.Data[i][:]
// 	}
//
// 	return &pb.SpentOutpointsIndexResponse{
// 		BlockIdentifier: &pb.BlockIdentifier{
// 			BlockHash:   headerInv.Hash[:],
// 			BlockHeight: uint64(headerInv.Height),
// 		},
// 		Data: spentOutpointsSliced,
// 	}, nil
// }

// StreamBlockBatchSlim streams lightweight block batches for efficient processing
func (s *OracleService) StreamBlockBatchSlim(
	req *pb.RangedBlockHeightRequest,
	stream pb.OracleService_StreamBlockBatchSlimServer,
) error {
	logging.L.Info().Msgf("streaming slim batches from %d to %d", req.Start, req.End)

	for height := req.Start; height <= req.End; height++ {
		blockhash, err := s.db.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to blockash by height")
			return err
		}

		// Fetch all data for this block
		// todo: make dependant on which index is supported
		// for now it's always full basic

		tweakIndex, err := s.db.TweaksForBlockAll(blockhash)
		if err != nil {
			logging.L.Err(err).
				Uint64("height", height).
				Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
				Msg("failed to pull tweaks")
			return status.Errorf(codes.Internal, "failed to pull tweak index for height %d", height)
		}

		// Convert tweaks to bytes
		tweakBytes := make([][]byte, len(tweakIndex))
		for i, tweak := range tweakIndex {
			tweakBytes[i] = tweak.Tweak[:]
		}

		batch := &pb.BlockBatchSlim{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   utils.ReverseBytesCopy(blockhash),
				BlockHeight: height,
			},
			Tweaks:           tweakBytes,
			NewUtxosFilter:   nil,
			SpentUtxosFilter: nil,
		}

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(codes.Internal, "failed to send block batch for height %d", height)
		}
	}

	return nil
}

// StreamBlockBatchFull streams complete block batches with all data
func (s *OracleService) StreamBlockBatchFull(
	req *pb.RangedBlockHeightRequest, stream pb.OracleService_StreamBlockBatchFullServer,
) error {
	// todo: should we move this into the actual call
	// at least make a wrapper which covers this
	_, syncTip, err := s.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).
			Msg("failed to get chain tip")
		return err
	}

	for height := req.Start; height <= req.End; height++ {
		select {
		case <-stream.Context().Done():
			logging.L.Debug().Msg("stream context cancelled")
			return nil
		default:
		}

		blockhash, err := s.db.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Warn().
				Err(err).
				Uint64("height", height).
				Msg("failed to get blockash by height")
			return err
		}

		// Fetch all data for this block
		// todo: make dependant on which index is supported
		// for now it's always full basic

		var wg sync.WaitGroup
		var pbUtxos []*pb.UTXO

		wg.Add(1)
		go func() {
			defer wg.Done()
			// utxos
			// todo: change to dustlimit request
			var outputs []*database.Output
			outputs, err = s.db.FetchOutputsAll(blockhash, syncTip)
			if err != nil {
				logging.L.Err(err).Msg("failed pulling utxos")
				// return status.Errorf(codes.Internal, "failed to pull utxos at height %d", height)
			}

			// Convert internal UTXO types to protobuf types
			pbUtxos = make([]*pb.UTXO, len(outputs))
			for i, utxo := range outputs {
				// todo: g in and change scriptpubKey to at least Byte slice if not even array
				pbUtxos[i] = &pb.UTXO{
					Txid:         utils.ReverseBytesCopy(utxo.Txid[:]),
					Vout:         utxo.Vout,
					Value:        utxo.Amount,
					ScriptPubKey: utxo.Pubkey,
					// BlockHeight:  height,
					// BlockHash:    blockhash,
				}
			}
		}()

		var tweakBytes [][]byte

		wg.Add(1)
		go func() {
			defer wg.Done()
			// tweaks
			tweakIndex, err := s.db.TweaksForBlockAll(blockhash)
			if err != nil {
				logging.L.Err(err).
					Uint64("height", height).
					Hex("blockhash", utils.ReverseBytesCopy(blockhash)).
					Msg("failed to pull tweaks")
				// return status.Errorf(codes.Internal, "failed to pull tweak index for height %d", height)
			}

			// Convert tweaks to bytes
			tweakBytes = make([][]byte, len(tweakIndex))
			for i, tweak := range tweakIndex {
				tweakBytes[i] = tweak.Tweak[:]
			}
		}()

		wg.Wait()

		batch := &pb.BlockBatchFull{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   utils.ReverseBytesCopy(blockhash),
				BlockHeight: height,
			},
			Tweaks:           tweakBytes,
			Utxos:            pbUtxos,
			NewUtxosFilter:   nil,
			SpentUtxosFilter: nil,
			SpentUtxos:       make([][]byte, 0),
		}

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(codes.Internal, "failed to send block batch for height %d", height)
		}
	}

	return nil
}

func (s *OracleService) StreamBlockBatchSlimStatic(
	req *pb.RangedBlockHeightRequest, stream pb.OracleService_StreamBlockBatchSlimStaticServer,
) error {
	for height := req.Start; height <= req.End; height++ {
		blockhash, err := s.db.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to blockash by height")
			return err
		}

		tweakIndex, err := s.db.FetchTweaksStatic(blockhash)
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to pull tweaks")
			return err
		}

		batch := &pb.BlockBatchSlim{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   utils.ReverseBytesCopy(blockhash),
				BlockHeight: height,
			},
			Tweaks:           tweakIndex,
			NewUtxosFilter:   nil,
			SpentUtxosFilter: nil,
		}

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(codes.Internal, "failed to send block batch for height %d", height)
		}
	}

	return nil
}

func (s *OracleService) StreamBlockBatchFullStatic(
	req *pb.RangedBlockHeightRequest, stream pb.OracleService_StreamBlockBatchFullStaticServer,
) error {
	for height := req.Start; height <= req.End; height++ {
		blockhash, err := s.db.GetBlockHashByHeight(uint32(height))
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to blockash by height")
			return err
		}

		var wg sync.WaitGroup
		var pbUtxos []*pb.UTXO
		var tweakIndex [][]byte
		errChan := make(chan error, 2)

		wg.Add(1)
		go func() {
			defer wg.Done()
			tweakIndex, err = s.db.FetchTweaksStatic(blockhash)
			if err != nil {
				logging.L.Err(err).Uint64("height", height).Msg("failed to pull tweaks")
				errChan <- err
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			outputs, err := s.db.FetchOutputsStatic(blockhash)
			if err != nil {
				logging.L.Err(err).Uint64("height", height).Msg("failed to pull outputs")
				errChan <- err
			}

			pbUtxos = make([]*pb.UTXO, len(outputs))
			for i, utxo := range outputs {
				pbUtxos[i] = &pb.UTXO{
					Txid:         utils.ReverseBytesCopy(utxo.Txid),
					Vout:         utxo.Vout,
					Value:        utxo.Amount,
					ScriptPubKey: utxo.Pubkey,
				}
			}
		}()

		wg.Wait()

		select {
		case err := <-errChan:
			logging.L.Err(err).Msg("ended with err")
			return err
		default:
			// No errors
		}
		batch := &pb.BlockBatchFull{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   utils.ReverseBytesCopy(blockhash),
				BlockHeight: height,
			},
			Tweaks:           tweakIndex,
			Utxos:            pbUtxos,
			NewUtxosFilter:   nil,
			SpentUtxosFilter: nil,
			SpentUtxos:       make([][]byte, 0),
		}

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(codes.Internal, "failed to send block batch for height %d", height)
		}
	}

	return nil
}
