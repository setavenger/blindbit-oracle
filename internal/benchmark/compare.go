package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/networking"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CompareV1V2Results compares v1 and v2 results for validation
func CompareV1V2Results(startHeight, endHeight uint64, httpURL, grpcHost string) {
	logging.L.Info().Msgf("Comparing v1 and v2 results from height %d to %d", startHeight, endHeight)

	for height := startHeight; height <= endHeight; height++ {
		logging.L.Info().Uint64("height", height).Msg("comparing block")

		// Fetch v1 data
		v1Data, err := fetchBlockDataV1(height, &networking.ClientBlindBit{BaseURL: httpURL})
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to fetch v1 data")
			continue
		}

		// Fetch v2 data
		v2Data, err := fetchBlockDataV2(height, grpcHost)
		if err != nil {
			logging.L.Err(err).Uint64("height", height).Msg("failed to fetch v2 data")
			continue
		}

		// Quick validation
		if len(v1Data.Tweaks) != len(v2Data.Tweaks) {
			logging.L.Warn().Uint64("height", height).
				Int("v1_tweaks", len(v1Data.Tweaks)).
				Int("v2_tweaks", len(v2Data.Tweaks)).
				Msg("tweak count mismatch")
		}

		if v1Data.FilterNew != nil && v2Data.FilterNew != nil {
			if len(v1Data.FilterNew.Data) != len(v2Data.FilterNew.Data) {
				logging.L.Warn().Uint64("height", height).
					Int("v1_filter_new", len(v1Data.FilterNew.Data)).
					Int("v2_filter_new", len(v2Data.FilterNew.Data)).
					Msg("new UTXOs filter data length mismatch")
			}
		}

		if v1Data.FilterSpent != nil && v2Data.FilterSpent != nil {
			if len(v1Data.FilterSpent.Data) != len(v2Data.FilterSpent.Data) {
				logging.L.Warn().Uint64("height", height).
					Int("v1_filter_spent", len(v1Data.FilterSpent.Data)).
					Int("v2_filter_spent", len(v2Data.FilterSpent.Data)).
					Msg("spent outpoints filter data length mismatch")
			}
		}

		logging.L.Info().Uint64("height", height).Msg("block comparison completed")
	}

	logging.L.Info().Msg("All block comparisons completed")
}

// fetchBlockDataV2 fetches block data using v2 gRPC API
func fetchBlockDataV2(height uint64, grpcHost string) (*BlockDataV2, error) {
	// Connect to gRPC server
	conn, err := grpc.Dial(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := pb.NewOracleServiceClient(conn)

	// Use streaming API to fetch single block
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := &pb.RangedBlockHeightRequest{
		Start: height,
		End:   height,
	}

	stream, err := client.StreamBlockBatchSlim(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming: %v", err)
	}

	// Receive the single batch
	batch, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive batch: %v", err)
	}

	// Convert protobuf data to our format
	return &BlockDataV2{
		Height:      height,
		FilterNew:   convertFilterData(batch.NewUtxosFilter, batch.BlockIdentifier),
		FilterSpent: convertFilterData(batch.SpentUtxosFilter, batch.BlockIdentifier),
		Tweaks:      convertTweaksToArray(batch.Tweaks),
	}, nil
}

// convertTweaksToArray converts [][]byte to [][33]byte
func convertTweaksToArray(tweaks [][]byte) [][33]byte {
	result := make([][33]byte, len(tweaks))
	for i, tweak := range tweaks {
		if len(tweak) == 33 {
			copy(result[i][:], tweak)
		}
	}
	return result
}

// convertFilterData converts protobuf FilterData to networking.Filter
func convertFilterData(pbFilter *pb.FilterData, blockIdentifier *pb.BlockIdentifier) *networking.Filter {
	if pbFilter == nil {
		return nil
	}

	// Convert block hash from bytes to [32]byte
	var blockHash [32]byte
	if len(blockIdentifier.BlockHash) == 32 {
		copy(blockHash[:], blockIdentifier.BlockHash)
	}

	return &networking.Filter{
		FilterType:  uint8(pbFilter.FilterType),
		BlockHeight: blockIdentifier.BlockHeight,
		BlockHash:   blockHash,
		Data:        pbFilter.Data,
	}
}

// BlockDataV2 represents the block data structure for v2
type BlockDataV2 struct {
	Height      uint64
	FilterNew   *networking.Filter
	FilterSpent *networking.Filter
	Tweaks      [][33]byte
}
