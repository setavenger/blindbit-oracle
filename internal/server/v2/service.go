package v2

import (
	"context"
	"encoding/hex"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

// OracleService implements the gRPC OracleService interface
type OracleService struct {
	pb.UnimplementedOracleServiceServer
}

// NewOracleService creates a new OracleService instance
func NewOracleService() *OracleService {
	return &OracleService{}
}

// GetInfo returns oracle information
func (s *OracleService) GetInfo(ctx context.Context, _ *emptypb.Empty) (*pb.InfoResponse, error) {
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
	if err != nil {
		logging.L.Err(err).Msg("error fetching highest block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve data from database")
	}

	return &pb.InfoResponse{
		Network:                        config.ChainToString(config.Chain),
		Height:                         uint64(lastHeader.Height),
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
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
	if err != nil {
		logging.L.Err(err).Msg("error fetching highest block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve data from database")
	}

	return &pb.BlockHeightResponse{
		BlockHeight: uint64(lastHeader.Height),
	}, nil
}

// GetBlockHashByHeight returns the block hash for a given height
func (s *OracleService) GetBlockHashByHeight(
	ctx context.Context, req *pb.BlockHeightRequest,
) (*pb.BlockHashResponse, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	return &pb.BlockHashResponse{
		BlockHash: headerInv.Hash[:],
	}, nil
}

// GetTweakArray returns tweaks for a specific block height
func (s *OracleService) GetTweakArray(ctx context.Context, req *pb.BlockHeightRequest) (*pb.TweakArray, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	tweaks, err := dblevel.FetchByBlockHashTweaks(headerInv.Hash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching tweaks")
		return nil, status.Errorf(codes.Internal, "could not retrieve tweak data")
	}

	// Convert tweaks to bytes
	tweakBytes := make([][]byte, len(tweaks))
	for i, tweak := range tweaks {
		tweakBytes[i] = tweak.TweakData[:]
	}

	return &pb.TweakArray{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   headerInv.Hash[:],
			BlockHeight: uint64(headerInv.Height),
		},
		Tweaks: tweakBytes,
	}, nil
}

// GetTweakIndexArray returns tweak index data for a specific block height
func (s *OracleService) GetTweakIndexArray(
	ctx context.Context, req *pb.GetTweakIndexRequest,
) (*pb.TweakArray, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	var tweaks [][33]byte

	if req.DustLimit > 0 {
		// todo: think about adding not supported error
		var rawTweaks []types.Tweak
		rawTweaks, err = dblevel.FetchByBlockHashDustLimitTweaks(headerInv.Hash, req.DustLimit)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "could not retrieve dusted tweak data")
		}
		tweaks = make([][33]byte, len(rawTweaks))
		for i := range rawTweaks {
			tweaks[i] = rawTweaks[i].TweakData
		}
	} else {
		var tweakIndex *types.TweakIndex
		tweakIndex, err = dblevel.FetchByBlockHashTweakIndex(headerInv.Hash)
		if err == nil {
			tweaks = tweakIndex.Data
		}
	}

	if err != nil {
		logging.L.Err(err).Msg("error fetching tweak index")
		return nil, status.Errorf(codes.Internal, "could not retrieve tweak index data")
	}

	// Convert tweaks to bytes
	tweakBytes := make([][]byte, len(tweaks))
	for i, tweak := range tweaks {
		tweakBytes[i] = tweak[:]
	}

	return &pb.TweakArray{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   headerInv.Hash[:],
			BlockHeight: uint64(headerInv.Height),
		},
		Tweaks: tweakBytes,
	}, nil
}

// GetUTXOArray returns UTXOs for a specific block height
func (s *OracleService) GetUTXOArray(ctx context.Context, req *pb.BlockHeightRequest) (*pb.UTXOArrayResponse, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	utxos, err := dblevel.FetchByBlockHashUTXOs(headerInv.Hash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching UTXOs")
		return nil, status.Errorf(codes.Internal, "could not retrieve UTXO data")
	}

	// Convert internal UTXO types to protobuf types
	pbUtxos := make([]*pb.UTXO, len(utxos))
	for i, utxo := range utxos {
		// todo: g in and change scriptpubKey to at least Byte slice if not even array
		scriptPubKeyBytes, _ := hex.DecodeString(utxo.ScriptPubKey)
		pbUtxos[i] = &pb.UTXO{
			Txid:         utxo.Txid[:],
			Vout:         utxo.Vout,
			Value:        utxo.Value,
			ScriptPubKey: scriptPubKeyBytes,
			BlockHeight:  uint64(utxo.BlockHeight),
			BlockHash:    utxo.BlockHash[:],
			Timestamp:    utxo.Timestamp,
			Spent:        utxo.Spent,
		}
	}

	return &pb.UTXOArrayResponse{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   headerInv.Hash[:],
			BlockHeight: uint64(headerInv.Height),
		},
		Utxos: pbUtxos,
	}, nil
}

// GetFilter returns filter data for a specific block height and type
func (s *OracleService) GetFilter(ctx context.Context, req *pb.GetFilterRequest) (*pb.FilterRepsonse, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	var filter types.Filter
	var err2 error

	switch req.FilterType {
	case pb.FilterType_FILTER_TYPE_SPENT:
		filter, err2 = dblevel.FetchByBlockHashSpentOutpointsFilter(headerInv.Hash)
	case pb.FilterType_FILTER_TYPE_NEW_UTXOS:
		filter, err2 = dblevel.FetchByBlockHashNewUTXOsFilter(headerInv.Hash)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid filter type")
	}

	if err2 != nil {
		logging.L.Err(err2).Msg("error fetching filter")
		return nil, status.Errorf(codes.Internal, "could not retrieve filter data")
	}

	return &pb.FilterRepsonse{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   headerInv.Hash[:],
			BlockHeight: uint64(headerInv.Height),
		},
		FilterData: &pb.FilterData{
			FilterType: req.FilterType,
			Data:       filter.Data,
		},
	}, nil
}

// GetSpentOutpointsIndex returns spent outpoints index for a specific block height
func (s *OracleService) GetSpentOutpointsIndex(ctx context.Context, req *pb.BlockHeightRequest) (*pb.SpentOutpointsIndexResponse, error) {
	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(req.BlockHeight))
	if err != nil {
		logging.L.Err(err).Msg("error fetching block header inv")
		return nil, status.Errorf(codes.Internal, "could not retrieve block data")
	}

	spentOutpoints, err := dblevel.FetchByBlockHashSpentOutpointIndex(headerInv.Hash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching spent outpoints index")
		return nil, status.Errorf(codes.Internal, "could not retrieve spent outpoints data")
	}

	spentOutpointsSliced := make([][]byte, len(spentOutpoints.Data))
	for i := range spentOutpointsSliced {
		spentOutpointsSliced[i] = spentOutpoints.Data[i][:]
	}

	return &pb.SpentOutpointsIndexResponse{
		BlockIdentifier: &pb.BlockIdentifier{
			BlockHash:   headerInv.Hash[:],
			BlockHeight: uint64(headerInv.Height),
		},
		Data: spentOutpointsSliced,
	}, nil
}

// StreamBlockBatchSlim streams lightweight block batches for efficient processing
func (s *OracleService) StreamBlockBatchSlim(
	req *pb.RangedBlockHeightRequest,
	stream pb.OracleService_StreamBlockBatchSlimServer,
) error {
	logging.L.Info().Msgf("streaming slim batches from %d to %d", req.Start, req.End)
	for height := req.Start; height <= req.End; height++ {
		headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(height))
		if err != nil {
			logging.L.Err(err).Msg("error fetching block header inv")
			return status.Errorf(codes.Internal, "could not retrieve block data for height %d", height)
		}

		// Fetch filters
		spentFilter, err := dblevel.FetchByBlockHashSpentOutpointsFilter(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching spent filter")
			return status.Errorf(codes.Internal, "could not retrieve spent filter for height %d", height)
		}

		newUtxosFilter, err := dblevel.FetchByBlockHashNewUTXOsFilter(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching new UTXOs filter")
			return status.Errorf(codes.Internal, "could not retrieve new UTXOs filter for height %d", height)
		}

		// Fetch tweaks
		// todo: make dependant on which index is supported
		// for now it's always full basic
		var tweakIndex *types.TweakIndex
		tweakIndex, err = dblevel.FetchByBlockHashTweakIndex(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Uint64("block_height", height).Msg("failed to pull tweaks")
			return status.Errorf(codes.Internal, "failed to pull tweak index for height %d", height)
		}

		// Convert tweaks to bytes
		tweakBytes := make([][]byte, len(tweakIndex.Data))
		for i, tweak := range tweakIndex.Data {
			tweakBytes[i] = tweak[:]
		}

		batch := &pb.BlockBatchSlim{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   headerInv.Hash[:],
				BlockHeight: uint64(headerInv.Height),
			},
			Tweaks: tweakBytes,
			NewUtxosFilter: &pb.FilterData{
				FilterType: pb.FilterType_FILTER_TYPE_NEW_UTXOS, Data: newUtxosFilter.Data,
			},
			SpentUtxosFilter: &pb.FilterData{
				FilterType: pb.FilterType_FILTER_TYPE_SPENT, Data: spentFilter.Data,
			},
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
	for height := req.Start; height <= req.End; height++ {
		headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(height))
		if err != nil {
			logging.L.Err(err).Msg("error fetching block header inv")
			return status.Errorf(codes.Internal, "could not retrieve block data for height %d", height)
		}

		// Fetch all data for this block
		// todo: make dependant on which index is supported
		// for now it's always full basic
		var tweakIndex *types.TweakIndex
		tweakIndex, err = dblevel.FetchByBlockHashTweakIndex(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Uint64("block_height", height).Msg("failed to pull tweaks")
			return status.Errorf(codes.Internal, "failed to pull tweak index for height %d", height)
		}

		utxos, err := dblevel.FetchByBlockHashUTXOs(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching UTXOs")
			return status.Errorf(codes.Internal, "could not retrieve UTXOs for height %d", height)
		}

		spentFilter, err := dblevel.FetchByBlockHashSpentOutpointsFilter(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching spent filter")
			return status.Errorf(codes.Internal, "could not retrieve spent filter for height %d", height)
		}

		newUtxosFilter, err := dblevel.FetchByBlockHashNewUTXOsFilter(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching new UTXOs filter")
			return status.Errorf(codes.Internal, "could not retrieve new UTXOs filter for height %d", height)
		}

		spentOutpoints, err := dblevel.FetchByBlockHashSpentOutpointIndex(headerInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching spent outpoints")
			return status.Errorf(codes.Internal, "could not retrieve spent outpoints for height %d", height)
		}

		// Convert tweaks to bytes
		tweakBytes := make([][]byte, len(tweakIndex.Data))
		for i, tweak := range tweakIndex.Data {
			tweakBytes[i] = tweak[:]
		}

		// Convert UTXOs to protobuf format
		pbUtxos := make([]*pb.UTXO, len(utxos))
		for i, utxo := range utxos {
			scripPubKeyBytes, _ := hex.DecodeString(utxo.ScriptPubKey)
			pbUtxos[i] = &pb.UTXO{
				Txid:         utxo.Txid[:],
				Vout:         uint32(utxo.Vout),
				Value:        utxo.Value,
				ScriptPubKey: scripPubKeyBytes,
				BlockHeight:  uint64(utxo.BlockHeight),
				BlockHash:    utxo.BlockHash[:],
				Timestamp:    utxo.Timestamp,
				Spent:        utxo.Spent,
			}
		}

		spentOutpointsSliced := make([][]byte, len(spentOutpoints.Data))
		for i := range spentOutpointsSliced {
			spentOutpointsSliced[i] = spentOutpoints.Data[i][:]
		}

		batch := &pb.BlockBatchFull{
			BlockIdentifier: &pb.BlockIdentifier{
				BlockHash:   headerInv.Hash[:],
				BlockHeight: uint64(headerInv.Height),
			},
			Tweaks:           tweakBytes,
			Utxos:            pbUtxos,
			NewUtxosFilter:   &pb.FilterData{FilterType: pb.FilterType_FILTER_TYPE_NEW_UTXOS, Data: newUtxosFilter.Data},
			SpentUtxosFilter: &pb.FilterData{FilterType: pb.FilterType_FILTER_TYPE_SPENT, Data: spentFilter.Data},
			SpentUtxos:       spentOutpointsSliced,
		}

		if err := stream.Send(batch); err != nil {
			logging.L.Err(err).Msg("error sending block batch")
			return status.Errorf(codes.Internal, "failed to send block batch for height %d", height)
		}
	}

	return nil
}
