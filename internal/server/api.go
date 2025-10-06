// Package server
//
// DEPRECATED: This file contains the old API implementation and is no longer used.
// All endpoints have been moved to handler.go which follows the new specification
// defined in README.md. This file is kept for reference but should not be used
// in new code.
//
// Use the new Handler struct and its methods instead:
// - GetTweaks
// - GetUtxos
// - GetSpentOutputs
// - GetComputeIndex
// - GetFullBlock
package server

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/setavenger/blindbit-lib/api"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// ApiHandler todo might not need ApiHandler struct if no data is stored within.
//
// Will keep for now just in case, so I don't have to refactor twice
type ApiHandler struct {
	db database.DB
}

// NewApiHandler creates a new ApiHandler instance
func NewApiHandler(db database.DB) *ApiHandler {
	return &ApiHandler{
		db: db,
	}
}

func (h *ApiHandler) GetInfo(c *gin.Context) {
	_, height, err := h.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("error fetching chain tip")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, api.InfoResponseOracle{
		Network:                        config.ChainToString(config.Chain),
		Height:                         height,
		TweaksOnly:                     config.TweaksOnly,
		TweaksFullBasic:                config.TweakIndexFullNoDust,
		TweaksFullWithDustFilter:       config.TweakIndexFullIncludingDust,
		TweaksCutThroughWithDustFilter: config.TweaksCutThroughWithDust,
	})
}

func (h *ApiHandler) GetBestBlockHeight(c *gin.Context) {
	_, height, err := h.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("error fetching chain tip")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, api.BlockHeightResponseOracle{
		BlockHeight: height,
	})
}

func (h *ApiHandler) GetBlockHashByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	c.JSON(http.StatusOK, api.BlockHashResponseOracle{
		BlockHash: hex.EncodeToString(utils.ReverseBytesCopy(blockhash)),
	})
}

func (h *ApiHandler) GetCFilterByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	filterType := c.Param("type")

	switch filterType {
	case "spent":
		// TODO: Spent outpoints filter not available in pebbledb interface
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "spent outpoints filter not implemented in pebbledb interface",
		})
		return
	case "new-utxos":
		// Use TaprootUnspentFilter as a substitute for new-utxos filter
		filterData, err := h.db.FetchTaprootUnspentFilter(blockhash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching taproot unspent filter")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not get filter from db",
			})
			return
		}

		data := api.FilterResponseOracle{
			FilterType:  1, // Assuming 1 for "new-utxos" filter type
			BlockHeight: uint32(height),
			BlockHash:   hex.EncodeToString(blockhash),
			Data:        hex.EncodeToString(filterData),
		}

		c.JSON(200, data)
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid filter type",
		})
		return
	}
}

func (h *ApiHandler) GetUtxosByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	// Get chain tip for FetchOutputsAll
	_, syncTip, err := h.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("could not fetch chain tip")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch chain tip"})
		return
	}

	outputs, err := h.db.FetchOutputsAll(blockhash, syncTip)
	if err != nil {
		logging.L.Err(err).Msg("error fetching outputs")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}

	if outputs != nil {
		c.JSON(200, outputs)
	} else {
		c.JSON(200, []any{})
	}
}

// GetTweakDataByHeight serves tweak data as json array of tweaks (33 byte as hex-formatted)
// todo can be changed to serve with verbosity aka serve with txid or even block data (height, hash)
func (h *ApiHandler) GetTweakDataByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	// Extracting the dustLimit query parameter and converting it to uint64
	dustLimitStr := c.DefaultQuery("dustLimit", "0") // Default to "0" if not provided

	var tweaks [][]byte
	if dustLimitStr == "0" {
		fmt.Printf("%x\n", blockhash)
		// Use static tweaks for better performance
		tweaks, err = h.db.FetchTweaksStatic(blockhash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching static tweaks")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}
	} else {
		// moved conversion inside to optimise performance of above query
		dustLimit, err := strconv.ParseUint(dustLimitStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dustLimit parameter"})
			return
		}

		// Get chain tip for TweaksForBlockCutThroughDustLimit
		_, _, err = h.db.GetChainTip()
		if err != nil {
			logging.L.Err(err).Msg("could not fetch chain tip")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch chain tip"})
			return
		}

		// TODO: Need to implement TweaksForBlockCutThroughDustLimit in pebbledb interface
		// For now, fall back to static tweaks
		tweaks, err = h.db.FetchTweaksStatic(blockhash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching static tweaks")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}

		// TODO: Apply dust limit filtering here
		_ = dustLimit // Suppress unused variable warning
	}

	var serveTweakData = []string{}
	for _, tweak := range tweaks {
		serveTweakData = append(serveTweakData, hex.EncodeToString(tweak))
	}

	c.JSON(http.StatusOK, serveTweakData)
}

func (h *ApiHandler) GetTweakIndexDataByHeight(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	// Extracting the dustLimit query parameter and converting it to uint64
	dustLimitStr := c.DefaultQuery("dustLimit", "0") // Default to "0" if not provided
	dustLimit, err := strconv.ParseUint(dustLimitStr, 10, 64)
	if err != nil {
		logging.L.Err(err).Msg("error parsing dust limit")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid dustLimit parameter"})
		return
	}

	if dustLimit != 0 && !config.TweakIndexFullIncludingDust {
		logging.L.Debug().Msg("tried accessing dust limits")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server does not allow dustLimits"})
		return
	}

	// Use static tweaks for both cases since pebbledb interface doesn't have separate dust index
	tweaks, err := h.db.FetchTweaksStatic(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching static tweaks")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}

	var serveTweakData = []string{}
	for _, tweak := range tweaks {
		// TODO: Apply dust limit filtering if needed
		// For now, just return all tweaks
		serveTweakData = append(serveTweakData, hex.EncodeToString(tweak))
	}

	c.JSON(200, serveTweakData)
}

func (h *ApiHandler) GetSpentOutpointsIndex(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "block height is required"})
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not parse block height"})
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not fetch block hash"})
		return
	}

	// TODO: Spent outpoints index not available in pebbledb interface
	_ = blockhash // Suppress unused variable warning
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "spent outpoints index not implemented in pebbledb interface",
	})
}

type TxRequest struct {
	Data string `form:"data" json:"data" binding:"required"`
}

func (h *ApiHandler) ForwardRawTX(c *gin.Context) {
	var txRequest TxRequest
	if err := c.ShouldBind(&txRequest); err != nil {
		logging.L.Err(err).Msg("error binding tx request")
		c.Status(http.StatusBadRequest)
		return
	}
	err := forwardTxToMemPool(txRequest.Data)
	if err != nil {
		logging.L.Err(err).Msg("error forwarding tx to mempool")
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func forwardTxToMemPool(txHex string) error {
	var url string

	switch config.Chain {
	case config.Mainnet:
		url = config.MempoolEndpointMainnet
	case config.Testnet3:
		url = config.MempoolEndpointTestnet3
	case config.Signet:
		url = config.MempoolEndpointSignet
	default:
		return errors.New("invalid chain")
	}

	resp, err := http.Post(url, "application/x-www-form-urlencoded", bytes.NewBufferString(txHex))
	if err != nil {
		logging.L.Err(err).Msg("error forwarding tx to mempool")
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.L.Err(err).Msg("error reading response")
		return err
	}

	logging.L.Debug().Msgf("Response: %s", string(body))
	return nil
}
