package server

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

func RunServer(api *ApiHandler) {
	gin.SetMode(gin.ReleaseMode)

	// todo merge gin logging into blindbit lib logging
	router := gin.Default()
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		MaxAge:           12 * time.Hour,
		AllowCredentials: true,
	}))

	router.GET("/info", api.GetInfo)
	router.GET("/block-height", api.GetBestBlockHeight)
	router.GET("/block-hash/:blockheight", FetchHeaderInvMiddleware, api.GetBlockHashByHeight)
	router.GET("/tweaks/:blockheight", FetchHeaderInvMiddleware, api.GetTweakDataByHeight)
	router.GET("/tweak-index/:blockheight", FetchHeaderInvMiddleware, api.GetTweakIndexDataByHeight)
	router.GET("/filter/:type/:blockheight", FetchHeaderInvMiddleware, api.GetCFilterByHeight)
	router.GET("/utxos/:blockheight", FetchHeaderInvMiddleware, api.GetUtxosByHeight)
	router.GET("/spent-index/:blockheight", FetchHeaderInvMiddleware, api.GetSpentOutpointsIndex)

	router.POST("/forward-tx", api.ForwardRawTX)

	if err := router.Run(config.HTTPHost); err != nil {
		logging.L.Err(err).Msg("could not run server")
	}
}
