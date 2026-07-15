package delivery

import (
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	webhookUC usecase.WebhookUsecase
}

func NewWebhookHandler(webhookUC usecase.WebhookUsecase) *WebhookHandler {
	return &WebhookHandler{webhookUC: webhookUC}
}

// HandleMidtrans godoc
// @Summary      Handle Midtrans Webhook Notification
// @Description  Endpoint for Midtrans payment gateway notification webhook. Validates SHA-512 signature, processes billing status changes, and settles transaction balances.
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Param        payload body map[string]interface{} true "Midtrans Notification Payload"
// @Success      200 {object} domain.SuccessResponse "Notification processed successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid payload or signature validation failed"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /webhooks/midtrans [post]
func (h *WebhookHandler) HandleMidtrans(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid payload: " + err.Error(),
		})
		return
	}

	if err := h.webhookUC.ProcessMidtransNotification(payload); err != nil {
		msg := err.Error()
		if msg == "invalid signature key" || msg == "missing or invalid order_id" {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				Status:  http.StatusBadRequest,
				Message: msg,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: msg,
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "notification processed successfully",
	})
}

// HandleIris godoc
// @Summary      Handle Midtrans Iris Callback Notification
// @Description  Endpoint for Midtrans Iris disbursement notification webhook. Updates payout transaction status to success or failed (and refunds balance upon failure).
// @Tags         webhooks
// @Accept       json
// @Produce      json
// @Param        payload body []domain.IrisCallbackItem true "Midtrans Iris Notification Payload"
// @Success      200 {object} domain.SuccessResponse "Notification processed successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid payload"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /webhooks/iris [post]
func (h *WebhookHandler) HandleIris(c *gin.Context) {
	var payload []domain.IrisCallbackItem
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid payload: " + err.Error(),
		})
		return
	}

	if err := h.webhookUC.ProcessIrisNotification(payload); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "notification processed successfully",
	})
}

