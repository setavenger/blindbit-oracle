package server

import (
	"SilentPaymentAppBackend/src/db/mongodb"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
)

type ApiHandler struct {
	BestHeight     uint32
	BestHeightChan chan uint32
}

type TxRequest struct {
	Data string `form:"data" json:"data" binding:"required"`
}

func (h *ApiHandler) HandleBestHeightUpdate() {
	for {
		select {
		case h.BestHeight = <-h.BestHeightChan:
			fmt.Println("new height", h.BestHeight)
		}
	}
}

func (h *ApiHandler) GetBestBlockHeight(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"block_height": h.BestHeight,
	})
}

func (h *ApiHandler) GetCFilterByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	cFilter := mongodb.RetrieveCFilterByHeight(uint32(height))
	fmt.Println("Filter:", strconv.FormatUint(height, 10), cFilter)
	data := gin.H{
		"filter_type":  cFilter.FilterType,
		"block_height": cFilter.BlockHeight,
		"block_header": cFilter.BlockHeader,
		"data":         hex.EncodeToString(cFilter.Data),
	}

	c.JSON(200, data)
}

func (h *ApiHandler) GetLightUTXOsByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	utxos := mongodb.RetrieveLightUTXOsByHeight(uint32(height))
	if utxos != nil {
		c.JSON(200, utxos)
	} else {
		c.JSON(200, []interface{}{})
	}
}

func (h *ApiHandler) GetSpentUTXOsByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}

	utxos := mongodb.RetrieveSpentUTXOsByHeight(uint32(height))
	if utxos != nil {
		c.JSON(200, utxos)
	} else {
		c.JSON(200, []interface{}{})
	}
}

func (h *ApiHandler) GetTweakDataByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	tweakData := mongodb.RetrieveTweakDataByHeight(uint32(height))
	serveTweakData := []string{}
	for _, data := range tweakData {
		serveTweakData = append(serveTweakData, hex.EncodeToString(data.Data[:]))
	}

	c.JSON(200, serveTweakData)
}

func (h *ApiHandler) ForwardRawTX(c *gin.Context) {
	var txRequest TxRequest
	if err := c.ShouldBind(&txRequest); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	err := forwardTxToMemPool(txRequest.Data)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func forwardTxToMemPool(txHex string) error {
	url := "http://localhost/api/tx"

	resp, err := http.Post(url, "application/x-www-form-urlencoded", bytes.NewBufferString(txHex))
	if err != nil {
		fmt.Printf("Failed to make request: %s\n", err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response: %s\n", err)
		return err
	}

	fmt.Println("Response:", string(body))
	return nil
}
