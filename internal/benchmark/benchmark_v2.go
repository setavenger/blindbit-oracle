package benchmark

import (
	"context"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BenchmarkV2 runs the v2 gRPC streaming benchmark
func BenchmarkV2(startHeight, endHeight uint64, grpcHost string) {
	logging.L.Info().Msgf("Starting v2 gRPC streaming benchmark from height %d to %d", startHeight, endHeight)

	// Connect to gRPC server
	conn, err := grpc.Dial(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logging.L.Err(err).Msg("failed to connect to gRPC server")
		return
	}
	defer conn.Close()

	client := pb.NewOracleServiceClient(conn)

	startTime := time.Now()

	// Use streaming API to fetch all blocks at once
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := &pb.RangedBlockHeightRequest{
		Start: startHeight,
		End:   endHeight,
	}

	stream, err := client.StreamBlockBatchSlim(ctx, req)
	if err != nil {
		logging.L.Err(err).Msg("failed to start streaming")
		return
	}

	blocksProcessed := uint64(0)

	for {
		batch, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			logging.L.Err(err).Msg("failed to receive batch")
			break
		}

		blocksProcessed++
		logging.L.Debug().Uint64("height", uint64(batch.BlockIdentifier.BlockHeight)).Msg("received block batch")

		// Use batch to avoid compiler warning
		_ = batch
	}

	duration := time.Since(startTime)

	logging.L.Info().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Uint64("blocks_processed", blocksProcessed).
		Dur("total_duration", duration).
		Float64("blocks_per_second", float64(blocksProcessed)/duration.Seconds()).
		Msg("v2 gRPC streaming benchmark completed")
}
