package core

import (
	"SilentPaymentAppBackend/src/common"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/btcec/v2"
	"testing"
)

// todo integrate the test vectors into the tests
func TestSimpleOutpointsHash(t *testing.T) {
	//
	//tx := &common.Transaction{
	//	Vin: []common.Vin{
	//		{
	//			Txid: "f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16",
	//			Vout: 0,
	//		},
	//		{
	//			Txid: "a1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d",
	//			Vout: 0,
	//		},
	//	},
	//}
	tx := &common.Transaction{
		Vin: []common.Vin{
			{
				Txid: "899f243469b7feec4f3b3847d6caebd9ef800730f42e9021ecb7ab5bf57ca879",
				Vout: 0,
			},
			{
				Txid: "a1075db55d416d3ca199f55b6084e2115b9345e16c5cf302fc80e9d5fbf5d48d",
				Vout: 0,
			},
		},
	}
	const resultHash = "210fef5d624db17c965c7597e2c6c9f60ef440c831d149c43567c50158557f12"

	hash, err := computeOutpointsHash(tx)
	if err != nil {
		t.Error("couldn't compute outpoints hash")
	}
	fmt.Println(hex.EncodeToString(hash[:]))

	if resultHash != hex.EncodeToString(hash[:]) {
		t.Errorf("Expected: %s got %s", resultHash, hex.EncodeToString(hash[:]))
	}
}

func TestSumPublicKeys(t *testing.T) {
	//givenPubKeys := []string{"5a1e61f898173040e20616d43e9f496fba90338a39faa1ed98fcbaeee4dd9be5", "bd85685d03d111699b15d046319febe77f8de5286e9e512703cdee1bf3be3792"}
	givenPubKeys := []string{"025a1e61f898173040e20616d43e9f496fba90338a39faa1ed98fcbaeee4dd9be5", "03bd85685d03d111699b15d046319febe77f8de5286e9e512703cdee1bf3be3792"}
	const expectedASum = "2562c1ab2d6bd45d7ca4d78f569999e5333dffd3ac5263924fd00d00dedc4bee"

	ASum, err := sumPublicKeys(givenPubKeys)
	if err != nil {
		t.Error("couldn't compute sum of public keys")
	}
	pubKeyAsString := fmt.Sprintf("%x", ASum.X())
	if pubKeyAsString != expectedASum {
		t.Errorf("couldn't compute sum of public keys: got %s and expected %s", pubKeyAsString, expectedASum)
	}
}

func TestSumPublicKeys2(t *testing.T) {
	givenPubKeys := []string{"025f1c13171d2e103b4384db08c926877420d2c402a1d32cfc0e2c3d484f656b8d", "034a2e397dbcd61d087dc98d0c6916f43c0c7291dbf050621b5a19003963eccd64"}
	const expectedASum = "1bef59e477d93a715f8e27c9ee495c34f6bf9bccc0f74f12144dbd93c27c3703"

	ASum, err := sumPublicKeys(givenPubKeys)
	if err != nil {
		t.Error("couldn't compute sum of public keys")
	}
	pubKeyAsString := fmt.Sprintf("%x", ASum.X())
	if pubKeyAsString != expectedASum {
		t.Errorf("couldn't compute sum of public keys: got %s and expected %s", pubKeyAsString, expectedASum)
	}
}

func TestTweakComputation(t *testing.T) {
	//givenPubKeys := []string{"5a1e61f898173040e20616d43e9f496fba90338a39faa1ed98fcbaeee4dd9be5", "bd85685d03d111699b15d046319febe77f8de5286e9e512703cdee1bf3be3792"}
	givenPubKeys := []string{"025a1e61f898173040e20616d43e9f496fba90338a39faa1ed98fcbaeee4dd9be5", "03bd85685d03d111699b15d046319febe77f8de5286e9e512703cdee1bf3be3792"}
	outpointshash, err := hex.DecodeString("210fef5d624db17c965c7597e2c6c9f60ef440c831d149c43567c50158557f12")
	if err != nil {
		t.Error("couldn't get bytes from outpoints hex")

	}
	const tweakResult = "997bf08b9516cd997f46957d923b1a20b25a5050bb8d98373b385909f01e296a"
	tweakResultBytes, err := hex.DecodeString(tweakResult)
	if err != nil {
		t.Error("error:", err)
	}
	ASum, err := sumPublicKeys(givenPubKeys)
	if err != nil {
		t.Error("couldn't compute sum of public keys")
	}

	// todo integrate into the actual function
	curve := btcec.KoblitzCurve{}

	x, y := curve.ScalarMult(ASum.X(), ASum.Y(), outpointshash[:])

	decodeString, err := hex.DecodeString(fmt.Sprintf("04%x%x", x, y))
	if err != nil {
		t.Error("error:", err)
	}

	tweakAsKey, err := btcec.ParsePubKey(decodeString)
	if err != nil {
		t.Error("error:", err)
	}

	tweakBytes := [32]byte{}
	copy(tweakBytes[:], tweakAsKey.SerializeCompressed()[1:])

	if bytes.Compare(tweakBytes[:], tweakResultBytes) != 0 {
		t.Errorf("Test Failed: expected: %x got %x", tweakResultBytes, tweakBytes)
	}
}

func TestTweakComputation2(t *testing.T) {
	const tweakResult = "9422fc9495927e74537ada465a47c93462f1597341a2fc8c0a65f516562a3498"

	givenPubKeys := []string{"025f1c13171d2e103b4384db08c926877420d2c402a1d32cfc0e2c3d484f656b8d", "034a2e397dbcd61d087dc98d0c6916f43c0c7291dbf050621b5a19003963eccd64"}
	tx := &common.Transaction{
		Vin: []common.Vin{
			{
				Txid: "b584654abca292095847aa181e822a77b1f89c4f08744bd2424f17e1446ca1f4",
				Vout: 0,
			},
			{
				Txid: "2333e16cfa10b50b48b94931d3a9154ffcb5033b4bb7faf153c8aebe093dc68a",
				Vout: 1,
			},
		},
	}
	outpointsHash, err := computeOutpointsHash(tx)
	if err != nil {
		t.Error("couldn't compute outpoints hash")
	}
	fmt.Printf("%x\n", outpointsHash)

	tweakResultBytes, err := hex.DecodeString(tweakResult)
	if err != nil {
		t.Error("error:", err)
	}
	ASum, err := sumPublicKeys(givenPubKeys)
	if err != nil {
		t.Error("couldn't compute sum of public keys")
	}

	// todo integrate into the actual function
	curve := btcec.KoblitzCurve{}

	x, y := curve.ScalarMult(ASum.X(), ASum.Y(), outpointsHash[:])

	decodeString, err := hex.DecodeString(fmt.Sprintf("04%x%x", x, y))
	if err != nil {
		t.Error("error:", err)
	}

	tweakAsKey, err := btcec.ParsePubKey(decodeString)
	if err != nil {
		t.Error("error:", err)
	}

	tweakBytes := [32]byte{}
	copy(tweakBytes[:], tweakAsKey.SerializeCompressed()[1:])

	if bytes.Compare(tweakBytes[:], tweakResultBytes) != 0 {
		t.Errorf("Test Failed: expected: %x got %x", tweakResultBytes, tweakBytes)
	}
}

func TestFullTweak(t *testing.T) {
	const ecdhSecret = "7001378204742ee5daa5435b69c0afe87072ba8038dc443387d3412591ee773e"

	tx := &common.Transaction{
		Txid:    "e73f1879a368043f50d09d279767c6b260dce81a3504f74d28a7ff3775aa47f1",
		Version: 2,
		Vin: []common.Vin{
			{
				IsCoinbase: false,
				Prevout: common.Prevout{
					Value:            100_000_000,
					Scriptpubkey:     "00149cb806c0f7c9b7e30edbefc6030b112c7cb78174",
					ScriptpubkeyType: "v0_p2wpkh",
				},
				Scriptsig: "",
				Txid:      "8db5f363b655a6778ba898e090783ceda27285779d04af09739890e029c5b746",
				Vout:      1,
				Witness: []string{
					"3045022100f31031f0eff9af18f36681bd5af34ab72caaa4dae730b8330b8a2efab348bf3902200d91895b2be6264de0f3c7272e32b34ac0481b306b8bf9d3b6741f890fd6a9c301",
					"0243c31d38ca6a7fbf17d21887854d2d9ea77b561cd3079e1ad590ae96b242aee6",
				},
				InnerRedeemscriptAsm:  "",
				InnerWitnessscriptAsm: "",
			},
		},
		Vout:   nil,
		Status: common.TransactionStatus{},
	}

	tweak, err := ComputeTweakPerTx(tx)
	if err != nil {
		t.Error("error:", err)
	}

	curve := btcec.KoblitzCurve{}

	publicKey, err := btcec.ParsePubKey(append([]byte{0x02}, tweak.Data[:]...))
	if err != nil {
		t.Error("error:", err)
	}

	pubKeys := extractPubKeys(tx)
	if pubKeys == nil {
		t.Error("error:", err)
	}
	key, err := sumPublicKeys(pubKeys)
	// scan key private
	bytesScan, err := hex.DecodeString("78e7fd7d2b7a2c1456709d147021a122d2dccaafeada040cc1002083e2833b09")
	if err != nil {
		t.Error("error:", err)
	}
	x, _ := curve.ScalarMult(key.X(), key.Y(), bytesScan[:])
	fmt.Printf("%x\n", x)

	x, _ = curve.ScalarMult(publicKey.X(), publicKey.Y(), bytesScan[:])
	fmt.Printf("%x\n", x)

	if ecdhSecret != fmt.Sprintf("%x", x) {
		t.Errorf("expected: %s received: %s", ecdhSecret, fmt.Sprintf("%x", x))
	}
}

func TestFullTweak2(t *testing.T) {
	const tweakResult = "baab42fe13ab4e6540da2bc8668165a6e3d0bddb646bb326870782720a6493f5"

	tx := &common.Transaction{
		Txid:    "e73f1879a368043f50d09d279767c6b260dce81a3504f74d28a7ff3775aa47f1",
		Version: 2,
		Vin: []common.Vin{
			{
				Prevout: common.Prevout{
					ScriptpubkeyType: "v0_p2wpkh",
				},
				Txid: "ab6c1ad9b076e202cd1a1aec34fdf0cfccbd6cae6e98d4c5c95b3b7cb227f3b2",
				Vout: 1,
				Witness: []string{
					"304402206f761f865d637e537ac8f6397a12d0f5f21342a7bf05f3a0b33e4f7d6aef23ec02206289dbe24e4955a406a093fa279c72f58849fb4946afc7b3f77e759331d3536101",
					"034a2e397dbcd61d087dc98d0c6916f43c0c7291dbf050621b5a19003963eccd64",
				},
			},
			{
				Prevout: common.Prevout{
					ScriptpubkeyType: "v0_p2wpkh",
				},
				Txid: "03c1d061a8aa13f696e5145590b43ef5575f369e3f10d0bdc9ad26b04f38a80c",
				Vout: 0,
				Witness: []string{
					"304402201f4b52e977e3f531576de3ae6febaf72bdc79522c314b67cc6fa5cf58a40337802205fcf4feb6999c1bc5eb3047b624877922d1b202d51cfedb032866563438bb97b01",
					"025f1c13171d2e103b4384db08c926877420d2c402a1d32cfc0e2c3d484f656b8d",
				},
			},
		},
		Vout:   nil,
		Status: common.TransactionStatus{},
	}

	tweak, err := ComputeTweakPerTx(tx)
	if err != nil {
		t.Error("error:", err)
	}

	if tweakResult != hex.EncodeToString(tweak.Data[:]) {
		t.Errorf("expected: %s received: %s", tweakResult, hex.EncodeToString(tweak.Data[:]))

	}
}
