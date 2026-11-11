package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/middleware"
	"portfolio-hub/apps/server/internal/model"
	"portfolio-hub/apps/server/internal/response"
	"portfolio-hub/apps/server/internal/service"
)

const cacheOperationRecordKey = "admin:cache:refresh-records"
const cacheOperationRecordLimit int64 = 30
const adminTokenTTL = 24 * time.Hour

func RegisterAdminRoutes(router *gin.Engine, cfg config.Config, store *cache.Store, githubService *service.GitHubService, aiService *service.AIService) {
	router.POST("/api/admin/login", login(cfg))
	router.POST("/api/admin/logout", logout(cfg))

	admin := router.Group("/api/admin")
	if cfg.AdminAuthReady() {
		admin.Use(middleware.RequireAuth(cfg.JWTSecret, cfg.AdminPassword))
	} else {
		admin.Use(adminAuthUnavailable)
	}
	admin.GET("/session", adminSession)
	admin.GET("/cache/records", cacheRecords(store))
	admin.POST("/cache/refresh/repos", refreshRepos(githubService, store))
	admin.POST("/cache/refresh/languages", refreshLanguages(githubService, store))
	admin.POST("/cache/refresh/activity", refreshActivity(githubService, store))
	admin.POST("/cache/clear/repo-details", clearRepoDetails(store))
	admin.POST("/cache/clear/repo-browser", clearRepoBrowser(store))
	admin.POST("/cache/clear/ai-projects", clearAIProjects(store))
	admin.POST("/cache/clear/ai-site", clearAISite(store))
	admin.POST("/cache/clear/ai", clearAI(store))
	admin.POST("/cache/refresh/all", refreshAll(githubService, store))
	admin.POST("/ai/project-summary", projectSummaryDebug(aiService, store))
	admin.GET("/env", getEnvFile)
	admin.PUT("/env", saveEnvFile)
	admin.GET("/featured", featuredProjects(githubService))
	admin.POST("/featured", saveFeaturedProjects(githubService, store))
	admin.GET("/hidden-projects", hiddenProjects(githubService))
	admin.POST("/hidden-projects", saveHiddenProjects(githubService, store))
}

func login(cfg config.Config) gin.HandlerFunc {
	limiter := newLoginLimiter(loginAttemptLimit, loginAttemptWindow, time.Now)
	return func(c *gin.Context) {
		if !cfg.AdminAuthReady() {
			adminAuthUnavailable(c)
			return
		}

		clientIP := c.ClientIP()
		if !limiter.allow(clientIP) {
			if retry := limiter.retryAfter(clientIP); retry > 0 {
				c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
			}
			response.Error(c, http.StatusTooManyRequests, "too many login attempts")
			return
		}

		var req struct {
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if !passwordMatches(req.Password, cfg.AdminPassword) {
			limiter.fail(clientIP)
			response.Error(c, http.StatusUnauthorized, "invalid password")
			return
		}

		limiter.reset(clientIP)
		token, err := middleware.NewToken(cfg.JWTSecret, cfg.AdminPassword, adminTokenTTL)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		csrf, err := middleware.NewCSRFToken()
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		setAdminCookies(c, cfg, token, csrf)
		response.OK(c, gin.H{"authenticated": true})
	}
}

func logout(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearAdminCookies(c, cfg)
		response.OK(c, gin.H{"authenticated": false})
	}
}

func adminSession(c *gin.Context) {
	response.OK(c, gin.H{"authenticated": true})
}

func adminAuthUnavailable(c *gin.Context) {
	c.Abort()
	response.Error(c, http.StatusServiceUnavailable, "admin auth is not configured; set ADMIN_PASSWORD and JWT_SECRET")
}

func setAdminCookies(c *gin.Context, cfg config.Config, token string, csrf string) {
	expires := time.Now().Add(adminTokenTTL)
	maxAge := int(adminTokenTTL.Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminTokenCookieName,
		Value:    token,
		Path:     "/api/admin",
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: true,
		Secure:   cfg.AdminCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminCSRFCookieName,
		Value:    csrf,
		Path:     "/",
		MaxAge:   maxAge,
		Expires:  expires,
		HttpOnly: false,
		Secure:   cfg.AdminCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearAdminCookies(c *gin.Context, cfg config.Config) {
	expires := time.Unix(1, 0)
	for _, cookie := range []*http.Cookie{
		{Name: middleware.AdminTokenCookieName, Path: "/api/admin", HttpOnly: true},
		{Name: middleware.AdminCSRFCookieName, Path: "/", HttpOnly: false},
	} {
		cookie.Value = ""
		cookie.MaxAge = -1
		cookie.Expires = expires
		cookie.Secure = cfg.AdminCookieSecure
		cookie.SameSite = http.SameSiteStrictMode
		http.SetCookie(c.Writer, cookie)
	}
}

func passwordMatches(input string, expected string) bool {
	inputHash := sha256.Sum256([]byte(input))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(inputHash[:], expectedHash[:]) == 1
}

func cacheRecords(store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var records []model.CacheOperationRecord
		if err := store.ListJSON(c.Request.Context(), cacheOperationRecordKey, &records, cacheOperationRecordLimit); err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		if records == nil {
			records = []model.CacheOperationRecord{}
		}
		response.OK(c, records)
	}
}

func refreshRepos(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		runCacheOperation(c, store, "refresh", "github:repos", "刷新项目列表", []string{"github:repos:base:*"}, func() (int64, error) {
			return 0, githubService.RefreshRepos(c.Request.Context())
		})
	}
}

func refreshLanguages(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		runCacheOperation(c, store, "refresh", "github:languages", "刷新语言统计", []string{"github:languages:*"}, func() (int64, error) {
			return 0, githubService.RefreshLanguages(c.Request.Context())
		})
	}
}

func refreshActivity(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		runCacheOperation(c, store, "refresh", "github:activity", "刷新贡献活动", []string{"github:activity:*", "github:activity-detail:*"}, func() (int64, error) {
			return 0, githubService.RefreshActivity(c.Request.Context())
		})
	}
}

func clearRepoDetails(store *cache.Store) gin.HandlerFunc {
	return clearCachePatterns(store, "clear", "github:repo-details", "清理仓库详情与 README", "github:repo:*", "github:readme:*")
}

func clearRepoBrowser(store *cache.Store) gin.HandlerFunc {
	return clearCachePatterns(store, "clear", "github:repo-browser", "清理仓库目录与文件内容", "github:tree:*", "github:file:*", "github:file-content:*")
}

func clearAIProjects(store *cache.Store) gin.HandlerFunc {
	return clearCachePatterns(store, "clear", "ai:project-summary", "清理 AI 项目简介", "ai:summary:*")
}

func clearAISite(store *cache.Store) gin.HandlerFunc {
	return clearCachePatterns(store, "clear", "ai:site-summary", "清理 AI 站点简介", "ai:site-summary:*")
}

func clearAI(store *cache.Store) gin.HandlerFunc {
	return clearCachePatterns(store, "clear", "ai:all", "清理全部 AI 缓存", "ai:summary:*", "ai:site-summary:*")
}

func projectSummaryDebug(aiService *service.AIService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.ProjectSummaryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		result, err := aiService.ProjectSummaryDebug(c.Request.Context(), req.RepoName, req.ForceRefresh)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		_ = store.DeletePattern(c.Request.Context(), "ai:site-summary:*")
		response.OK(c, result)
	}
}

func refreshAll(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		patterns := []string{
			"github:repo:*",
			"github:readme:*",
			"github:tree:*",
			"github:file:*",
			"github:file-content:*",
			"github:activity:*",
			"github:activity-detail:*",
			"ai:summary:*",
			"ai:site-summary:*",
		}
		runCacheOperation(c, store, "refresh", "cache:all", "刷新核心缓存并清理派生缓存", patterns, func() (int64, error) {
			deleted, err := store.DeletePatterns(c.Request.Context(), patterns...)
			if err != nil {
				return deleted, err
			}
			if err := githubService.RefreshRepos(c.Request.Context()); err != nil {
				return deleted, err
			}
			if err := githubService.RefreshLanguages(c.Request.Context()); err != nil {
				return deleted, err
			}
			if err := githubService.RefreshActivity(c.Request.Context()); err != nil {
				return deleted, err
			}
			return deleted, nil
		})
	}
}

func clearCachePatterns(store *cache.Store, operation string, scope string, message string, patterns ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		runCacheOperation(c, store, operation, scope, message, patterns, func() (int64, error) {
			return store.DeletePatterns(c.Request.Context(), patterns...)
		})
	}
}

func runCacheOperation(c *gin.Context, store *cache.Store, operation string, scope string, message string, patterns []string, run func() (int64, error)) {
	started := time.Now().UTC()
	affectedKeys, err := run()
	finished := time.Now().UTC()

	status := "success"
	recordMessage := message
	if err != nil {
		status = "failed"
		recordMessage = err.Error()
	}

	record := model.CacheOperationRecord{
		ID:           fmt.Sprintf("%d", started.UnixNano()),
		Operation:    operation,
		Scope:        scope,
		Status:       status,
		Message:      recordMessage,
		Patterns:     patterns,
		AffectedKeys: affectedKeys,
		StartedAt:    started.Format(time.RFC3339),
		FinishedAt:   finished.Format(time.RFC3339),
		DurationMs:   finished.Sub(started).Milliseconds(),
	}
	if store != nil {
		if recordErr := store.PushJSON(c.Request.Context(), cacheOperationRecordKey, record, cacheOperationRecordLimit); recordErr != nil && err == nil {
			record.Status = "failed"
			record.Message = recordErr.Error()
			err = recordErr
		}
	}

	if err != nil {
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.OK(c, record)
}

func getEnvFile(c *gin.Context) {
	envFile, err := config.ReadDotEnvFile()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, envFile)
}

func saveEnvFile(c *gin.Context) {
	var req struct {
		Entries []config.EnvEntry `json:"entries"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	envFile, err := config.SaveDotEnvFile(req.Entries)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, envFile)
}

func featuredProjects(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, githubService.FeaturedRepos())
	}
}

func saveFeaturedProjects(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		names, ok := bindProjectNameList(c)
		if !ok {
			return
		}

		envFile, err := config.SaveDotEnvValues(map[string]string{"FEATURED_REPOS": strings.Join(names, ",")})
		if err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		githubService.SetFeaturedRepos(names)
		_ = store.DeletePattern(c.Request.Context(), "ai:site-summary:*")
		response.OK(c, gin.H{"saved": true, "projects": names, "path": envFile.Path})
	}
}

func hiddenProjects(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		response.OK(c, githubService.HiddenRepos())
	}
}

func saveHiddenProjects(githubService *service.GitHubService, store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		names, ok := bindProjectNameList(c)
		if !ok {
			return
		}

		envFile, err := config.SaveDotEnvValues(map[string]string{"HIDDEN_REPOS": strings.Join(names, ",")})
		if err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		githubService.SetHiddenRepos(names)
		_, _ = store.DeletePatterns(c.Request.Context(), "github:activity:*", "github:activity-detail:*")
		if err := githubService.ClearRepositories(c.Request.Context(), names); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		_ = store.DeletePattern(c.Request.Context(), "github:languages:*")
		_ = store.DeletePattern(c.Request.Context(), "ai:site-summary:*")
		response.OK(c, gin.H{"saved": true, "projects": names, "path": envFile.Path})
	}
}

func bindProjectNameList(c *gin.Context) ([]string, bool) {
	var req struct {
		Projects []string `json:"projects"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return nil, false
	}

	names := make([]string, 0, len(req.Projects))
	seen := map[string]bool{}
	for _, project := range req.Projects {
		name := strings.TrimSpace(project)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		names = append(names, name)
		seen[key] = true
	}
	return names, true
}
