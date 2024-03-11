package testhelpers

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"
)

type TestCase struct {
	Comment   string           `json:"comment"`
	Sending   []TestCaseDetail `json:"sending"`
	Receiving []TestCaseDetail `json:"receiving"`
}

type TestCaseDetail struct {
	Given    TestCaseGiven    `json:"given"`
	Expected TestCaseExpected `json:"expected"`
}

type TestCaseGiven struct {
	Vin         []VinDetail     `json:"vin"`
	Recipients  [][]interface{} `json:"recipients"`
	Outputs     []string        `json:"outputs,omitempty"`      // For receiving
	KeyMaterial interface{}     `json:"key_material,omitempty"` // Placeholder for simplicity
}

type TestCaseExpected struct {
	Outputs   interface{} `json:"outputs,omitempty"`   // Flexible for both sending and receiving cases
	Addresses []string    `json:"addresses,omitempty"` // For receiving
	MoreData  interface{} `json:"more_data,omitempty"` // Placeholder for simplicity
	Tweak     string      `json:"tweak"`
}

type VinDetail struct {
	Txid        string        `json:"txid"`
	Vout        uint32        `json:"vout"`
	ScriptSig   string        `json:"scriptSig"`
	Txinwitness string        `json:"txinwitness,omitempty"`
	Prevout     PrevoutDetail `json:"prevout"`
}

type PrevoutDetail struct {
	ScriptPubKey ScriptPubKeyDetail `json:"scriptPubKey"`
}

type ScriptPubKeyDetail struct {
	Hex  string `json:"hex"`
	Type string `json:"type"`
}

func TransformTestCaseDetailToTransaction(detail TestCaseDetail) (types.Transaction, error) {
	transaction := types.Transaction{
		// Initialize other necessary fields of Transaction if needed
	}
	// Iterate over each VinDetail in the Given part of TestCaseDetail
	for _, vinDetail := range detail.Given.Vin {
		witnessScript, err := parseWitnessScript(vinDetail.Txinwitness)
		if err != nil {
			common.ErrorLogger.Println(err)
			return types.Transaction{}, err
		}
		vin := types.Vin{
			Txinwitness: witnessScript, // txinwitness needs to be parsed due to different witness lengths and format
			Txid:        vinDetail.Txid,
			Vout:        vinDetail.Vout,
			Prevout: &types.Prevout{
				ScriptPubKey: types.ScriptPubKey{
					Hex:  vinDetail.Prevout.ScriptPubKey.Hex,
					Type: vinDetail.Prevout.ScriptPubKey.Type,
				},
			},
			ScriptSig: types.ScriptSig{
				Hex: vinDetail.ScriptSig,
			},
			// Initialize other necessary fields of Vin if needed
		}

		// Append the constructed Vin to the Transaction's Vin slice
		transaction.Vin = append(transaction.Vin, vin)
	}

	return transaction, nil
}

func LoadCaseData(t *testing.T) ([]TestCase, error) {
	filePath := "../test_data/send_and_receive_test_vectors_with_type.json"

	// Read the JSON file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Errorf("Error reading JSON file: %s", err)
		return nil, err
	}

	// Assuming `testCases` is the variable for storing the unmarshaled data
	var testCases []TestCase

	// Unmarshal the JSON data into the struct
	err = json.Unmarshal(data, &testCases)
	if err != nil {
		t.Errorf("Error unmarshaling JSON: %s", err)
		return nil, err
	}

	return testCases, err
}

// parseWitnessScript parses a hex-encoded witness script and returns the actual witness data as a list,
// assuming the first byte indicates the number of items.
func parseWitnessScript(script string) ([]string, error) {
	// Decode the hex-encoded script
	data, err := hex.DecodeString(script)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	if len(data) == 0 {
		return []string{}, err
	}

	// The first byte indicates the number of items in the witness data
	itemCount := int(data[0])
	var witnessData []string
	i := 1 // Start index after the item count byte

	for j := 0; j < itemCount && i < len(data); j++ {
		if i >= len(data) {
			return nil, fmt.Errorf("script is shorter than expected")
		}

		// The first byte of each item indicates its length
		length := int(data[i])
		i++

		// Extract the witness data item based on the length
		if i+length > len(data) {
			return nil, fmt.Errorf("invalid length for witness data item")
		}
		item := data[i : i+length]

		// Append the hex-encoded item to the result list
		witnessData = append(witnessData, hex.EncodeToString(item))
		i += length
	}

	if len(witnessData) != itemCount {
		return nil, fmt.Errorf("actual item count does not match the expected count")
	}

	return witnessData, nil
}
