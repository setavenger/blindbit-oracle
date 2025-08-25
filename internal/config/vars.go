package config

import (
	"runtime"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
)

// TaprootActivation
// todo might be inapplicable due to transactions that have taproot prevouts from before the activation
//
//	is relevant for the height-to-hash lookup in the db

var (
	LogLevel = "info"
)

const (
	TaprootActivation    uint32 = 709632
	ConfigFileName       string = "blindbit.toml"
	DefaultBaseDirectory string = "~/.blindbit-oracle"
)

var (
	TweaksOnly                  bool
	TweakIndexFullNoDust        bool
	TweakIndexFullIncludingDust bool
	TweaksCutThroughWithDust    bool
)

var (
	RpcEndpoint  = "http://127.0.0.1:8332" // default local node
	RestEndpoint = ""                      // default local node
	CookiePath   = ""
	RpcUser      = ""
	RpcPass      = ""

	BaseDirectory = ""
	DBPath        = ""
	LogsPath      = ""

	HTTPHost = "127.0.0.1:8000"
	GRPCHost = "" // default value is empty (deactivated)
)

type chain int

const (
	Unknown chain = iota
	Mainnet
	Signet
	Regtest
	Testnet3
)

// control vars
var (
	SyncStartHeight uint32 = 833_000 // random block where BIP352 was not active yet. todo change to actual number
	// MinHeightToProcess No block below this number will be processed
	// todo is this actually needed
	//MinHeightToProcess uint32 = 833_000

	Chain = Unknown

	// SyncHeadersMaxPerCall how many headers will maximally be requested in one batched RPC call
	SyncHeadersMaxPerCall uint32 = 10_000
	// MaxParallelRequests sets how many RPC calls will be made in parallel to the Node
	MaxParallelRequests uint16 = 2
	// MaxParallelTweakComputations number of parallel processes which will be spawned in order to compute the tweaks for a given block
	MaxParallelTweakComputations = 2

	// We default to max num cores - 2
	MaxCPUCores = runtime.NumCPU() - 2

	// PruneFrequency every x blocks the data will be checked and pruned
	// possible routines: -remove utxos for 100% spent transaction
	PruneFrequency = 72
)

// one has to call SetDirectories otherwise config.DBPath will be empty
var (
	DBPathHeaders              string
	DBPathHeadersInv           string // for height to blockHash mapping
	DBPathFilters              string
	DBPathTweaks               string
	DBPathTweakIndex           string
	DBPathUTXOs                string
	DBPathTweakIndexDust       string
	DBPathSpentOutpointsIndex  string
	DBPathSpentOutpointsFilter string
)

// NumsH = 0x50929b74c1a04954b78b4b6035e97a5e078a5a0f28ec96d547bfee9ace803ac0
var NumsH = []byte{80, 146, 155, 116, 193, 160, 73, 84, 183, 139, 75, 96, 53, 233, 122, 94, 7, 138, 90, 15, 40, 236, 150, 213, 71, 191, 238, 154, 206, 128, 58, 192}

func SetDirectories() {
	BaseDirectory = utils.ResolvePath(BaseDirectory)

	DBPath = BaseDirectory + "/data"
	LogsPath = BaseDirectory + "/logs"

	DBPathHeaders = DBPath + "/headers"
	DBPathHeadersInv = DBPath + "/headers-inv"
	DBPathFilters = DBPath + "/filters"
	DBPathTweaks = DBPath + "/tweaks"
	DBPathTweakIndex = DBPath + "/tweak-index"
	DBPathTweakIndexDust = DBPath + "/tweak-index-dust"
	DBPathUTXOs = DBPath + "/utxos"
	DBPathSpentOutpointsIndex = DBPath + "/spent-index"
	DBPathSpentOutpointsFilter = DBPath + "/spent-filter"
}

func HeaderMustSyncHeight() uint32 {
	switch Chain {
	case Mainnet:
		// height based on heuristic checks to see where no old taproot style coins were locked
		return 500_000
	case Signet:
		return 1
	case Regtest:
		return 1
	case Testnet3:
		return 1
	case Unknown:
		logging.L.Panic().Msg("chain not defined")
		return 0
	default:
		return 1
	}
}

func ChainToString(c chain) string {
	switch Chain {
	case Mainnet:
		return "main"
	case Signet:
		return "signet"
	case Regtest:
		return "regtest"
	case Testnet3:
		return "testnet"
	default:
		logging.L.Panic().Msg("chain not defined")
		return ""
	}

}
