package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/setavenger/blindbit-lib/api"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-lib/utils"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

type Handler struct {
	db database.DB
}

func NewHandler(db database.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) GetInfo(c *gin.Context) {
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

func (h *Handler) GetBestBlockHeight(c *gin.Context) {
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

func (h *Handler) GetBlockHashByHeight(c *gin.Context) {
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

// GetUtxos returns UTXO information for a specific block
func (h *Handler) GetUtxos(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("block height is required")))
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("could not parse block height")))
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch block hash")))
		return
	}

	// Get chain tip for FetchOutputsAll
	_, syncTip, err := h.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("could not fetch chain tip")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch chain tip")))
		return
	}

	outputs, err := h.db.FetchOutputsAll(blockhash, syncTip)
	if err != nil {
		logging.L.Err(err).Msg("error fetching outputs")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve data from database")))
		return
	}

	// Convert outputs to the expected format
	var utxoItems []UTXOItem
	for _, output := range outputs {
		if output != nil {
			var pubkey [32]byte
			copy(pubkey[:], output.Pubkey)

			utxoItems = append(utxoItems, UTXOItem{
				TxId:   [32]byte(utils.ReverseBytesCopy(output.Txid)),
				Vout:   output.Vout,
				Amount: output.Amount,
				Pubkey: pubkey,
			})
		}
	}

	response := struct {
		BlockIdentifier BlockIdentifier `json:"block_identifier"`
		Index           []UTXOItem      `json:"index"`
	}{
		BlockIdentifier: BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint32(height),
		},
		Index: utxoItems,
	}

	c.JSON(http.StatusOK, response)
}

// GetTweaks returns a simple list of tweaks as 33-byte public keys
func (h *Handler) GetTweaks(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("block height is required")))
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("could not parse block height")))
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch block hash")))
		return
	}

	tweakRows, err := h.db.TweaksForBlockAll(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching tweak rows")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve tweaks from database")))
		return
	}

	// Convert tweaks to hex strings
	tweaksOut := make([][33]byte, 0, len(tweakRows))
	for _, tweakRow := range tweakRows {
		if tweakRow != nil {
			tweaksOut = append(tweaksOut, tweakRow.Tweak)
		}
	}

	response := TweakIndexResponse{
		BlockIdentifier: BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint32(height),
		},
		Index: tweaksOut,
	}

	c.JSON(http.StatusOK, response)
}

// GetSpentOutputs returns spent output information in a compact format
func (h *Handler) GetSpentOutputs(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("block height is required")))
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("could not parse block height")))
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch block hash")))
		return
	}

	// Fetch spent outputs data
	spentOutputsData, err := h.db.FetchSpentOutputsShort(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching spent outputs")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve spent outputs from database")))
		return
	}

	fmt.Println("short outputs:", len(spentOutputsData))

	// Convert spent outputs data to SpentIndex format
	var spentOutputsShort SpentIndex
	if len(spentOutputsData) > 0 {
		for i := 0; i+8 <= len(spentOutputsData); i += 8 {
			// if i+8 <= len(spentOutputsData) {
			var outputBytes [8]byte
			copy(outputBytes[:], spentOutputsData[i:i+8])
			spentOutputsShort = append(spentOutputsShort, outputBytes)
			// }
		}
	}

	response := SpentIndexResponse{
		BlockIdentifier: BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint32(height),
		},
		Index: spentOutputsShort,
	}

	c.JSON(http.StatusOK, response)
}

// GetComputeIndex returns a compact transaction index with tweak mappings
func (h *Handler) GetComputeIndex(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("block height is required")))
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("could not parse block height")))
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch block hash")))
		return
	}

	computeIndexItems, err := h.db.FetchComputeIndex(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("error fetching compute index")
		c.JSON(
			http.StatusInternalServerError,
			NewErrorResponse(errors.New("could not retrieve data from database")),
		)
		return
	}

	// Handle empty data gracefully - return empty slice instead of null
	var indexItems []ComputeIndexItem
	if len(computeIndexItems) == 0 {
		indexItems = []ComputeIndexItem{}
	} else {
		// Convert to the expected format
		for _, item := range computeIndexItems {
			if item != nil {
				// Convert outputs short from []byte to OutputsShort
				var outputsShort OutputsShort
				outputsData := item.OutputsShort
				for i := 0; i < len(outputsData); i += 8 {
					if i+8 <= len(outputsData) {
						var outputBytes [8]byte
						copy(outputBytes[:], outputsData[i:i+8])
						outputsShort = append(outputsShort, outputBytes)
					}
				}

				indexItems = append(indexItems, ComputeIndexItem{
					TxId:         [32]byte(item.Txid[:]),
					Tweak:        [33]byte(item.Tweak[:]),
					OutputsShort: outputsShort,
				})
			}
		}
	}

	response := ComputeIndexResponse{
		BlockIdentifier: BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint32(height),
		},
		Index: indexItems,
	}

	c.JSON(http.StatusOK, response)
}

// GetFullBlock returns complete block data with all transaction details
func (h *Handler) GetFullBlock(c *gin.Context) {
	heightStr := c.Param("blockheight")
	if heightStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("block height is required")))
		return
	}

	height, err := strconv.ParseUint(heightStr, 10, 32)
	if err != nil {
		logging.L.Err(err).Msg("could not parse block height")
		c.JSON(http.StatusBadRequest, NewErrorResponse(errors.New("could not parse block height")))
		return
	}

	blockhash, err := h.db.GetBlockHashByHeight(uint32(height))
	if err != nil {
		logging.L.Err(err).Msg("could not fetch block hash")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch block hash")))
		return
	}

	// Get chain tip for FetchOutputsAll
	_, syncTip, err := h.db.GetChainTip()
	if err != nil {
		logging.L.Err(err).Msg("could not fetch chain tip")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not fetch chain tip")))
		return
	}

	// Fetch all the data we need for the full block
	outputs, err := h.db.FetchOutputsAll(blockhash, syncTip)
	if err != nil {
		logging.L.Err(err).Msg("error fetching outputs")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve outputs from database")))
		return
	}

	// Fetch tweaks with transaction IDs
	tweakRows, err := h.db.TweaksForBlockAll(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching tweak rows")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve tweaks from database")))
		return
	}

	// Group outputs by transaction ID
	txOutputs := make(map[string][]*database.Output)
	for _, output := range outputs {
		if output != nil {
			txid := hex.EncodeToString(output.Txid)
			txOutputs[txid] = append(txOutputs[txid], output)
		}
	}

	// Create a map of txid to tweak for quick lookup
	txidToTweak := make(map[string][33]byte)
	for _, tweakRow := range tweakRows {
		if tweakRow != nil {
			txid := hex.EncodeToString(tweakRow.Txid[:])
			txidToTweak[txid] = tweakRow.Tweak
		}
	}

	// Fetch all txid-outpoints mappings for this block
	txidOutpointsMap, err := h.db.FetchAllTxidOutpointsForBlock(blockhash)
	if err != nil {
		logging.L.Err(err).Msg("error fetching txid-outpoints mappings")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(errors.New("could not retrieve txid-outpoints from database")))
		return
	}

	// Build the full transaction items
	var fullTxItems []FullTxItem

	// Sort transaction IDs to ensure consistent ordering
	var sortedTxids []string
	for txid := range txOutputs {
		sortedTxids = append(sortedTxids, txid)
	}
	sort.Strings(sortedTxids)

	for _, txid := range sortedTxids {
		outputs := txOutputs[txid]
		txidBytes, err := hex.DecodeString(txid)
		if err != nil {
			logging.L.Err(err).Msg("error decoding txid")
			continue
		}

		var txidArray [32]byte
		copy(txidArray[:], txidBytes)

		// Get tweak for this transaction
		var tweak [33]byte
		if tweakBytes, exists := txidToTweak[txid]; exists {
			tweak = tweakBytes
		}

		// Get inputs (spent outpoints) for this transaction
		var inputs SpentOutpoints
		if outpoints, exists := txidOutpointsMap[txidArray]; exists {
			for i := range outpoints {
				utils.ReverseBytes(outpoints[i][:32])
			}
			inputs = SpentOutpoints(outpoints)
		}

		// Convert outputs to UTXO items
		var utxoItems []UTXOItemLight
		for _, output := range outputs {
			var pubkey [32]byte
			copy(pubkey[:], output.Pubkey)

			utxoItems = append(utxoItems, UTXOItemLight{
				Vout:   output.Vout,
				Amount: output.Amount,
				Pubkey: pubkey,
			})
		}

		fullTxItems = append(fullTxItems, FullTxItem{
			TxId:   [32]byte(utils.ReverseBytesCopy(txidArray[:])),
			Tweak:  tweak,
			Inputs: inputs,
			UTXOs:  utxoItems,
		})
	}

	response := FullBlockResponse{
		BlockIdentifier: BlockIdentifier{
			BlockHash:   utils.ReverseBytesCopy(blockhash),
			BlockHeight: uint32(height),
		},
		Index: fullTxItems,
	}

	c.JSON(http.StatusOK, response)
}
