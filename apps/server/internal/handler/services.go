package handler

import (
	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/service"
)

func NewGitHubService(cfg config.Config, store *cache.Store) *service.GitHubService {
	return service.NewGitHubService(cfg, store)
}

func NewAIService(cfg config.Config, store *cache.Store, github *service.GitHubService) *service.AIService {
	return service.NewAIService(cfg, store, github)
}
