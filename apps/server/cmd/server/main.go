package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/handler"
	"portfolio-hub/apps/server/internal/middleware"
)

func main() {
	cfg := config.Load()

	redisStore := cache.NewStore(cfg)
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.WebOrigin))

	router.GET("/api/health", handler.Health(redisStore))

	githubService := handler.NewGitHubService(cfg, redisStore)
	aiService := handler.NewAIService(cfg, redisStore, githubService)

	handler.RegisterGitHubRoutes(router, githubService)
	handler.RegisterAIRoutes(router, aiService)
	handler.RegisterAdminRoutes(router, cfg, redisStore, githubService, aiService)

	addr := ":" + cfg.ServerPort
	log.Printf("server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatal(err)
	}
}
