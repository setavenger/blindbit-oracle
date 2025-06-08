package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
)

func FetchHeaderInvMiddleware(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		c.Abort()
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		c.Abort()
		return
	}

	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch header inv")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch header inv"})
		c.Abort()
		return
	}

	// Store headerInv in Gin context
	c.Set("headerInv", headerInv)
	c.Next()
}
