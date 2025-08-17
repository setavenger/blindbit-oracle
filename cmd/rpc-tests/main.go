package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/indexer"
)

var (
	blockHashStr = "00000089e14c2ace89680e4edbd178324c4f44c950d7fc4a4833be076050873c"
	blockHash    chainhash.Hash
)

// todo: go on rabbit hole whether the memory assignment can be optimised

func main() {
	blockHashBytes, _ := hex.DecodeString(blockHashStr)
	utils.ReverseBytes(blockHashBytes)
	err := blockHash.SetBytes(blockHashBytes)
	if err != nil {
		panic(err)
	}
	var b *btcutil.Block
	b, err = getBlockByHash(&blockHash)
	if err != nil {
		panic(err)
	}

	_ = b

	spentTxOut, err := getSpentUtxos(&blockHash)
	if err != nil {
		panic(err)
	}

	fmt.Println(spentTxOut)

	return
}

func getSpentUtxos(hash *chainhash.Hash) ([][]*wire.TxOut, error) {
	if hash == nil {
		hash = (*chainhash.Hash)(blockHash.CloneBytes())
	}

	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:38332/rest/spenttxouts/%s.bin", hash.String()),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return nil, nil
	}

	fmt.Println(req.URL.String())

	// Make the HTTP request...
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return nil, nil
	}
	defer resp.Body.Close()
	// Read and unmarshal the response...
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.L.Err(err).
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("error reading response body")
		return nil, nil
	}

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("response", string(body)).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	return indexer.ParseSpentTxOuts(body)
}

func getBlockByHash(hash *chainhash.Hash) (block *btcutil.Block, err error) {
	if hash == nil {
		hash = (*chainhash.Hash)(blockHash.CloneBytes())
	}

	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:38332/rest/block/%s.bin", hash.String()),
		nil,
	)
	if err != nil {
		err = fmt.Errorf("error creating request: %v", err)
		logging.L.Err(err).Msg("error creating request")
		return
	}

	// Set headers and auth...
	req.Header.Set("Content-Type", "application/json")
	authText := fmt.Sprintf("%s:%s", config.RpcUser, config.RpcPass)
	auth := base64.StdEncoding.EncodeToString([]byte(authText))
	req.Header.Add("Authorization", "Basic "+auth)

	// Make the HTTP request...
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("error performing request: %v", err)
		logging.L.Err(err).Msg("error performing request")
		return
	}
	defer resp.Body.Close()
	// Read and unmarshal the response...
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.L.Err(err).
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("error reading response body")
		return
	}

	if resp.StatusCode != 200 {
		logging.L.Fatal().
			Str("response", string(body)).
			Str("status", resp.Status).
			Msg("bad status code")
	}

	return btcutil.NewBlockFromBytes(body)
	// if err != nil {
	// 	logging.L.Err(err).Msg("failed to deserialise bytes to block")
	// 	return
	// }

	// fmt.Printf("block_hash: %s\n", block.Hash().String())
	// fmt.Printf("height:     %d\n", block.Height())
	// fmt.Printf("tx_count:   %d\n", len(block.Transactions()))
	// fmt.Printf("tx - 1:\n%+v\n", block.Transactions()[1].MsgTx().TxIn[0].PreviousOutPoint)
	//
	// return b
}
