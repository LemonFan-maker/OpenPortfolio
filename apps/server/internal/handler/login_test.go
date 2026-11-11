package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/middleware"
)

const (
	testAdminPassword = "test-admin-password"
	testJWTSecret     = "0123456789abcdef0123456789abcdef"
)

func testAdminConfig() config.Config {
	return config.Config{AdminPassword: testAdminPassword, JWTSecret: testJWTSecret}
}

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
	cfg := testAdminConfig()
	router := gin.New()
	router.POST("/api/admin/login", login(cfg))

	for i := 0; i < loginAttemptLimit; i++ {
		recorder := loginRequest(router, "203.0.113.10:1234", "wrong")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, recorder.Code, http.StatusUnauthorized)
		}
	}

	blocked := loginRequest(router, "203.0.113.10:9999", testAdminPassword)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want %d; body=%s", blocked.Code, http.StatusTooManyRequests, blocked.Body.String())
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("blocked response has no Retry-After header")
	}

	other := loginRequest(router, "203.0.113.11:1234", testAdminPassword)
	if other.Code != http.StatusOK {
		t.Fatalf("other ip status = %d, want %d; body=%s", other.Code, http.StatusOK, other.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Authenticated bool `json:"authenticated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(other.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if !body.Success || !body.Data.Authenticated {
		t.Fatalf("unexpected login response: %#v", body)
	}

	cookies := other.Result().Cookies()
	tokenCookie := findCookie(cookies, middleware.AdminTokenCookieName)
	if tokenCookie == nil || tokenCookie.Value == "" {
		t.Fatalf("login cookies = %#v, want admin token cookie", cookies)
	}

	protected := gin.New()
	protected.GET("/protected", middleware.RequireAuth(cfg.JWTSecret, cfg.AdminPassword), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(tokenCookie)
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("issued cookie status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func TestLoginSetsHttpOnlyAuthCookieAndCSRFCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAdminConfig()
	cfg.AdminCookieSecure = true
	router := gin.New()
	router.POST("/api/admin/login", login(cfg))

	recorder := loginRequest(router, "203.0.113.30:1234", testAdminPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	cookies := recorder.Result().Cookies()
	authCookie := findCookie(cookies, middleware.AdminTokenCookieName)
	if authCookie == nil {
		t.Fatalf("login cookies = %#v, want auth cookie", cookies)
	}
	if !authCookie.HttpOnly {
		t.Fatal("auth cookie must be HttpOnly")
	}
	if !authCookie.Secure {
		t.Fatal("auth cookie must be Secure when ADMIN_COOKIE_SECURE is enabled")
	}
	if authCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("auth cookie SameSite = %d, want Strict", authCookie.SameSite)
	}
	if authCookie.Path != "/api/admin" {
		t.Fatalf("auth cookie path = %q, want /api/admin", authCookie.Path)
	}

	csrfCookie := findCookie(cookies, middleware.AdminCSRFCookieName)
	if csrfCookie == nil || csrfCookie.Value == "" {
		t.Fatalf("login cookies = %#v, want CSRF cookie", cookies)
	}
	if csrfCookie.HttpOnly {
		t.Fatal("CSRF cookie must be readable by the admin page")
	}
	if csrfCookie.Path != "/" {
		t.Fatalf("CSRF cookie path = %q, want /", csrfCookie.Path)
	}
}

func TestLoginRejectsDefaultAdminSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/admin/login", login(config.Config{}))

	recorder := loginRequest(router, "203.0.113.40:1234", "anything")
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
}

func TestLoginSuccessResetsRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAdminConfig()
	router := gin.New()
	router.POST("/api/admin/login", login(cfg))

	for i := 0; i < loginAttemptLimit-1; i++ {
		if recorder := loginRequest(router, "203.0.113.20:1234", "wrong"); recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, recorder.Code, http.StatusUnauthorized)
		}
	}
	if recorder := loginRequest(router, "203.0.113.20:1234", testAdminPassword); recorder.Code != http.StatusOK {
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

func TestLoginIgnoresForwardedForWhenProxyIsUntrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAdminConfig()
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	router.POST("/api/admin/login", login(cfg))

	for i := 0; i < loginAttemptLimit; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "203.0.113."+strconv.Itoa(i+1))
		req.RemoteAddr = "198.51.100.10:1234"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want %d", i+1, recorder.Code, http.StatusUnauthorized)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"password":"`+testAdminPassword+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	req.RemoteAddr = "198.51.100.10:1234"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed forwarded IP status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}
