package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func generateValidTestToken(secret, userID, email string) string {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return tokenStr
}

func TestJWTAuth_Success(t *testing.T) {
	secret := "test_jwt_secret"
	userID := "7ced518f-b482-4147-b108-634b2ada54b9"
	email := "test@example.com"
	token := generateValidTestToken(secret, userID, email)

	r := setupTestRouter()
	r.Use(JWTAuth(secret))
	r.GET("/protected", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		em, _ := c.Get("email")
		c.JSON(http.StatusOK, gin.H{
			"user_id": uid,
			"email":   em,
		})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["user_id"] != userID {
		t.Errorf("expected user_id %s, got %v", userID, body["user_id"])
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r := setupTestRouter()
	r.Use(JWTAuth("secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	r := setupTestRouter()
	r.Use(JWTAuth("secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormatToken")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	r := setupTestRouter()
	r.Use(JWTAuth("correct_secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	badToken := generateValidTestToken("wrong_secret", "user1", "email@mail.com")

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestCORSMiddleware(t *testing.T) {
	r := setupTestRouter()
	r.Use(CORSMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// GET Request Check
	req := httptest.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin: *, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// OPTIONS Preflight Check
	reqOptions := httptest.NewRequest("OPTIONS", "/ping", nil)
	wOptions := httptest.NewRecorder()
	r.ServeHTTP(wOptions, reqOptions)

	if wOptions.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS, got %d", wOptions.Code)
	}
}

func TestIPBasedRateLimiter(t *testing.T) {
	r := setupTestRouter()
	// Allow 2 tokens maximum
	r.Use(IPBasedRateLimiter(2, 1, 1*time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First 2 requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d expected 200 OK, got %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited (429)
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.RemoteAddr = "192.168.1.100:12345"
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 Too Many Requests, got %d", w3.Code)
	}
}

func TestRequire2FAIfEnabled_Unauthorized(t *testing.T) {
	r := setupTestRouter()
	r.Use(Require2FAIfEnabled(nil))
	r.POST("/sensitive", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/sensitive", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
	}
}
