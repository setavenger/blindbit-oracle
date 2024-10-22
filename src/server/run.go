package server

import (
	"SilentPaymentAppBackend/src/common"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RunServer(api *ApiHandler) {
	// todo merge gin logging into common logging
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.GET("/block-height", api.GetBestBlockHeight)
	router.GET("/tweaks/:blockheight", FetchHeaderInvMiddleware, api.GetTweakDataByHeight)
	router.GET("/tweak-index/:blockheight", FetchHeaderInvMiddleware, api.GetTweakIndexDataByHeight)
	router.GET("/filter/:type/:blockheight", FetchHeaderInvMiddleware, api.GetCFilterByHeight)
	router.GET("/utxos/:blockheight", FetchHeaderInvMiddleware, api.GetUtxosByHeight)
	router.GET("/spent-index/:blockheight", FetchHeaderInvMiddleware, api.GetSpentOutpointsIndex)

	router.POST("/forward-tx", api.ForwardRawTX)

	if err := router.Run(common.Host); err != nil {
		common.ErrorLogger.Fatal(err)
	}
}
