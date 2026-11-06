package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// pooling of api calls to potentially improve performance.
// Requests that fail because Core is unavailable are retried by restGet.
var httpClient = &http.Client{
	Timeout: 100 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   50 * time.Second,
			KeepAlive: 300 * time.Second,
		}).DialContext,

		// Pooling / reuse
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0,

		IdleConnTimeout:       900 * time.Second,
		TLSHandshakeTimeout:   50 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
	},
}

type ChainInfo struct {
	Chain                string          `json:"chain"`
	Blocks               int64           `json:"blocks"`
	Headers              int64           `json:"headers"`
	BestBlockHash        string          `json:"bestblockhash"`
	Bits                 string          `json:"bits"`
	Target               string          `json:"target"`
	Difficulty           float64         `json:"difficulty"`
	Time                 int64           `json:"time"`
	MedianTime           int64           `json:"mediantime"`
	VerificationProgress float64         `json:"verificationprogress"`
	InitialBlockDownload bool            `json:"initialblockdownload"`
	ChainWork            string          `json:"chainwork"`
	SizeOnDisk           int64           `json:"size_on_disk"`
	Pruned               bool            `json:"pruned"`
	SignetChallenge      string          `json:"signet_challenge"`
	Warnings             json.RawMessage `json:"warnings"`
}

const chainInfoPath = "/rest/chaininfo.json"

// GetChainInfo returns the node's chain state, waiting for Core while it is
// unavailable. ctx bounds the wait, so a caller that is being cancelled is not
// held up by an unresponsive node.
func GetChainInfo(ctx context.Context) (*ChainInfo, error) {
	return restGet(ctx, chainInfoPath, decodeChainInfo)
}

// getChainInfoOnce makes a single attempt, for callers that must not wait.
func getChainInfoOnce(ctx context.Context) (*ChainInfo, error) {
	return restGetOnce(ctx, chainInfoPath, decodeChainInfo)
}

func decodeChainInfo(r io.Reader) (*ChainInfo, error) {
	var chainInfo ChainInfo
	if err := json.NewDecoder(r).Decode(&chainInfo); err != nil {
		return nil, err
	}
	return &chainInfo, nil
}

func getBlockHashByHeight(ctx context.Context, height int64) (*chainhash.Hash, error) {
	return restGet(ctx, fmt.Sprintf("/rest/blockhashbyheight/%d.bin", height), decodeBlockHash)
}

func decodeBlockHash(r io.Reader) (*chainhash.Hash, error) {
	var blockhash chainhash.Hash
	if _, err := io.ReadFull(r, blockhash[:]); err != nil {
		return nil, err
	}
	return &blockhash, nil
}

func getSpentUtxos(ctx context.Context, blockhash string) ([][]*wire.TxOut, error) {
	return restGet(ctx, fmt.Sprintf("/rest/spenttxouts/%s.bin", blockhash), ParseSpentTxOuts)
}

func getBlockByHash(ctx context.Context, blockhash string) (*btcutil.Block, error) {
	return restGet(ctx, fmt.Sprintf("/rest/block/%s.bin", blockhash), btcutil.NewBlockFromReader)
}
