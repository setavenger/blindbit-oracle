package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// todo might need to unify common.types and the types here for consistency

func makeRPCRequest(rpcData interface{}, result interface{}) error {
	payload, err := json.Marshal(rpcData)
	if err != nil {
		return fmt.Errorf("error marshaling RPC data: %v", err)
	}

	// Prepare the request...
	req, err := http.NewRequest("POST", common.RpcEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers and auth...
	req.Header.Set("Content-Type", "application/json")
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", common.RpcUser, common.RpcPass)))
	req.Header.Add("Authorization", "Basic "+auth)

	// Make the HTTP request...
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		common.DebugLogger.Printf("response %+v\n", resp)
		return fmt.Errorf("error performing request: %v", err)
	}
	defer resp.Body.Close()

	// Read and unmarshal the response...
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		common.DebugLogger.Println("status code:", resp.Status)
		return fmt.Errorf("error reading response body: %v", err)
	}

	err = json.Unmarshal(body, result)
	if err != nil {
		common.DebugLogger.Println("status code:", resp.Status)
		common.DebugLogger.Println("data:", string(body))
		return fmt.Errorf("error unmarshaling response: %v", err)
	}

	return nil
}

func GetFullBlockPerBlockHash(blockHash string) (*types.Block, error) {
	//common.InfoLogger.Println("Fetching block:", blockHash)
	rpcData := types.RPCRequest{
		JSONRPC: "1.0",
		ID:      "blindbit-silent-payment-backend-v0",
		Method:  "getblock",
		Params:  []interface{}{blockHash, 3}, // 3 for maximum verbosity such that we easily get the prevouts for tweaking
	}

	var rpcResponse types.RPCResponseBlock
	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		common.ErrorLogger.Printf("%v\n", err)
		return nil, err
	}

	if rpcResponse.Error != nil {
		common.ErrorLogger.Printf("RPC Error: %v\n", rpcResponse.Error)
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

	var rpcResponse struct { // Anonymous struct for this specific response
		ID     string      `json:"id"`
		Result string      `json:"result,omitempty"`
		Error  interface{} `json:"error,omitempty"`
	}

	err := makeRPCRequest(rpcData, &rpcResponse)
	if err != nil {
		common.ErrorLogger.Printf("%v\n", err)
		return "", err
	}

	if rpcResponse.Error != nil {
		common.ErrorLogger.Printf("RPC Error: %v\n", rpcResponse.Error)
		return "", err
	}

	return rpcResponse.Result, nil
}

func GetBlockHeadersBatch(startHeight, count uint32) ([]types.BlockHeader, error) {
	// Prepare the batch request
	batch := make([]types.RPCRequest, count)
	headers := make([]types.BlockHeader, count)

	// Initialize the batch with `getblockhash` requests
	for i := uint32(0); i < count; i++ {
		batch[i] = types.RPCRequest{
			JSONRPC: "1.0",
			ID:      "blindbit-silent-payment-backend-v0",
			Method:  "getblockhash",
			Params:  []interface{}{startHeight + i},
		}
	}

	// Perform the batched `getblockhash` requests
	hashResponses := make([]struct {
		ID     string      `json:"id"`
		Result string      `json:"result,omitempty"`
		Error  interface{} `json:"error,omitempty"`
	}, count)

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
	headerResponses := make([]types.RPCResponseHeader, count)

	err = makeRPCRequest(batch, &headerResponses)
	if err != nil {
		return nil, fmt.Errorf("error fetching block headers: %v", err)
	}

	// Extract the headers from the responses
	for i, headerResponse := range headerResponses {
		if headerResponse.Error != nil {
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
		common.ErrorLogger.Printf("%v\n", err)
		return nil, err
	}

	if rpcResponse.Error != nil {
		common.ErrorLogger.Printf("RPC Error: %v\n", rpcResponse.Error)
		return nil, fmt.Errorf("RPC Error: %v", rpcResponse.Error)
	}

	return &rpcResponse.Result, nil
}
