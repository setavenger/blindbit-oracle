package common

import (
	"github.com/spf13/viper"
)

func LoadConfigs(pathToConfig string) {
	// Set the file name of the configurations file
	viper.SetConfigFile(pathToConfig)

	// Handle errors reading the config file
	if err := viper.ReadInConfig(); err != nil {
		WarningLogger.Println("No config file detected", err.Error())
		return
	}

	/* set defaults */
	// network
	viper.SetDefault("max_parallel_requests", MaxParallelRequests)
	viper.SetDefault("host", Host)

	// RPC endpoint only. Fails if others are not set
	viper.SetDefault("rpc_endpoint", RpcEndpoint)

	/* read and set config variables */
	Host = viper.GetString("host")

	MaxParallelRequests = viper.GetUint16("max_parallel_requests")
	MaxParallelTweakComputations = viper.GetInt("max_parallel_tweak_computations")

	RpcEndpoint = viper.GetString("rpc_endpoint")
	RpcPass = viper.GetString("rpc_pass")
	RpcUser = viper.GetString("rpc_user")

	SyncStartHeight = viper.GetUint32("sync_start_height")
}
