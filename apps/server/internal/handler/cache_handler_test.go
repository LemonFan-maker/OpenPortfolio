package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/middleware"
	"portfolio-hub/apps/server/internal/model"
)

func TestHealthReportsRedisDisabledWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := cache.NewStore(config.Config{RedisAddr: ""})

	router := gin.New()
	router.GET("/api/health", Health(store))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
			Redis  string `json:"redis"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Data.Status != "ok" || body.Data.Redis != "disabled" {
		t.Fatalf("unexpected health body: %#v", body)
	}
}

func TestHealthReportsRedisDegradedWhenUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := cache.NewStore(config.Config{RedisAddr: "127.0.0.1:0"})

	router := gin.New()
	router.GET("/api/health", Health(store))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Status string `json:"status"`
			Redis  string `json:"redis"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Data.Status != "ok" || body.Data.Redis != "degraded" {
		t.Fatalf("unexpected health body: %#v", body)
	}
}

func TestAdminCacheClearSucceedsWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		AdminPassword:  testAdminPassword,
		JWTSecret:      testJWTSecret,
		RedisAddr:      "",
		GitHubUsername: "tester",
	}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	token, err := middleware.NewToken(cfg.JWTSecret, cfg.AdminPassword, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cache/clear/repo-details", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Success bool                       `json:"success"`
		Data    model.CacheOperationRecord `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got %#v", body)
	}
	if body.Data.Status != "success" {
		t.Fatalf("record status = %q, want success", body.Data.Status)
	}
	if body.Data.Scope != "github:repo-details" {
		t.Fatalf("record scope = %q, want github:repo-details", body.Data.Scope)
	}
	if body.Data.AffectedKeys != 0 {
		t.Fatalf("affected keys = %d, want 0 without redis", body.Data.AffectedKeys)
	}
}

func TestAdminCacheRecordsUseMemoryFallbackWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		AdminPassword:  testAdminPassword,
		JWTSecret:      testJWTSecret,
		RedisAddr:      "",
		GitHubUsername: "tester",
	}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	token, err := middleware.NewToken(cfg.JWTSecret, cfg.AdminPassword, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	clearRecorder := httptest.NewRecorder()
	clearReq := httptest.NewRequest(http.MethodPost, "/api/admin/cache/clear/repo-details", nil)
	clearReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(clearRecorder, clearReq)
	if clearRecorder.Code != http.StatusOK {
		t.Fatalf("clear status code = %d, want %d; body=%s", clearRecorder.Code, http.StatusOK, clearRecorder.Body.String())
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/cache/records", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Success bool                         `json:"success"`
		Data    []model.CacheOperationRecord `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success {
		t.Fatalf("expected success response, got %#v", body)
	}
	if len(body.Data) != 1 {
		t.Fatalf("records length = %d, want 1 from memory fallback", len(body.Data))
	}
	if body.Data[0].Scope != "github:repo-details" {
		t.Fatalf("record scope = %q, want github:repo-details", body.Data[0].Scope)
	}
}

func TestAdminRoutesFailClosedWithoutStrongSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{RedisAddr: ""}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	session := httptest.NewRecorder()
	router.ServeHTTP(session, httptest.NewRequest(http.MethodGet, "/api/admin/session", nil))
	if session.Code != http.StatusServiceUnavailable {
		t.Fatalf("session status = %d, want %d", session.Code, http.StatusServiceUnavailable)
	}

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", nil)
	router.ServeHTTP(login, req)
	if login.Code != http.StatusServiceUnavailable {
		t.Fatalf("login status = %d, want %d", login.Code, http.StatusServiceUnavailable)
	}
}

func TestAdminCookieCSRFFlowThroughRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testAdminConfig()
	cfg.RedisAddr = ""
	cfg.GitHubUsername = "tester"
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	login := loginRequest(router, "203.0.113.50:1234", testAdminPassword)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", login.Code, http.StatusOK, login.Body.String())
	}
	authCookie := findCookie(login.Result().Cookies(), middleware.AdminTokenCookieName)
	csrfCookie := findCookie(login.Result().Cookies(), middleware.AdminCSRFCookieName)
	if authCookie == nil || csrfCookie == nil {
		t.Fatalf("login cookies = %#v, want auth and CSRF cookies", login.Result().Cookies())
	}

	session := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/api/admin/session", nil)
	sessionReq.AddCookie(authCookie)
	router.ServeHTTP(session, sessionReq)
	if session.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d; body=%s", session.Code, http.StatusOK, session.Body.String())
	}

	blocked := httptest.NewRecorder()
	blockedReq := httptest.NewRequest(http.MethodPost, "/api/admin/cache/clear/repo-details", nil)
	blockedReq.AddCookie(authCookie)
	router.ServeHTTP(blocked, blockedReq)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("write without CSRF status = %d, want %d; body=%s", blocked.Code, http.StatusForbidden, blocked.Body.String())
	}

	allowed := httptest.NewRecorder()
	allowedReq := httptest.NewRequest(http.MethodPost, "/api/admin/cache/clear/repo-details", nil)
	allowedReq.AddCookie(authCookie)
	allowedReq.AddCookie(csrfCookie)
	allowedReq.Header.Set("X-CSRF-Token", csrfCookie.Value)
	router.ServeHTTP(allowed, allowedReq)
	if allowed.Code != http.StatusOK {
		t.Fatalf("write with CSRF status = %d, want %d; body=%s", allowed.Code, http.StatusOK, allowed.Body.String())
	}
}
