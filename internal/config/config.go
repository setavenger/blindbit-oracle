package config

import (
	"errors"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/spf13/viper"
)

func LoadConfigs(pathToConfig string) {
	// Set the file name of the configurations file
	viper.SetConfigFile(pathToConfig)

	// Handle errors reading the config file
	if err := viper.ReadInConfig(); err != nil {
		logging.L.Warn().Err(err).Msg("No config file detected")
	}

	/* set defaults */
	viper.SetDefault("max_parallel_requests", MaxParallelRequests)
	viper.SetDefault("max_cpu_cores", MaxCPUCores)
	viper.SetDefault("http_host", HTTPHost)
	viper.SetDefault("grpc_host", GRPCHost)
	viper.SetDefault("chain", "signet")

	viper.SetDefault("rpc_endpoint", RpcEndpoint)
	viper.SetDefault("rest_endpoint", RestEndpoint)

	viper.SetDefault("tweaks_only", false)
	viper.SetDefault("tweaks_full_basic", true)
	viper.SetDefault("tweaks_full_with_dust_filter", false)
	viper.SetDefault("tweaks_cut_through_with_dust_filter", false)
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_path", "")
	viper.SetDefault("log_to_console", true)
	// Bind viper keys to environment variables (optional, for backup)
	viper.AutomaticEnv()
	viper.BindEnv("http_host", "HTTP_HOST")
	viper.BindEnv("grpc_host", "GRPC_HOST")
	viper.BindEnv("chain", "CHAIN")
	viper.BindEnv("core_rpc_endpoint", "CORE_RPC_ENDPOINT")
	viper.BindEnv("core_rest_endpoint", "CORE_REST_ENDPOINT")
	viper.BindEnv("cookie_path", "COOKIE_PATH")
	viper.BindEnv("rpc_pass", "RPC_PASS")
	viper.BindEnv("rpc_user", "RPC_USER")
	viper.BindEnv("sync_start_height", "SYNC_START_HEIGHT")
	viper.BindEnv("max_parallel_requests", "MAX_PARALLEL_REQUESTS")
	viper.BindEnv("max_parallel_tweak_computations", "MAX_PARALLEL_TWEAK_COMPUTATIONS")
	viper.BindEnv("max_cpu_cores", "MAX_CPU_CORES")
	viper.BindEnv("tweaks_only", "TWEAKS_ONLY")
	viper.BindEnv("tweaks_full_basic", "TWEAKS_FULL_BASIC")
	viper.BindEnv("tweaks_full_with_dust_filter", "TWEAKS_FULL_WITH_DUST_FILTER")
	viper.BindEnv("tweaks_cut_through_with_dust_filter", "TWEAKS_CUT_THROUGH_WITH_DUST_FILTER")
	viper.BindEnv("log_level", "LOG_LEVEL")

	/* read and set config variables */
	// General
	SyncStartHeight = viper.GetUint32("sync_start_height")
	HTTPHost = viper.GetString("http_host")
	GRPCHost = viper.GetString("grpc_host")
	LogLevel = viper.GetString("log_level")
	LogsPath = viper.GetString("log_path")
	LogToConsole = viper.GetBool("log_to_console")

	// Performance
	MaxParallelRequests = viper.GetUint16("max_parallel_requests")
	MaxParallelTweakComputations = viper.GetInt("max_parallel_tweak_computations")
	MaxCPUCores = viper.GetInt("max_cpu_cores")

	// RPC
	RpcEndpoint = viper.GetString("core_rpc_endpoint")
	RestEndpoint = viper.GetString("core_rest_endpoint")
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
		logging.L.Fatal().Msg("chain undefined")
		return
	}

	switch LogLevel {
	case "trace":
		logging.SetLogLevel(zerolog.TraceLevel)
	case "info":
		logging.SetLogLevel(zerolog.InfoLevel)
	case "debug":
		logging.SetLogLevel(zerolog.DebugLevel)
	case "warn":
		logging.SetLogLevel(zerolog.WarnLevel)
	case "error":
		logging.SetLogLevel(zerolog.ErrorLevel)
	}

	// todo print settings
	logging.L.Info().Msgf("tweaks_only: %t", TweaksOnly)
	logging.L.Info().Msgf("tweaks_full_basic: %t", TweakIndexFullNoDust)
	logging.L.Info().Msgf("tweaks_full_with_dust_filter: %t", TweakIndexFullIncludingDust)
	logging.L.Info().Msgf("tweaks_cut_through_with_dust_filter: %t", TweaksCutThroughWithDust)

	if !TweakIndexFullNoDust && !TweakIndexFullIncludingDust && !TweaksCutThroughWithDust {
		logging.L.Warn().Msg("no tweaks are being collected, all tweak settings were set to 0")
		logging.L.Warn().Msg("make sure your configuration loaded correctly, check example blindbit.toml for configuration")
	}

	if TweaksCutThroughWithDust && TweaksOnly {
		err := errors.New("cut through requires tweaks_only to be set to 0")
		logging.L.Fatal().Err(err).Msg("cut through requires tweaks_only to be set to 0")
		return
	}

	if RpcEndpoint != "" {
		if CookiePath != "" {
			data, err := os.ReadFile(CookiePath)
			if err != nil {
				logging.L.Fatal().Err(err).Msg("error reading cookie file")
			}

			credentials := strings.Split(string(data), ":")
			if len(credentials) != 2 {
				logging.L.Fatal().Msg("cookie file is invalid")
			}
			RpcUser = credentials[0]
			RpcPass = credentials[1]
		}

		if RpcUser == "" {
			logging.L.Fatal().Msg("rpc user not set") // todo use cookie file to circumvent this requirement
		}

		if RpcPass == "" {
			logging.L.Fatal().Msg("rpc pass not set") // todo use cookie file to circumvent this requirement
		}
	}

}
