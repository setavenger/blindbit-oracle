package tweak

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"SilentPaymentAppBackend/src/p2p"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"io/ioutil"
	"log"
	"net/http"
)

//const MempoolEndpoint = "https://mempool.space/api/tx/"

func StartFetchRoutine(foundTaprootTXChan chan chainhash.Hash, handler *p2p.PeerHandler) {
	log.Println("starting fetch routine")
	for {
		select {
		case txId := <-foundTaprootTXChan:
			log.Println("new txid:", txId.String())

			transactionDetails, err := getTransactionDetails(txId, handler)
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			//log.Printf("%+v\n", transactionDetails)
			// todo make a break to not query too much at once
			//<-time.After(100 * time.Millisecond)

			// these are the spent transaction outputs, they will be removed from the light utxo database
			// in order to keep the database lean and tht new client syncs don't have to see this data at all
			taprootSpent := extractSpentTaprootPubKeys(transactionDetails, handler)

			go func() {
				for _, spentUTXO := range taprootSpent {
					log.Printf("Deleting Output: %s:%d\n", spentUTXO.Txid, spentUTXO.Vout)
					mongodb.DeleteLightUTXOByTxIndex(spentUTXO.Txid, spentUTXO.Vout)
					mongodb.SaveSpentUTXO(&spentUTXO)
				}
			}()

			go mongodb.SaveTransactionDetails(transactionDetails)
			//log.Println("here")
			tweakData, err := ComputeTweak(transactionDetails)
			if err != nil {
				common.ErrorLogger.Println(err)
				continue
			}
			mongodb.SaveTweakData(tweakData)
		}
	}
}

func getTransactionDetails(txId chainhash.Hash, ph *p2p.PeerHandler) (*common.Transaction, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", common.MempoolEndpoint, txId.String()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Transaction was not found:", txId)
		return nil, fmt.Errorf("HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	var tx common.Transaction
	err = json.Unmarshal(body, &tx)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	bytes, err := hex.DecodeString(tx.Status.BlockHash)
	if err != nil {
		common.ErrorLogger.Println(err)
		return nil, err
	}

	hash := chainhash.Hash{}
	err = hash.SetBytes(bytes)
	if err != nil {
		common.ErrorLogger.Println(err)
		//panic(err)
		return nil, err
	}

	newHash := common.GetChainHash(hash)

	var blockHeight = ph.GetBlockHeightByHeader(newHash)
	if blockHeight == -1 {
		blockHeight = 0
	}

	tx.Status.BlockHeight = uint32(blockHeight)
	return &tx, nil
}
