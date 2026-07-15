package delivery

import (
	"net/http"
	"strconv"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type WalletHandler struct {
	walletUC usecase.WalletUsecase
}

// NewWalletHandler constructs WalletHandler with wallet usecase.
func NewWalletHandler(walletUC usecase.WalletUsecase) *WalletHandler {
	return &WalletHandler{walletUC: walletUC}
}

// TopUp godoc
// @Summary      Top-up wallet balance
// @Description  Adds the specified amount to the authenticated user's wallet balance. Records a transaction with type 'topup'.
// @Tags         wallet
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.TopUpRequest true "Top-up payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.TopUpResponse} "Top-up successful"
// @Failure      400 {object} domain.ErrorResponse "Invalid request"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /topup [post]
func (h *WalletHandler) TopUp(c *gin.Context) {
	var req domain.TopUpRequest
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

	resp, err := h.walletUC.TopUp(userID, req.Amount, "IDR", req.Notes)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "top-up initiated successfully", resp)
}

// GetTransactionHistory godoc
// @Summary      Get transaction history
// @Description  Returns the authenticated user's transaction history with pagination and filters.
// @Tags         wallet
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number"
// @Param        per_page query int false "Items per page"
// @Param        asset query string false "Filter by asset (e.g. IDR, USDT, USDC)"
// @Param        type query string false "Filter by transaction type"
// @Success      200 {object} domain.SuccessResponse{data=domain.TransactionHistoryResponse} "Transaction history retrieved"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /transactions [get]
func (h *WalletHandler) GetTransactionHistory(c *gin.Context) {
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

	// Parse pagination and filter query params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	assetFilter := c.Query("asset")
	typeFilter := c.Query("type")

	resp, err := h.walletUC.GetTransactionHistory(userID, page, perPage, assetFilter, typeFilter)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "transaction history retrieved", resp)
}

// GetDashboard godoc
// @Summary      Get wallet dashboard
// @Description  Returns the authenticated user's wallet balances (IDR, USDT, USDC) and estimated total IDR value.
// @Tags         wallet
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse{data=domain.DashboardResponse} "Dashboard data retrieved"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /wallet/dashboard [get]
func (h *WalletHandler) GetDashboard(c *gin.Context) {
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

	dashboard, err := h.walletUC.GetDashboard(userID)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "dashboard retrieved successfully", dashboard)
}
