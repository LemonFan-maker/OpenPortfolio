package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/middleware"
)

func loginRequest(router *gin.Engine, remoteAddr string, password string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"password":"`+password+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestLoginRateLimitsFailuresPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{AdminPassword: "secret", JWTSecret: "jwt-secret"}
	router := gin.New()
	router.POST("/api/admin/login", login(cfg))

	for i := 0; i < loginAttemptLimit; i++ {
		recorder := loginRequest(router, "203.0.113.10:1234", "wrong")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, recorder.Code, http.StatusUnauthorized)
		}
	}

	blocked := loginRequest(router, "203.0.113.10:9999", "secret")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d; body=%s", blocked.Code, http.StatusTooManyRequests, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("blocked response has no Retry-After header")
	}

	other := loginRequest(router, "203.0.113.11:1234", "secret")
	if other.Code != http.StatusOK {
		t.Fatalf("other ip status = %d, want %d; body=%s", other.Code, http.StatusOK, other.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(other.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !body.Success || body.Data.Token == "" {
		t.Fatalf("unexpected login response: %#v", body)
	}

	protected := gin.New()
	protected.GET("/protected", middleware.RequireAuth(cfg.JWTSecret, cfg.AdminPassword), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+body.Data.Token)
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("issued token status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestLoginSuccessResetsRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{AdminPassword: "secret", JWTSecret: "jwt-secret"}
	router := gin.New()
	router.POST("/api/admin/login", login(cfg))

	for i := 0; i < loginAttemptLimit-1; i++ {
		if recorder := loginRequest(router, "203.0.113.20:1234", "wrong"); recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, recorder.Code, http.StatusUnauthorized)
		}
	}
	if recorder := loginRequest(router, "203.0.113.20:1234", "secret"); recorder.Code != http.StatusOK {
		t.Fatalf("successful login status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder := loginRequest(router, "203.0.113.20:1234", "wrong"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("post-reset failure status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginLimiterBlocksAndResets(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(2, time.Minute, func() time.Time { return now })

	if !limiter.allow("ip") {
		t.Fatal("new key should be allowed")
	}
	limiter.fail("ip")
	limiter.fail("ip")
	if limiter.allow("ip") {
		t.Fatal("key should be blocked after reaching the limit")
	}
	if !limiter.allow("other") {
		t.Fatal("other key should not be affected")
	}
	if retry := limiter.retryAfter("ip"); retry <= 0 || retry > time.Minute {
		t.Fatalf("retryAfter = %v, want (0, 1m]", retry)
	}

	now = now.Add(time.Minute)
	if !limiter.allow("ip") {
		t.Fatal("key should be allowed after the window expires")
	}
	limiter.fail("ip")
	limiter.reset("ip")
	if !limiter.allow("ip") {
		t.Fatal("key should be allowed after a reset")
	}
}
