package server

import (
	"github.com/gin-gonic/gin"
	"log"
)

func RunServer(api *ApiHandler) {

	router := gin.Default()

	router.GET("/block-height", api.GetBestBlockHeight)
	router.GET("/tweak/:blockheight", api.GetTweakDataByHeight)
	router.GET("/filter/:blockheight", api.GetCFilterByHeight)
	router.GET("/utxos/:blockheight", api.GetLightUTXOsByHeight)
	router.GET("/utxos-spent/:blockheight", api.GetSpentUTXOsByHeight)

	router.POST("/forward-tx", api.ForwardRawTX)

	if err := router.Run(":8000"); err != nil {
		log.Fatal(err)
	}
}
