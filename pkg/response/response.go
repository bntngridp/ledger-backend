package response

import (
	"errors"
	"log"
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/gin-gonic/gin"
)

// HandleError maps domain errors to proper HTTP response codes and structures.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	var status int
	var message string

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
		message = err.Error()

	case errors.Is(err, domain.ErrUnauthorized):
		status = http.StatusUnauthorized
		message = err.Error()

	case errors.Is(err, domain.ErrForbidden):
		status = http.StatusForbidden
		message = err.Error()

	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
		message = err.Error()

	case errors.Is(err, domain.ErrInsufficientBalance):
		status = http.StatusUnprocessableEntity
		message = err.Error()

	case errors.Is(err, domain.ErrInvalidInput),
		errors.Is(err, domain.ErrSelfTransfer),
		errors.Is(err, domain.ErrInvalidAddress),
		errors.Is(err, domain.ErrUnsupportedAsset),
		errors.Is(err, domain.ErrUnsupportedNetwork),
		errors.Is(err, domain.ErrSameAssetSwap):
		status = http.StatusBadRequest
		message = err.Error()

	case errors.Is(err, domain.ErrDuplicateTransaction):
		// Idempotent success case
		c.JSON(http.StatusOK, domain.SuccessResponse{
			Status:  http.StatusOK,
			Message: "transaction already processed",
		})
		return

	case errors.Is(err, domain.ErrExternalService),
		errors.Is(err, domain.ErrRateUnavailable):
		status = http.StatusBadGateway
		message = err.Error()

	case errors.Is(err, domain.ErrInvalidSignature):
		status = http.StatusForbidden
		message = err.Error()

	default:
		log.Printf("[ERROR] Unhandled error: %v", err)
		status = http.StatusInternalServerError
		message = "internal server error"
	}

	c.JSON(status, domain.ErrorResponse{
		Status:  status,
		Message: message,
	})
}

// SendSuccess sends a standardized success response.
func SendSuccess(c *gin.Context, status int, message string, data interface{}) {
	c.JSON(status, domain.SuccessResponse{
		Status:  status,
		Message: message,
		Data:    data,
	})
}
