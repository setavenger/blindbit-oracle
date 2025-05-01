package main

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/core"
	"flag"
	"fmt"
)

func main() {
	txid := flag.String("txid", "", "give tx hex")
	blockhash := flag.String("blockhash", "", "blockhash might be needed if txindex is not enabled on the node")
	rpcUser := flag.String("rpc-user", "", "the nodes rpc user")
	rpcPass := flag.String("rpc-pass", "", "the nodes rpc password")
	rpcHost := flag.String("rpc-host", "", "the hostname (including port) of the bitcoin core node")

	flag.Parse()

	common.RpcUser = *rpcUser
	common.RpcPass = *rpcPass
	common.RpcEndpoint = *rpcHost

	var tx *types.Transaction
	var err error
	if blockhash != nil {
		tx, err = core.GetRawTransaction(*txid, *blockhash)
	} else {
		tx, err = core.GetRawTransaction(*txid)
	}
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	tweak, err := core.ComputeTweakPerTx(*tx)
	if err != nil {
		fmt.Println("err:", err)
		return
	}
	fmt.Printf("tweak: %x\n", tweak.TweakData)
}
