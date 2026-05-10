package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/response"
	"portfolio-hub/apps/server/internal/service"
)

func RegisterGitHubRoutes(router *gin.Engine, githubService *service.GitHubService) {
	group := router.Group("/api/github")
	group.GET("/repos", listRepos(githubService))
	group.GET("/repos/:name", repoDetail(githubService))
	group.GET("/languages", languageStats(githubService))
	group.GET("/activity", commitActivity(githubService))
	group.GET("/activity/:date", commitActivityDetail(githubService))
}

func listRepos(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		repos, err := githubService.ListRepos(
			c.Request.Context(),
			c.Query("sort"),
			c.Query("language"),
			c.Query("keyword"),
			false,
		)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, repos)
	}
}

func repoDetail(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		detail, err := githubService.RepoDetail(c.Request.Context(), c.Param("name"), false)
		if err != nil {
			response.Error(c, http.StatusNotFound, err.Error())
			return
		}
		response.OK(c, detail)
	}
}

func languageStats(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := githubService.LanguageStats(c.Request.Context(), false)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, stats)
	}
}

func commitActivity(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		activity, err := githubService.CommitActivity(c.Request.Context(), false)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, activity)
	}
}

func commitActivityDetail(githubService *service.GitHubService) gin.HandlerFunc {
	return func(c *gin.Context) {
		detail, err := githubService.CommitActivityDetail(c.Request.Context(), c.Param("date"), false)
		if err != nil {
			response.Error(c, http.StatusBadGateway, err.Error())
			return
		}
		response.OK(c, detail)
	}
}
