package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

func makeRPCRequest(rpcData, result any) error {
	payload, err := json.Marshal(rpcData)
	if err != nil {
		logging.L.Err(err).Msg("error marshaling RPC data")
		return fmt.Errorf("error marshaling RPC data: %v", err)
	}

	// Prepare the request...
	req, err := http.NewRequest("POST", config.RpcEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		logging.L.Err(err).Msg("error creating request")
		return fmt.Errorf("error creating request: %v", err)
	}

	logging.L.Trace().Any("req", rpcData).Msg("")

	// Set headers and auth...
	req.Header.Set("Content-Type", "application/json")
	authText := fmt.Sprintf("%s:%s", config.RpcUser, config.RpcPass)
	auth := base64.StdEncoding.EncodeToString([]byte(authText))
	req.Header.Add("Authorization", "Basic "+auth)

	// Make the HTTP request...
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logging.L.Err(err).Msg("error performing request")
		return fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	// Read and unmarshal the response...
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.L.Err(err).
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("error reading response body")
		return err
	}

	if resp.StatusCode >= 400 {
		err = fmt.Errorf("request failed")
		logging.L.Err(err).
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("error unmarshaling response")
		return err
	}

	err = json.Unmarshal(body, result)
	if err != nil {
		logging.L.Err(err).
			Int("status_code", resp.StatusCode).
			Str("body", string(body)).
			Msg("error unmarshaling response")

		return err
	}

	return nil
}

func GetFullBlockPerBlockHash(blockHash string) (*types.Block, error) {
	rpcData := types.RPCRequest{
		JSONRPC: "1.0",
		ID:      "blindbit-silent-payment-backend-v0",
		Method:  "getblock",
		Params:  []interface{}{blockHash, 3}, // 3 for maximum verbosity such that we easily get the prevouts for tweaking
	}

	var rpcResponse types.RPCResponseBlock
	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		logging.L.Err(err).Msg("error getting full block per block hash")
		return nil, err
	}

	if rpcResponse.Error != "" {
		err = errors.New(string(rpcResponse.Error))
		logging.L.Err(err).Msg("RPC error")
		return nil, err
	}

	return &rpcResponse.Block, nil
}

func GetBestBlockHash() (string, error) {
	rpcData := types.RPCRequest{
		JSONRPC: "1.0",
		ID:      "blindbit-silent-payment-backend-v0",
		Method:  "getbestblockhash",
		Params:  []interface{}{},
	}

	var rpcResponse types.RPCResponseHighestHash
	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		logging.L.Err(err).Msg("error getting best block hash")
		return "", err
	}

	if rpcResponse.Error != "" {
		err = errors.New(string(rpcResponse.Error))
		logging.L.Err(err).Msg("RPC error")
		return "", err
	}

	return rpcResponse.Result, nil
}

func GetBlockHeadersBatch(heights []uint32) ([]types.BlockHeader, error) {
	// Prepare the batch request
	batch := make([]types.RPCRequest, len(heights))
	headers := make([]types.BlockHeader, len(heights))

	// Initialize the batch with `getblockhash` requests
	for idx, height := range heights {
		batch[idx] = types.RPCRequest{
			JSONRPC: "1.0",
			ID:      "blindbit-silent-payment-backend-v0",
			Method:  "getblockhash",
			Params:  []interface{}{height},
		}
	}

	// Perform the batched `getblockhash` requests
	hashResponses := make([]struct {
		ID     string      `json:"id"`
		Result string      `json:"result,omitempty"`
		Error  interface{} `json:"error,omitempty"`
	}, len(heights))

	err := makeRPCRequest(batch, &hashResponses)
	if err != nil {
		return nil, fmt.Errorf("error fetching block hashes: %v", err)
	}

	// Prepare a new batch for `getblockheader` requests using the hashes from the previous step
	for i, hashResponse := range hashResponses {
		if hashResponse.Error != nil {
			return nil, fmt.Errorf("error in hash response: %v", hashResponse.Error)
		}

		batch[i] = types.RPCRequest{
			JSONRPC: "1.0",
			ID:      "blindbit-silent-payment-backend-v0",
			Method:  "getblockheader",
			Params:  []interface{}{hashResponse.Result},
		}
	}

	// Perform the batched `getblockheader` requests
	headerResponses := make([]types.RPCResponseHeader, len(heights))

	err = makeRPCRequest(batch, &headerResponses)
	if err != nil {
		return nil, fmt.Errorf("error fetching block headers: %v", err)
	}

	// Extract the headers from the responses
	for i, headerResponse := range headerResponses {
		if headerResponse.Error != "" {
			return nil, fmt.Errorf("error in header response: %v", headerResponse.Error)
		}
		headers[i] = types.BlockHeader{
			Hash:          headerResponse.Result.Hash,
			PrevBlockHash: headerResponse.Result.PreviousBlockHash,
			Timestamp:     headerResponse.Result.Timestamp,
			Height:        headerResponse.Result.Height,
		}
	}

	return headers, nil
}

func GetBlockchainInfo() (*types.BlockchainInfo, error) {
	rpcData := types.RPCRequest{
		JSONRPC: "1.0",
		ID:      "blindbit-silent-payment-backend-v0",
		Method:  "getblockchaininfo",
		Params:  []interface{}{},
	}

	var rpcResponse types.RPCResponseBlockchainInfo

	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		logging.L.Err(err).Msg("error getting blockchain info")
		return nil, err
	}

	if rpcResponse.Error != nil {
		err = fmt.Errorf("RPC Error: %v", rpcResponse.Error)
		logging.L.Err(err).Msg("RPC error")
		return nil, err
	}

	return &rpcResponse.Result, nil
}

func GetRawTransaction(
	txid string, blockhash ...string,
) (*types.Transaction, error) {
	var params = []any{txid, 2} // verbosity level 2 so we easily get the prevouts for tweaking
	if len(blockhash) > 0 {
		params = append(params, blockhash[0])
	}

	rpcData := types.RPCRequest{
		JSONRPC: "1.0",
		ID:      "blindbit-silent-payment-backend-v0",
		Method:  "getrawtransaction",
		Params:  params,
	}

	var rpcResponse types.RPCResponseGetRawTransaction

	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		logging.L.Err(err).Msg("rpc call failed")
		return nil, err
	}

	if rpcResponse.Error != nil {
		logging.L.Error().Msgf("RPC Error: %v\n", rpcResponse.Error)
		return nil, fmt.Errorf("RPC Error: %v", rpcResponse.Error)
	}

	return &rpcResponse.Result, nil
}
