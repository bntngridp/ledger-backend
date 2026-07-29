package delivery

import (
	"net/http"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authUC      usecase.AuthUsecase
	jwtSecret   string
	expiryHours int
}

// NewAuthHandler godoc
// @Description Constructs AuthHandler with usecase and JWT config.
func NewAuthHandler(authUC usecase.AuthUsecase, jwtSecret string, expiryHours int) *AuthHandler {
	return &AuthHandler{
		authUC:      authUC,
		jwtSecret:   jwtSecret,
		expiryHours: expiryHours,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account and an associated wallet with balance 0 in a single atomic transaction.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body domain.RegisterRequest true "Registration payload"
// @Success      201 {object} domain.SuccessResponse{data=domain.RegisterResponse} "User registered successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid request body"
// @Failure      409 {object} domain.ErrorResponse "Email or username already taken"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.authUC.Register(req.Username, req.Email, req.Password)
	if err != nil {
		switch err.Error() {
		case "email already registered", "username already taken":
			c.JSON(http.StatusConflict, domain.ErrorResponse{
				Status:  http.StatusConflict,
				Message: err.Error(),
			})
			return
		default:
			c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
				Status:  http.StatusInternalServerError,
				Message: "internal server error",
			})
			return
		}
	}

	c.JSON(http.StatusCreated, domain.SuccessResponse{
		Status:  http.StatusCreated,
		Message: "user registered successfully",
		Data:    resp,
	})
}

// Login godoc
// @Summary      Login user
// @Description  Authenticates a user and returns a JWT token valid for 24 hours.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body domain.LoginRequest true "Login payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.LoginResponse} "Login successful"
// @Failure      400 {object} domain.ErrorResponse "Invalid request body"
// @Failure      401 {object} domain.ErrorResponse "Invalid email or password"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.authUC.Login(req.Email, req.Password, h.jwtSecret, h.expiryHours)
	if err != nil {
		if err.Error() == "invalid email or password" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Status:  http.StatusUnauthorized,
				Message: "invalid email or password",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: "internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "login successful",
		Data:    resp,
	})
}

// Enable2FA godoc
// @Summary      Generate 2FA TOTP secret key
// @Description  Generates a new TOTP secret key and QR code provisioning URL for the authenticated user.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse{data=domain.Enable2FAResponse} "Secret generated successfully"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Router       /auth/2fa/enable [post]
func (h *AuthHandler) Enable2FA(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	resp, err := h.authUC.Generate2FASecret(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "2FA secret generated successfully",
		Data:    resp,
	})
}

// Verify2FA godoc
// @Summary      Verify and enable 2FA TOTP
// @Description  Verifies the first TOTP code to confirm scanning and officially enables 2FA for the account.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.Verify2FARequest true "Verify 2FA payload"
// @Success      200 {object} domain.SuccessResponse "2FA enabled successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid code or signature"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Router       /auth/2fa/verify [post]
func (h *AuthHandler) Verify2FA(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	var req domain.Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	recoveryCodes, err := h.authUC.Enable2FA(userID, req.Code)
	if err != nil {
		if err == domain.ErrInvalid2FACode {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "2FA enabled successfully",
		Data: domain.Enable2FAConfirmResponse{
			RecoveryCodes: recoveryCodes,
		},
	})
}

// Send2FAEmailOTP godoc
// @Summary      Send Email OTP for 2FA Deactivation/Recovery
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse "OTP sent to email"
// @Router       /auth/2fa/email-otp/send [post]
func (h *AuthHandler) Send2FAEmailOTP(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	if err := h.authUC.Send2FAEmailOTP(userID); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "Kode OTP telah dikirimkan ke email Anda",
	})
}

// Disable2FA godoc
// @Summary      Disable 2FA TOTP
// @Description  Disables 2FA by validating the current TOTP code, Recovery Code, or Email OTP.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.Disable2FARequest true "Disable 2FA payload"
// @Success      200 {object} domain.SuccessResponse "2FA disabled successfully"
// @Failure      400 {object} domain.ErrorResponse "Invalid code"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Router       /auth/2fa/disable [post]
func (h *AuthHandler) Disable2FA(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	var req domain.Disable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.authUC.Disable2FA(userID, req); err != nil {
		if err == domain.ErrInvalid2FACode {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "2FA disabled successfully",
	})
}

// Login2FA godoc
// @Summary      Complete 2FA login challenge
// @Description  Verifies the TOTP code against the pre-auth token and returns the final JWT token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body domain.Login2FARequest true "Login 2FA challenge payload"
// @Success      200 {object} domain.SuccessResponse{data=domain.LoginResponse} "Login successful"
// @Failure      400 {object} domain.ErrorResponse "Invalid pre-auth token or code"
// @Failure      401 {object} domain.ErrorResponse "Invalid TOTP code"
// @Router       /auth/2fa/login [post]
func (h *AuthHandler) Login2FA(c *gin.Context) {
	var req domain.Login2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	resp, err := h.authUC.Verify2FALogin(req.PreAuthToken, req.Code, h.jwtSecret, h.expiryHours)
	if err != nil {
		if err == domain.ErrInvalid2FACode || err == domain.ErrUnauthorized {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Status:  http.StatusUnauthorized,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: "internal server error: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "login successful",
		Data:    resp,
	})
}

// SendChangePasswordEmailOTP godoc
// @Summary      Send Email OTP for Change Password Verification
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} domain.SuccessResponse "OTP sent to email"
// @Router       /auth/password/email-otp/send [post]
func (h *AuthHandler) SendChangePasswordEmailOTP(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	if err := h.authUC.SendChangePasswordEmailOTP(userID); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "Kode OTP verifikasi telah dikirimkan ke email Anda",
	})
}

// ChangePassword godoc
// @Summary      Change User Password
// @Description  Updates user password with old password, email OTP, and optional 2FA verification.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.ChangePasswordRequest true "Change Password payload"
// @Success      200 {object} domain.SuccessResponse "Password changed successfully"
// @Failure      400 {object} domain.ErrorResponse "Bad request / Invalid verification"
// @Failure      401 {object} domain.ErrorResponse "Unauthorized"
// @Router       /auth/password/change [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	var req domain.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.authUC.ChangePassword(userID, req); err != nil {
		if err == domain.ErrInvalid2FACode {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "Kata sandi berhasil diperbarui",
	})
}

// SetupPIN handles setting up or updating transaction PIN
func (h *AuthHandler) SetupPIN(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	var req domain.SetupPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.authUC.SetupPIN(userID, req.PIN); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "PIN Transaksi berhasil disetel",
	})
}

// VerifyPIN handles verifying transaction PIN before sensitive operations
func (h *AuthHandler) VerifyPIN(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Status:  http.StatusUnauthorized,
			Message: "unauthorized",
		})
		return
	}

	var req domain.VerifyPINRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.authUC.VerifyPIN(userID, req.PIN); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.SuccessResponse{
		Status:  http.StatusOK,
		Message: "PIN Transaksi valid",
	})
}
