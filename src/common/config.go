package common

import (
	"github.com/spf13/viper"
)

func LoadConfigs(pathToConfig string) {
	// Set the file name of the configurations file
	viper.SetConfigFile(pathToConfig)

	// Handle errors reading the config file
	if err := viper.ReadInConfig(); err != nil {
		ErrorLogger.Println("No config file detected", err.Error())
		return
	}

	/* set defaults */
	// network
	viper.SetDefault("max_parallel_requests", MaxParallelRequests)
	viper.SetDefault("host", Host)
	viper.SetDefault("chain", "signet")

	// RPC endpoint only. Fails if others are not set
	viper.SetDefault("rpc_endpoint", RpcEndpoint)

	//Others
	viper.SetDefault("tweaks_only", false)

	/* read and set config variables */
	Host = viper.GetString("host")

	MaxParallelRequests = viper.GetUint16("max_parallel_requests")
	MaxParallelTweakComputations = viper.GetInt("max_parallel_tweak_computations")

	RpcEndpoint = viper.GetString("rpc_endpoint")
	CookiePath = viper.GetString("cookie_path")
	RpcPass = viper.GetString("rpc_pass")
	RpcUser = viper.GetString("rpc_user")

	SyncStartHeight = viper.GetUint32("sync_start_height")

	TweaksOnly = viper.GetBool("tweaks_only")

	chainInput := viper.GetString("chain")

	switch chainInput {
	case "main":
		Chain = Mainnet
	case "signet":
		Chain = Signet
	case "regtest":
		Chain = Regtest
	case "testnet":
		Chain = Testnet3
	default:
		panic("chain undefined")
	}
}
