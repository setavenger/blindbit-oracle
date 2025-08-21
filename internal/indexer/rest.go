package indexer

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

// pooling of api calls to potentially improve performance
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		// Pooling / reuse
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0,

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

type ChainInfo struct {
	Chain                string   `json:"chain"`
	Blocks               int64    `json:"blocks"`
	Headers              int64    `json:"headers"`
	BestBlockHash        string   `json:"bestblockhash"`
	Bits                 string   `json:"bits"`
	Target               string   `json:"target"`
	Difficulty           float64  `json:"difficulty"`
	Time                 int64    `json:"time"`
	MedianTime           int64    `json:"mediantime"`
	VerificationProgress float64  `json:"verificationprogress"`
	InitialBlockDownload bool     `json:"initialblockdownload"`
	ChainWork            string   `json:"chainwork"`
	SizeOnDisk           int64    `json:"size_on_disk"`
	Pruned               bool     `json:"pruned"`
	SignetChallenge      string   `json:"signet_challenge"`
	Warnings             []string `json:"warnings"`
}

func GetChainInfo() (*ChainInfo, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/rest/chaininfo.json", config.RpcEndpoint),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return nil, err
	}

	resp, err := httpClient.Do(req) // <-- reuse the shared client
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("url", req.URL.String()).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	var chainInfo ChainInfo
	err = json.NewDecoder(resp.Body).Decode(&chainInfo)
	if err != nil {
		logging.L.Err(err).Msg("unabel to decode body")
		return nil, err
	}

	return &chainInfo, err
}

func getBlockHashByHeight(height int64) (*chainhash.Hash, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s/rest/blockhashbyheight/%d.bin", config.RpcEndpoint, height),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return nil, err
	}

	resp, err := httpClient.Do(req) // <-- reuse the shared client
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("url", req.URL.String()).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	var blockhash chainhash.Hash
	resp.Body.Read(blockhash[:])

	return &blockhash, err
}

func getSpentUtxos(blockhash string) ([][]*wire.TxOut, error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:38332/rest/spenttxouts/%s.bin", blockhash),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return nil, err
	}

	resp, err := httpClient.Do(req) // <-- reuse the shared client
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("url", req.URL.String()).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	return ParseSpentTxOuts(resp.Body)
}

func getBlockByHash(blockhash string) (block *btcutil.Block, err error) {
	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:38332/rest/block/%s.bin", blockhash),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return
	}

	resp, err := httpClient.Do(req) // <-- reuse the shared client
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("url", req.URL.String()).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	return btcutil.NewBlockFromReader(resp.Body)
}
