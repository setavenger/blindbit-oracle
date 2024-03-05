package server

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/db/mongodb"
	"bytes"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
)

// todo might not need ApiHandler struct if no data is stored within.
//  Will keep for now just in case, so I don't have to refactor twice
type ApiHandler struct{}

type TxRequest struct {
	Data string `form:"data" json:"data" binding:"required"`
}

func (h *ApiHandler) GetBestBlockHeight(c *gin.Context) {
	lastHeader, err := mongodb.RetrieveLastHeader()
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"block_height": lastHeader.Height,
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
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	cFilter, err := mongodb.RetrieveCFilterByHeight(uint32(height))
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}

	data := gin.H{
		"filter_type":  cFilter.FilterType,
		"block_height": cFilter.BlockHeight,
		"block_header": cFilter.BlockHeader,
		"data":         hex.EncodeToString(cFilter.Data),
	}

	c.JSON(200, data)
}

func (h *ApiHandler) GetCFilterByHeightTaproot(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	cFilter, err := mongodb.RetrieveCFilterByHeight(uint32(height))

	var data gin.H
	if cFilter != nil {
		data = gin.H{
			"filter_type":  cFilter.FilterType,
			"block_height": cFilter.BlockHeight,
			"block_header": cFilter.BlockHeader,
			"data":         hex.EncodeToString(cFilter.Data),
		}
	} else {
		data = gin.H{
			"filter_type":  "",
			"block_height": "",
			"block_header": "",
			"data":         "",
		}
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
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	utxos, err := mongodb.RetrieveLightUTXOsByHeight(uint32(height))
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
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
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}

	utxos, err := mongodb.RetrieveSpentUTXOsByHeight(uint32(height))
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
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
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "could not parse height",
		})
		return
	}
	tweakIndex, err := mongodb.RetrieveTweakIndexByHeight(uint32(height))
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	//serveTweakData := []string{}
	//for _, data := range tweakIndex.Data {
	//	serveTweakData = append(serveTweakData, hex.EncodeToString(data[:]))
	//}

	c.JSON(200, tweakIndex)
}

func (h *ApiHandler) ForwardRawTX(c *gin.Context) {
	var txRequest TxRequest
	if err := c.ShouldBind(&txRequest); err != nil {
		common.ErrorLogger.Println(err)
		c.Status(http.StatusBadRequest)
		return
	}
	err := forwardTxToMemPool(txRequest.Data)
	if err != nil {
		common.ErrorLogger.Println(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func forwardTxToMemPool(txHex string) error {
	//url := "http://localhost/api/tx"

	resp, err := http.Post(common.MempoolEndpoint, "application/x-www-form-urlencoded", bytes.NewBufferString(txHex))
	if err != nil {
		common.ErrorLogger.Printf("Failed to make request: %s\n", err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		common.ErrorLogger.Printf("Failed to read response: %s\n", err)
		return err
	}

	common.DebugLogger.Println("Response:", string(body))
	return nil
}
