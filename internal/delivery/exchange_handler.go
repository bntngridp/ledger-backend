package delivery

import (
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ExchangeHandler struct {
	exchangeUC usecase.ExchangeUsecase
}

// NewExchangeHandler constructs ExchangeHandler with exchange usecase.
func NewExchangeHandler(exchangeUC usecase.ExchangeUsecase) *ExchangeHandler {
	return &ExchangeHandler{exchangeUC: exchangeUC}
}

// GetRate godoc
// @Summary      Get exchange rate
// @Description  Returns the current exchange rate for a given pair (e.g. USDT_IDR, USDC_IDR) from Binance API.
// @Tags         exchange
// @Produce      json
// @Param        pair query string true "Pair symbol (e.g. USDT_IDR, USDC_IDR)"
// @Success      200 {object} domain.SuccessResponse{data=domain.ExchangeRateResponse} "Exchange rate retrieved"
// @Failure      400 {object} domain.ErrorResponse "Invalid query parameters"
// @Failure      502 {object} domain.ErrorResponse "External Binance service failure"
// @Router       /exchange/rate [get]
func (h *ExchangeHandler) GetRate(c *gin.Context) {
	pair := c.Query("pair")
	if pair == "" {
		response.HandleError(c, domain.ErrInvalidInput)
		return
	}

	resp, err := h.exchangeUC.GetExchangeRate(pair)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "exchange rate retrieved successfully", resp)
}

// Swap godoc
// @Summary      Swap assets (Fiat/Crypto)
// @Description  Exchanges a specified amount of one asset to another asset internally. Applies a 0.5% platform fee.
// @Tags         exchange
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.SwapRequest true "Swap payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.SwapResponse} "Swap transaction completed"
// @Failure      400 {object} domain.ErrorResponse "Invalid input, same asset swap"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      422 {object} domain.ErrorResponse "Insufficient balance"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /exchange/swap [post]
func (h *ExchangeHandler) Swap(c *gin.Context) {
	var req domain.SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.HandleError(c, domain.ErrInvalidInput)
		return
	}

	userIDStr, exists := c.Get("user_id")
	if !exists {
		response.HandleError(c, domain.ErrUnauthorized)
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		response.HandleError(c, domain.ErrUnauthorized)
		return
	}

	resp, err := h.exchangeUC.Swap(userID, req)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "swap completed successfully", resp)
}
