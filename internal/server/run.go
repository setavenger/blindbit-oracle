package server

import (
	"errors"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

func RunServer(handler *Handler) {
	if handler.db == nil {
		err := errors.New("db of handler was nil")
		logging.L.Panic().Err(err).Msg("missing db in handler")
	}
	gin.SetMode(gin.ReleaseMode)

	// todo merge gin logging into blindbit lib logging
	router := gin.Default()
	// router.Use(gin.Recovery())
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		MaxAge:           12 * time.Hour,
		AllowCredentials: true,
	}))

	// New API endpoints following README specification
	router.GET("/tweaks/:blockheight", handler.GetTweaks)
	router.GET("/utxos/:blockheight", handler.GetUtxos)
	router.GET("/spent-outputs/:blockheight", handler.GetSpentOutputs) // todo: do we really need this?
	router.GET("/compute-index/:blockheight", handler.GetComputeIndex)
	router.GET("/full-block/:blockheight", handler.GetFullBlock)

	if err := router.Run(config.HTTPHost); err != nil {
		logging.L.Err(err).Msg("could not run server")
	}
}
