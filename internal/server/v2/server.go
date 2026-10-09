package v2

import (
	"net"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/proto/pb"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func RunGRPCServer(db database.DB) {
	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register the OracleService
	oracleService := NewOracleService(db)
	pb.RegisterOracleServiceServer(grpcServer, oracleService)

	// Enable reflection for debugging (optional)
	reflection.Register(grpcServer)

	// Create listener for gRPC
	lis, err := net.Listen("tcp", config.GRPCHost)
	if err != nil {
		logging.L.Err(err).Msg("failed to listen for gRPC")
		panic(err)
	}

	logging.L.Info().Msgf("Starting gRPC server on host %s", config.GRPCHost)

	// Start gRPC server
	if err := grpcServer.Serve(lis); err != nil {
		logging.L.Err(err).Msg("failed to serve gRPC")
		panic(err)
	}
}
