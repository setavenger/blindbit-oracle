package server

import (
	"SilentPaymentAppBackend/src/common"
	"SilentPaymentAppBackend/src/common/types"
	"SilentPaymentAppBackend/src/db/dblevel"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ApiHandler todo might not need ApiHandler struct if no data is stored within.
//
//	Will keep for now just in case, so I don't have to refactor twice
type ApiHandler struct{}

type Info struct {
	Network                        string `json:"network"`
	Height                         uint32 `json:"height"`
	TweaksOnly                     bool   `json:"tweaks_only"`
	TweaksFullBasic                bool   `json:"tweaks_full_basic"`
	TweaksFullWithDustFilter       bool   `json:"tweaks_full_with_dust_filter"`
	TweaksCutThroughWithDustFilter bool   `json:"tweaks_cut_through_with_dust_filter"`
}

func (h *ApiHandler) GetInfo(c *gin.Context) {
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, Info{
		Network:                        common.ChainToString(common.Chain),
		Height:                         lastHeader.Height,
		TweaksOnly:                     common.TweaksOnly,
		TweaksFullBasic:                common.TweakIndexFullNoDust,
		TweaksFullWithDustFilter:       common.TweakIndexFullIncludingDust,
		TweaksCutThroughWithDustFilter: common.TweaksCutThroughWithDust,
	})
}

func (h *ApiHandler) GetBestBlockHeight(c *gin.Context) {
	// todo returns one height too low
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
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
	headerInv, exists := c.Get("headerInv")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
		return
	}

	filterType := c.Param("type")

	var cFilter types.Filter
	var err error
	switch filterType {
	case "spent":
		cFilter, err = dblevel.FetchByBlockHashSpentOutpointsFilter(hInv.Hash)
		if err != nil {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not get filter from db",
			})
			return
		}
	case "new-utxos":
		cFilter, err = dblevel.FetchByBlockHashNewUTXOsFilter(hInv.Hash)
		if err != nil {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not get filter from db",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid filter type",
		})
		return
	}

	data := gin.H{
		"filter_type":  cFilter.FilterType,
		"block_height": hInv.Height,
		"block_hash":   cFilter.BlockHash,
		"data":         hex.EncodeToString(cFilter.Data),
	}

	c.JSON(200, data)
}

func (h *ApiHandler) GetUtxosByHeight(c *gin.Context) {
	headerInv, exists := c.Get("headerInv")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
		return
	}
	utxos, err := dblevel.FetchByBlockHashUTXOs(hInv.Hash)
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
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

// GetTweakDataByHeight serves tweak data as json array of tweaks (33 byte as hex-formatted)
// todo can be changed to serve with verbosity aka serve with txid or even block data (height, hash)
func (h *ApiHandler) GetTweakDataByHeight(c *gin.Context) {
	// todo outsource all the blockHeight extraction and conversion through the inverse header table into middleware
	headerInv, exists := c.Get("headerInv")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
		return
	}
	var tweaks []types.Tweak

	// Extracting the dustLimit query parameter and converting it to uint64
	dustLimitStr := c.DefaultQuery("dustLimit", "0") // Default to "0" if not provided
	dustLimit, err := strconv.ParseUint(dustLimitStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dustLimit parameter"})
		return
	}
	if dustLimit == 0 {
		// this query should have a better performance due to no required checks
		tweaks, err = dblevel.FetchByBlockHashTweaks(hInv.Hash)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}
	} else {
		tweaks, err = dblevel.FetchByBlockHashDustLimitTweaks(hInv.Hash, dustLimit)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}
	}

	if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
		c.JSON(http.StatusOK, []string{})
		return
	}

	var serveTweakData = []string{}
	for _, tweak := range tweaks {
		serveTweakData = append(serveTweakData, hex.EncodeToString(tweak.TweakData[:]))
	}

	c.JSON(http.StatusOK, serveTweakData)
}

func (h *ApiHandler) GetTweakIndexDataByHeight(c *gin.Context) {
	headerInv, exists := c.Get("headerInv")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
		return
	}

	// Extracting the dustLimit query parameter and converting it to uint64
	dustLimitStr := c.DefaultQuery("dustLimit", "0") // Default to "0" if not provided
	dustLimit, err := strconv.ParseUint(dustLimitStr, 10, 64)
	if err != nil {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dustLimit parameter"})
		return
	}

	if dustLimit != 0 && !common.TweakIndexFullIncludingDust {
		common.DebugLogger.Println("tried accessing dust limits")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server does not allow dustLimits"})
		return
	}

	// todo basically duplicate code could be simplified and generalised with interface/(generics?)
	if common.TweakIndexFullIncludingDust {
		var tweakIndex *types.TweakIndexDust
		tweakIndex, err = dblevel.FetchByBlockHashTweakIndexDust(hInv.Hash)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}

		if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			c.JSON(http.StatusOK, []string{})
			return
		}

		var serveTweakData = []string{}
		for _, tweak := range tweakIndex.Data {
			if tweak.HighestValue() < dustLimit {
				continue
			}
			data := tweak.Tweak()
			serveTweakData = append(serveTweakData, hex.EncodeToString(data[:]))
		}

		c.JSON(200, serveTweakData)
		return
	} else {
		// this query should have a better performance due to no required checks
		tweakIndex, err := dblevel.FetchByBlockHashTweakIndex(hInv.Hash)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			common.ErrorLogger.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}

		if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
			c.JSON(http.StatusOK, []string{})
			return
		}

		var serveTweakData = []string{}
		for _, tweak := range tweakIndex.Data {
			serveTweakData = append(serveTweakData, hex.EncodeToString(tweak[:]))
		}

		c.JSON(200, serveTweakData)
		return
	}
}

func (h *ApiHandler) GetSpentOutpointsIndex(c *gin.Context) {
	headerInv, exists := c.Get("headerInv")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
		return
	}
	spentOutpointsIndex, err := dblevel.FetchByBlockHashSpentOutpointIndex(hInv.Hash)
	if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
		common.ErrorLogger.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}

	if err != nil && errors.Is(err, dblevel.NoEntryErr{}) {
		c.JSON(http.StatusOK, []string{})
		return
	}

	var result struct {
		BlockHash string   `json:"block_hash"`
		Data      []string `json:"data"`
	}

	result.BlockHash = spentOutpointsIndex.BlockHash

	if len(spentOutpointsIndex.Data) == 0 {
		common.WarningLogger.Println("spentOutpointsIndex was empty")
		result.Data = []string{}
		c.JSON(http.StatusOK, result)
		return
	}

	resultData := make([]string, len(spentOutpointsIndex.Data))
	for i, hash := range spentOutpointsIndex.Data {
		resultData[i] = hex.EncodeToString(hash[:])
	}

	result.Data = resultData

	c.JSON(http.StatusOK, result)
}

type TxRequest struct {
	Data string `form:"data" json:"data" binding:"required"`
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		common.ErrorLogger.Printf("Failed to read response: %s\n", err)
		return err
	}

	common.DebugLogger.Println("Response:", string(body))
	return nil
}
