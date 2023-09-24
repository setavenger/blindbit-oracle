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
	fmt.Println(cFilter)
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

	c.JSON(200, utxos)
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
	// Create a new request using http
	url := "http://localhost/api/tx"
	//data := "0200000001fd5b5fcd1cb066c27cfc9fda5428b9be850b81ac440ea51f1ddba2f987189ac1010000008a4730440220686a40e9d2dbffeab4ca1ff66341d06a17806767f12a1fc4f55740a7af24c6b5022049dd3c9a85ac6c51fecd5f4baff7782a518781bbdd94453c8383755e24ba755c01410436d554adf4a3eb03a317c77aa4020a7bba62999df633bba0ea8f83f48b9e01b0861d3b3c796840f982ee6b14c3c4b7ad04fcfcc3774f81bff9aaf52a15751fedfdffffff02416c00000000000017a914bc791b2afdfe1e1b5650864a9297b20d74c61f4787d71d0000000000001976a9140a59837ccd4df25adc31cdad39be6a8d97557ed688ac00000000"

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
