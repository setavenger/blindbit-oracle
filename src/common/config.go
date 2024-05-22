package common

import (
	"errors"
	"os"

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

	//Tweaks
	viper.SetDefault("tweaks_only", false)
	viper.SetDefault("tweaks_full_basic", true)
	viper.SetDefault("tweaks_full_with_dust_filter", false)
	viper.SetDefault("tweaks_cut_through_with_dust_filter", false)

	/* read and set config variables */
	// General
	SyncStartHeight = viper.GetUint32("sync_start_height")
	Host = viper.GetString("host")

	// Performance
	MaxParallelRequests = viper.GetUint16("max_parallel_requests")
	MaxParallelTweakComputations = viper.GetInt("max_parallel_tweak_computations")

	// RPC
	RpcEndpoint = viper.GetString("rpc_endpoint")
	CookiePath = viper.GetString("cookie_path")
	RpcPass = viper.GetString("rpc_pass")
	RpcUser = viper.GetString("rpc_user")

	// Tweaks
	TweaksOnly = viper.GetBool("tweaks_only")
	TweakIndexFullNoDust = viper.GetBool("tweaks_full_basic")
	TweakIndexFullIncludingDust = viper.GetBool("tweaks_full_with_dust_filter")
	TweaksCutThroughWithDust = viper.GetBool("tweaks_cut_through_with_dust_filter")

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

	// todo print settings
	InfoLogger.Printf("tweaks_only: %t\n", TweaksOnly)
	InfoLogger.Printf("tweaks_full_basic: %t\n", TweakIndexFullNoDust)
	InfoLogger.Printf("tweaks_full_with_dust_filter: %t\n", TweakIndexFullIncludingDust)
	InfoLogger.Printf("tweaks_cut_through_with_dust_filter: %t\n", TweaksCutThroughWithDust)

	if !TweakIndexFullNoDust && !TweakIndexFullIncludingDust && !TweaksCutThroughWithDust {
		WarningLogger.Println("no tweaks are being collected, all tweak settings were set to 0")
		WarningLogger.Println("make sure your configuration loaded correctly, check example blindbit.toml for configuration")
	}

	if TweaksCutThroughWithDust && !TweaksOnly {
		err := errors.New("cut through requires tweaks_only to be set to 1")
		ErrorLogger.Println(err)
		os.Exit(1)
	}
}
