package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/model"
	"portfolio-hub/apps/server/internal/response"
	"portfolio-hub/apps/server/internal/service"
)

func RegisterAIRoutes(router *gin.Engine, aiService *service.AIService, cfg config.Config) {
	limiter := newPublicAILimiter(cfg.AIPublicPerMinute, cfg.AIPublicGlobalPerMinute, cfg.AIPublicMaxConcurrency)
	router.POST("/api/ai/project-summary", publicAIRateLimit(limiter), projectSummaryStatus(aiService))
	router.POST("/api/ai/site-summary", publicAIRateLimit(limiter), siteSummary(aiService))
}

func projectSummaryStatus(aiService *service.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.ProjectSummaryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		// Public requests must never bypass the cache; only the admin debug
		// endpoint is allowed to force regeneration.
		summary, err := aiService.ProjectSummaryStatus(c.Request.Context(), req.RepoName, false)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, summary)
	}
}

func siteSummary(aiService *service.AIService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req model.SiteSummaryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}

		// Public requests must never bypass the cache; only the admin debug
		// endpoint is allowed to force regeneration.
		summary, err := aiService.SiteSummary(c.Request.Context(), false)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, summary)
	}
}
