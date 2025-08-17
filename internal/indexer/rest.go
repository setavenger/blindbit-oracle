package indexer

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
)

// pooling of api calls to potentially improve performance
var httpClient = &http.Client{
	Timeout: 10 * time.Second, // overall request deadline (guardrail)
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		// Pooling / reuse
		MaxIdleConns:        200, // total idle across hosts
		MaxIdleConnsPerHost: 100, // bump to your parallelism per host
		MaxConnsPerHost:     0,   // 0 = no hard cap; let HTTP/2 multiplex

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,

		// Keep-alives are ON by default; don't disable them.
	},
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
		return nil, nil
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("status", resp.Status).
			Msg("bad status code")
	}

	return btcutil.NewBlockFromReader(resp.Body)
}
