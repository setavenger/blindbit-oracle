package core

import (
	"SilentPaymentAppBackend/src/common"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

func init() {
	err := os.Mkdir("./logs", 0750)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		fmt.Println(err.Error())
		log.Fatal(err)
	}

	file, err := os.OpenFile(fmt.Sprintf("./logs/logs-%s.txt", time.Now()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	multi := io.MultiWriter(file, os.Stdout)

	common.DebugLogger = log.New(multi, "[DEBUG] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.InfoLogger = log.New(multi, "[INFO] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.WarningLogger = log.New(multi, "[WARNING] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
	common.ErrorLogger = log.New(multi, "[ERROR] ", log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix)
}

// todo integrate the test vectors into the tests
func TestSimpleInputHash(t *testing.T) {
	const controlInputHash = "5bfe5321d759e01a2ac9292f0f396ff9c3d8b58d89ccb21a6922e84bb7ad0668"
	testCases, err := common.LoadCaseData(t)
	if err != nil {
		t.Error(err)
		return
	}

	tx, err := common.TransformTestCaseDetailToTransaction(testCases[0].Receiving[0]) // Example for the first sending case
	if err != nil {
		t.Error(err)
		return
	}

	pubKeys := extractPubKeys(&tx)
	if pubKeys == nil {
		t.Error("exit no pub keys found")
		return
	}

	summedKey, err := sumPublicKeys(pubKeys)
	if err != nil {
		t.Error(err)
		return
	}
	common.DebugLogger.Println(hex.EncodeToString(summedKey.SerializeCompressed()))
	inputHash, err := ComputeInputHash(&tx, summedKey)
	if err != nil {
		t.Error(err)
		return
	}
	common.DebugLogger.Println(hex.EncodeToString(inputHash[:]))
	inputHashHex := hex.EncodeToString(inputHash[:])
	if inputHashHex != controlInputHash {
		t.Errorf("computed input hash does not match: %s - %s\n", inputHashHex, controlInputHash)
		return
	}
	// At this point, `testCases` contains the data from your JSON file
	// You can now process it as needed
	common.InfoLogger.Println("Done")
}

func TestComputeAllReceivingTweaks(t *testing.T) {
	testCases, err := common.LoadCaseData(t)
	if err != nil {
		t.Error(err)
		return
	}

	for _, testCase := range testCases {
		common.InfoLogger.Println(testCase.Comment)
		if testCase.Comment == "Skip invalid P2SH inputs" {
			fmt.Println("pause")
		}
		for _, caseDetail := range testCase.Receiving {
			tx, err := common.TransformTestCaseDetailToTransaction(caseDetail)
			if err != nil {
				t.Error(err)
				return
			}
			tweakPerTx, err := ComputeTweakPerTx(&tx)
			if err != nil {
				t.Error(err)
				return
			}

			if testCase.Comment == "No valid inputs, sender generates no outputs" && tweakPerTx == nil {
				// this test case is supposed to be empty hence the exception
				continue
			}

			if caseDetail.Expected.Tweak != hex.EncodeToString(tweakPerTx[:]) {
				t.Errorf("incorrect tweak: %s - %s", caseDetail.Expected.Tweak, hex.EncodeToString(tweakPerTx[:]))
				return
			}
		}
	}

}
