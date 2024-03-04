package core

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
)

func syncChain() {
	lastHeader, err := mongodb.RetrieveLastHeader()
	if err != nil {
		// fatal due to startup condition
		common.ErrorLogger.Fatalln(err)
		return
	}

}
