package delivery

import (
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type CryptoHandler struct {
	cryptoUC usecase.CryptoUsecase
}

// NewCryptoHandler constructs CryptoHandler with crypto usecase.
func NewCryptoHandler(cryptoUC usecase.CryptoUsecase) *CryptoHandler {
	return &CryptoHandler{cryptoUC: cryptoUC}
}

// GetDepositAddress godoc
// @Summary      Get or generate crypto deposit address
// @Description  Returns the authenticated user's EVM deposit address for the specified network and asset. Generates a new keypair if not exists.
// @Tags         crypto
// @Produce      json
// @Security     BearerAuth
// @Param        network query string true "Network name (polygon_amoy, sepolia)"
// @Param        asset_symbol query string true "Asset symbol (USDT, USDC)"
// @Success      200 {object} domain.SuccessResponse{data=domain.DepositAddressResponse} "Address retrieved successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid query parameters"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /crypto/address [get]
func (h *CryptoHandler) GetDepositAddress(c *gin.Context) {
	network := c.Query("network")
	assetSymbol := c.Query("asset_symbol")

	if network == "" || assetSymbol == "" {
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

	resp, err := h.cryptoUC.GetOrCreateDepositAddress(userID, network, assetSymbol)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "deposit address retrieved successfully", resp)
}

// WithdrawCrypto godoc
// @Summary      Withdraw crypto to external address
// @Description  Initiates an on-chain withdrawal of crypto to the specified destination address.
// @Tags         crypto
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.CryptoWithdrawRequest true "Withdrawal payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.CryptoWithdrawResponse} "Withdrawal initiated"
// @Failure      400 {object} domain.ErrorResponse "Invalid input or address"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      422 {object} domain.ErrorResponse "Insufficient balance"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /crypto/withdraw [post]
func (h *CryptoHandler) WithdrawCrypto(c *gin.Context) {
	var req domain.CryptoWithdrawRequest
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

	resp, err := h.cryptoUC.WithdrawCrypto(userID, req)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "withdrawal initiated successfully", resp)
}
