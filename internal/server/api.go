package server

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/setavenger/blindbit-lib/api"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/dblevel"
	"github.com/setavenger/blindbit-oracle/internal/types"
)

// ApiHandler todo might not need ApiHandler struct if no data is stored within.
//
//	Will keep for now just in case, so I don't have to refactor twice
type ApiHandler struct{}

func (h *ApiHandler) GetInfo(c *gin.Context) {
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
	if err != nil {
		logging.L.Err(err).Msg("error fetching highest block header inv")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, api.InfoResponseOracle{
		Network:                        config.ChainToString(config.Chain),
		Height:                         lastHeader.Height,
		TweaksOnly:                     config.TweaksOnly,
		TweaksFullBasic:                config.TweakIndexFullNoDust,
		TweaksFullWithDustFilter:       config.TweakIndexFullIncludingDust,
		TweaksCutThroughWithDustFilter: config.TweaksCutThroughWithDust,
	})
}

func (h *ApiHandler) GetBestBlockHeight(c *gin.Context) {
	// todo returns one height too low
	lastHeader, err := dblevel.FetchHighestBlockHeaderInvByFlag(true)
	if err != nil {
		logging.L.Err(err).Msg("error fetching highest block header inv")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	c.JSON(http.StatusOK, api.BlockHeightResponseOracle{
		BlockHeight: lastHeader.Height,
	})
}

func (h *ApiHandler) GetBlockHashByHeight(c *gin.Context) {
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

	c.JSON(http.StatusOK, api.BlockHashResponseOracle{
		BlockHash: hInv.Hash,
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
			logging.L.Err(err).Msg("error fetching spent outpoints filter")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not get filter from db",
			})
			return
		}
	case "new-utxos":
		cFilter, err = dblevel.FetchByBlockHashNewUTXOsFilter(hInv.Hash)
		if err != nil {
			logging.L.Err(err).Msg("error fetching new utxos filter")
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

	data := api.FilterResponseOracle{
		FilterType:  cFilter.FilterType,
		BlockHeight: hInv.Height,
		BlockHash:   cFilter.BlockHash,
		Data:        hex.EncodeToString(cFilter.Data),
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
		logging.L.Err(err).Msg("error fetching utxos")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could could not retrieve data from database",
		})
		return
	}
	if utxos != nil {
		c.JSON(200, utxos)
	} else {
		c.JSON(200, []any{})
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
			logging.L.Err(err).Msg("error fetching tweaks")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "could could not retrieve data from database",
			})
			return
		}
	} else {
		tweaks, err = dblevel.FetchByBlockHashDustLimitTweaks(hInv.Hash, dustLimit)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			logging.L.Err(err).Msg("error fetching dust limit tweaks")
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
		logging.L.Error().Msg("headerInv not found")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "headerInv not found"})
		return
	}
	hInv, ok := headerInv.(types.BlockHeaderInv) // Assuming HeaderInventory is the expected type
	if !ok {
		logging.L.Error().Msg("invalid headerInv type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid headerInv type"})
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

	// todo basically duplicate code could be simplified and generalised with interface/(generics?)
	if config.TweakIndexFullIncludingDust {
		var tweakIndex *types.TweakIndexDust
		tweakIndex, err = dblevel.FetchByBlockHashTweakIndexDust(hInv.Hash)
		if err != nil && !errors.Is(err, dblevel.NoEntryErr{}) {
			logging.L.Err(err).Msg("error fetching tweak index dust")
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
			logging.L.Err(err).Msg("error fetching tweak index")
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
		logging.L.Err(err).Msg("error fetching spent outpoints index")
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
		logging.L.Debug().Msg("spentOutpointsIndex was empty")
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
