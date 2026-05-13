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

func TestAdminCacheClearSucceedsWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.Config{
		AdminPassword:  "test-password",
		JWTSecret:      "test-secret",
		RedisAddr:      "",
		GitHubUsername: "tester",
	}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	token, err := middleware.NewToken(cfg.JWTSecret, time.Hour)
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
		AdminPassword:  "test-password",
		JWTSecret:      "test-secret",
		RedisAddr:      "",
		GitHubUsername: "tester",
	}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)

	router := gin.New()
	RegisterAdminRoutes(router, cfg, store, githubService, aiService)

	token, err := middleware.NewToken(cfg.JWTSecret, time.Hour)
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
