package handler

import (
	"net/http"
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

func RegisterAdminRoutes(router *gin.Engine, cfg config.Config, store *cache.Store, githubService *service.GitHubService, aiService *service.AIService) {
	router.POST("/api/admin/login", login(cfg))

	admin := router.Group("/api/admin")
	admin.Use(middleware.RequireAuth(cfg.JWTSecret))
	admin.POST("/cache/refresh/repos", refreshRepos(githubService))
	admin.POST("/cache/refresh/languages", refreshLanguages(githubService))
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
	return func(c *gin.Context) {
		var req struct {
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if req.Password != cfg.AdminPassword {
			response.Error(c, http.StatusUnauthorized, "invalid password")
			return
		}
		token, err := middleware.NewToken(cfg.JWTSecret, 24*time.Hour)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.OK(c, gin.H{"token": token})
	}
}

func refreshRepos(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := githubService.RefreshRepos(c.Request.Context()); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, gin.H{"refreshed": "repos"})
	}
}

func refreshLanguages(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := githubService.RefreshLanguages(c.Request.Context()); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, gin.H{"refreshed": "languages"})
	}
}

func clearAI(store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.DeletePattern(c.Request.Context(), "ai:summary:*"); err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		_ = store.DeletePattern(c.Request.Context(), "ai:site-summary:*")
		response.OK(c, gin.H{"cleared": "ai"})
	}
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
		if err := githubService.RefreshRepos(c.Request.Context()); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		if err := githubService.RefreshLanguages(c.Request.Context()); err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		_ = store.DeletePattern(c.Request.Context(), "ai:summary:*")
		_ = store.DeletePattern(c.Request.Context(), "ai:site-summary:*")
		response.OK(c, gin.H{"refreshed": "all"})
	}
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
