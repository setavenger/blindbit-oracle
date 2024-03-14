package core

import (
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/testhelpers"
	"log"
	"testing"
)

/*
todo blocks to test
 000000000000000000030fcdb1ee03e49a5c50c0d457441a7bf4215920048824 ~4.8k txs
 000000000000000000027bd4698820dc77142b578a0bb824af9bdc799e731b85 ~5.2k txs
 000000000000000000028988a6b092b1bd1aa64211495e280ed274985fbfada5 ~6.1k txs
 00000000000000000000d1b78dabafed74c4483fdde4d899952274fafb70998c ~0.9k txs but 19k taproot UTXOs
*/
var (
	block833000, block833010, block833013, block834469 types.Block
)

func init() {
	err := testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833000.json", &block833000)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833010.json", &block833010)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_833013.json", &block833013)
	if err != nil {
		log.Fatalln(err)
	}
	err = testhelpers.LoadAndUnmarshalBlockFromFile("../test_data/block_834469.json", &block834469)
	if err != nil {
		log.Fatalln(err)
	}
}

func BenchmarkTweakV4Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV4(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV3Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV3(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833000)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV4Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV4(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV3Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV3(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833010(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833010)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV4Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV4(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV3Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV3(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block833013(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block833013)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV4Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV4(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV3Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV3(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV2Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV2(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func BenchmarkTweakV1Block834469(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ComputeTweaksForBlockV1(&block834469)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
