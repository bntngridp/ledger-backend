package delivery

import (
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FiatHandler struct {
	fiatUC usecase.FiatUsecase
}

// NewFiatHandler constructs FiatHandler with fiat usecase.
func NewFiatHandler(fiatUC usecase.FiatUsecase) *FiatHandler {
	return &FiatHandler{fiatUC: fiatUC}
}

// WithdrawFiat godoc
// @Summary      Withdraw Rupiah to bank account
// @Description  Initiates a withdrawal of Rupiah (IDR) to an external bank account via Midtrans Iris Sandbox. Deducts Rp 2.500 admin fee.
// @Tags         fiat
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.WithdrawFiatRequest true "Withdrawal payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.WithdrawFiatResponse} "Withdrawal initiated"
// @Failure      400 {object} domain.ErrorResponse "Invalid input, below minimum withdrawal"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      422 {object} domain.ErrorResponse "Insufficient balance"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /fiat/withdraw [post]
func (h *FiatHandler) WithdrawFiat(c *gin.Context) {
	var req domain.WithdrawFiatRequest
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

	resp, err := h.fiatUC.WithdrawFiat(userID, req)
	if err != nil {
		response.HandleError(c, err)
		return
	}

	response.SendSuccess(c, http.StatusOK, "fiat withdrawal initiated successfully", resp)
}
