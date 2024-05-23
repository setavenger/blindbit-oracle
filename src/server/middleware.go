package server

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/dblevel"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		c.Abort()
		return
	}

	headerInv, err := dblevel.FetchByBlockHeightBlockHeaderInv(uint32(height))
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch header inventory"})
		c.Abort()
		return
	}

	// Store headerInv in Gin context
	c.Set("headerInv", headerInv)
	c.Next()
}
