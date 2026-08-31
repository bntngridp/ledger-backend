package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/bntngridp/ledger-backend/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type securityPayload struct {
	TwoFactorCode string `json:"two_factor_code"`
	EmailOTP      string `json:"email_otp"`
}

// Require2FAIfEnabled enforces dual TOTP and Email OTP verification check if the authenticated user has enabled 2FA.
func Require2FAIfEnabled(authUC usecase.AuthUsecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr, exists := c.Get("user_id")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
			})
			return
		}

		userID, err := uuid.Parse(userIDStr.(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{
				Status:  http.StatusUnauthorized,
				Message: "unauthorized",
			})
			return
		}

		twoFactorCode := c.GetHeader("X-2FA-Code")
		emailOTP := c.GetHeader("X-Email-OTP")

		// Fallback to reading from JSON request body if headers are not set
		if (twoFactorCode == "" || emailOTP == "") && c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				// Restore body for subsequent handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				var secReq securityPayload
				if json.Unmarshal(bodyBytes, &secReq) == nil {
					if twoFactorCode == "" {
						twoFactorCode = secReq.TwoFactorCode
					}
					if emailOTP == "" {
						emailOTP = secReq.EmailOTP
					}
				}
			}
		}

		if err := authUC.VerifyPaymentSecurity(userID, twoFactorCode, emailOTP); err != nil {
			response.HandleError(c, err)
			c.Abort()
			return
		}

		c.Next()
	}
}
