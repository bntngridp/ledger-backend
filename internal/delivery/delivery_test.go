package delivery

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestGinRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestRegisterHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewAuthHandler(nil, "secret", 24)
	r.POST("/api/v1/auth/register", handler.Register)

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestLoginHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewAuthHandler(nil, "secret", 24)
	r.POST("/api/v1/auth/login", handler.Login)

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestTransferHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewTransferHandler(nil)
	r.POST("/api/v1/transfer", func(c *gin.Context) {
		c.Set("user_id", "7ced518f-b482-4147-b108-634b2ada54b9")
		handler.Transfer(c)
	})

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/transfer", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestSwapHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewExchangeHandler(nil)
	r.POST("/api/v1/exchange/swap", func(c *gin.Context) {
		c.Set("user_id", "7ced518f-b482-4147-b108-634b2ada54b9")
		handler.Swap(c)
	})

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/exchange/swap", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func jsonBytes(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestWithdrawFiatHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewFiatHandler(nil)
	r.POST("/api/v1/fiat/withdraw", func(c *gin.Context) {
		c.Set("user_id", "7ced518f-b482-4147-b108-634b2ada54b9")
		handler.WithdrawFiat(c)
	})

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/fiat/withdraw", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestCryptoWithdrawHandler_InvalidInput(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewCryptoHandler(nil)
	r.POST("/api/v1/crypto/withdraw", func(c *gin.Context) {
		c.Set("user_id", "7ced518f-b482-4147-b108-634b2ada54b9")
		handler.WithdrawCrypto(c)
	})

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/crypto/withdraw", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestWebhookHandler_MidtransInvalidBody(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewWebhookHandler(nil)
	r.POST("/api/v1/webhooks/midtrans", handler.HandleMidtrans)

	invalidJSON := []byte(`{ invalid json }`)

	req := httptest.NewRequest("POST", "/api/v1/webhooks/midtrans", bytes.NewBuffer(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}

func TestGetNotifications_Unauthorized(t *testing.T) {
	r := setupTestGinRouter()
	handler := NewNotificationHandler(nil)
	r.GET("/api/v1/notifications", handler.GetNotifications)

	req := httptest.NewRequest("GET", "/api/v1/notifications", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
	}
}
