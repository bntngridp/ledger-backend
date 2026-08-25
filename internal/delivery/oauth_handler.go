package delivery

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/bntngridp/ledger-backend/internal/domain"
	"github.com/bntngridp/ledger-backend/internal/usecase"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

type OAuthHandler struct {
	authUC      usecase.AuthUsecase
	oauthConfig *oauth2.Config
	jwtSecret   string
	expiryHours int
}

// NewOAuthHandler constructs OAuthHandler with usecase, OAuth2 config and JWT secret details.
func NewOAuthHandler(authUC usecase.AuthUsecase, oauthConfig *oauth2.Config, jwtSecret string, expiryHours int) *OAuthHandler {
	return &OAuthHandler{
		authUC:      authUC,
		oauthConfig: oauthConfig,
		jwtSecret:   jwtSecret,
		expiryHours: expiryHours,
	}
}

func generateStateOauthCookie(c *gin.Context) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Set state cookie for CSRF protection
	c.SetCookie("oauthstate", state, int(10*time.Minute/time.Second), "/", "", false, true)
	return state
}

// LoginGoogle godoc
// @Summary      Redirect to Google Login
// @Description  Redirects the client to Google's OAuth2 consent page.
// @Tags         auth
// @Success      307 "Temporary Redirect to Google Consent Page"
// @Router       /auth/google [get]
func (h *OAuthHandler) LoginGoogle(c *gin.Context) {
	state := generateStateOauthCookie(c)
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "select_account"))
	c.Redirect(http.StatusTemporaryRedirect, url)
}

// GoogleCallback godoc
// @Summary      Google Login Callback
// @Description  Handles the authorization code returned by Google, exchanges it for an access token, fetches the user's profile, and issues a JWT token.
// @Tags         auth
// @Param        state query string true "OAuth state parameter"
// @Param        code query string true "OAuth authorization code"
// @Success      200 {object} domain.SuccessResponse{data=domain.LoginResponse} "Google login successful"
// @Failure      400 {object} domain.ErrorResponse "Invalid oauth state or missing authorization code"
// @Failure      500 {object} domain.ErrorResponse "Internal server error"
// @Failure      502 {object} domain.ErrorResponse "Bad gateway: failed to retrieve Google profile"
// @Router       /auth/google/callback [get]
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	// 1. Verify CSRF state token
	stateCookie, err := c.Cookie("oauthstate")
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid state cookie: " + err.Error(),
		})
		return
	}

	stateParam := c.Query("state")
	if stateParam != stateCookie {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "invalid oauth state",
		})
		return
	}

	// Clear state cookie
	c.SetCookie("oauthstate", "", -1, "/", "", false, true)

	// 2. Get auth code
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Status:  http.StatusBadRequest,
			Message: "missing authorization code",
		})
		return
	}

	// 3. Exchange code for access token
	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.ErrorResponse{
			Status:  http.StatusBadGateway,
			Message: "failed to exchange oauth code: " + err.Error(),
		})
		return
	}

	// 4. Retrieve Google User Profile Info
	client := h.oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusBadGateway, domain.ErrorResponse{
			Status:  http.StatusBadGateway,
			Message: "failed to retrieve google user info: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, domain.ErrorResponse{
			Status:  http.StatusBadGateway,
			Message: fmt.Sprintf("google api returned status code %d", resp.StatusCode),
		})
		return
	}

	var googleProfile domain.GoogleUserProfile
	if err := json.NewDecoder(resp.Body).Decode(&googleProfile); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Status:  http.StatusInternalServerError,
			Message: "failed to decode google profile: " + err.Error(),
		})
		return
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:8081"
	}

	if googleProfile.Email == "" {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, url.QueryEscape("google account has no email")))
		return
	}

	// 5. Call usecase to login / register
	loginResp, err := h.authUC.LoginWithGoogle(&googleProfile, h.jwtSecret, h.expiryHours)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, url.QueryEscape("login failed: "+err.Error())))
		return
	}

	if loginResp.TwoFactorRequired {
		c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?pre_auth_token=%s&requires_2fa=true", frontendURL, loginResp.PreAuthToken))
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?token=%s", frontendURL, loginResp.Token))
}
