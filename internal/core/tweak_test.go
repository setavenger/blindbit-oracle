package core

import (
	"encoding/hex"
	"testing"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/testhelpers"
	"github.com/setavenger/blindbit-oracle/internal/types"
	"github.com/setavenger/go-bip352"
)

var b833000 types.Block

func init() {
	err := testhelpers.LoadAndUnmarshalBlockFromFile("../../test_data/block_833000.json", &b833000)
	if err != nil {
		logging.L.Fatal().Err(err).Msg("error loading block 833000")
	}
}

// todo integrate the test vectors into the tests
func TestSimpleInputHash(t *testing.T) {
	const controlInputHash = "5bfe5321d759e01a2ac9292f0f396ff9c3d8b58d89ccb21a6922e84bb7ad0668"
	testCases, err := testhelpers.LoadCaseData(t)
	if err != nil {
		t.Error(err)
		return
	}

	tx, err := testhelpers.TransformTestCaseDetailToTransaction(testCases[0].Receiving[0]) // Example for the first sending case
	if err != nil {
		t.Error(err)
		return
	}

	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		t.Error("exit no pub keys found")
		return
	}

	fixSizePubKeys := utils.ConvertPubkeySliceToFixedLength33(pubKeys)

	summedKey, err := bip352.SumPublicKeys(fixSizePubKeys)
	if err != nil {
		t.Error(err)
		return
	}
	// logging.L.Debug().Hex("summed_key", summedKey[:]).Msg("summed key")
	inputHash, err := ComputeInputHash(tx, summedKey)
	if err != nil {
		t.Error(err)
		return
	}
	// logging.L.Debug().Hex("input_hash", inputHash[:]).Msg("input hash")
	inputHashHex := hex.EncodeToString(inputHash[:])
	if inputHashHex != controlInputHash {
		t.Errorf("computed input hash does not match: %s - %s\n", inputHashHex, controlInputHash)
		return
	}
	logging.L.Info().Msg("Done")
}

func TestComputeAllReceivingTweaks(t *testing.T) {
	testCases, err := testhelpers.LoadCaseData(t)
	if err != nil {
		t.Error(err)
		return
	}

	for _, testCase := range testCases {
		logging.L.Info().Msg(testCase.Comment)

		for _, caseDetail := range testCase.Receiving {
			tx, err := testhelpers.TransformTestCaseDetailToTransaction(caseDetail)
			if err != nil {
				t.Error(err)
				return
			}
			tweakPerTx, err := ComputeTweakPerTx(tx)
			if err != nil {
				t.Error(err)
				return
			}

			if testCase.Comment == "No valid inputs, sender generates no outputs" && tweakPerTx == nil {
				// this test case is supposed to be empty hence the exception
				continue
			}

			if caseDetail.Expected.Tweak != hex.EncodeToString(tweakPerTx.TweakData[:]) {
				t.Errorf("incorrect tweak: %s - %s", caseDetail.Expected.Tweak, hex.EncodeToString(tweakPerTx.TweakData[:]))
				return
			}
		}
	}

}

func TestBlockProcessingTime(t *testing.T) {
	logging.L.Info().Msg("Starting v3 computation")
	_, err := ComputeTweaksForBlockV3(&b833000)
	if err != nil {
		t.Error(err)
		return
	}
	logging.L.Info().Msg("Finished v3 computation")
	logging.L.Info().Msg("Starting v2 computation")
	_, err = ComputeTweaksForBlockV2(&b833000)
	if err != nil {
		t.Error(err)
		return
	}
	logging.L.Info().Msg("Finished v2 computation")
	logging.L.Info().Msg("Starting v1 computation")
	_, err = ComputeTweaksForBlockV1(&b833000)
	if err != nil {
		t.Error(err)
		return
	}
	logging.L.Info().Msg("Finished v1 computation")
}

func TestV3NoTxs(t *testing.T) {
	// if Txs field is nil
	block1 := types.Block{
		Hash:              "testHash",
		Height:            111,
		PreviousBlockHash: "testHashBefore",
		Timestamp:         1234,
		Txs:               nil,
	}
	_, err := ComputeTweaksForBlockV3(&block1)
	if err != nil {
		t.Error(err)
		return
	}

	// if Txs field is empty array
	block2 := types.Block{
		Hash:              "testHash",
		Height:            111,
		PreviousBlockHash: "testHashBefore",
		Timestamp:         1234,
		Txs:               []types.Transaction{},
	}
	_, err = ComputeTweaksForBlockV3(&block2)
	if err != nil {
		t.Error(err)
		return
	}

}

// TestAllTweakVersionsOutputs all tweak computations should match the computed length of tweaks from the V1
func TestAllTweakVersionsOutputs(t *testing.T) {
	compareForBlock(t, &block833000)
	compareForBlock(t, &block833010)
	compareForBlock(t, &block833013)
	compareForBlock(t, &block834469)
}

func compareForBlock(t *testing.T, block *types.Block) {
	tweaks4, err := ComputeTweaksForBlockV4(block)
	if err != nil {
		t.Error(err)
		return
	}
	tweaks3, err := ComputeTweaksForBlockV3(block)
	if err != nil {
		t.Error(err)
		return
	}
	tweaks2, err := ComputeTweaksForBlockV2(block)
	if err != nil {
		t.Error(err)
		return
	}
	tweaks1, err := ComputeTweaksForBlockV1(block)
	if err != nil {
		t.Error(err)
		return
	}

	if len(tweaks1) != len(tweaks2) {
		t.Errorf("block: %d tweak1 and tweak2 don't match: %d - %d\n", block.Height, len(tweaks1), len(tweaks2))
	}
	if len(tweaks1) != len(tweaks3) {
		t.Errorf("block: %d tweak1 and tweak3 don't match: %d - %d\n", block.Height, len(tweaks1), len(tweaks3))
	}
	if len(tweaks1) != len(tweaks4) {
		t.Errorf("block: %d tweak1 and tweak4 don't match: %d - %d\n", block.Height, len(tweaks1), len(tweaks4))
	}
}
