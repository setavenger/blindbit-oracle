package server

import (
	"SilentPaymentAppBackend/src/common"
	"github.com/gin-gonic/gin"
)

func RunServer(api *ApiHandler) {
	// todo merge gin logging into common logging
	router := gin.Default()

	router.GET("/block-height", api.GetBestBlockHeight)
	router.GET("/tweak/:blockheight", api.GetTweakDataByHeight)
	router.GET("/filter/:blockheight", api.GetCFilterByHeight)
	router.GET("/utxos/:blockheight", api.GetLightUTXOsByHeight)

	router.POST("/forward-tx", api.ForwardRawTX)

	if err := router.Run(":8000"); err != nil {
		common.ErrorLogger.Fatal(err)
	}
}
