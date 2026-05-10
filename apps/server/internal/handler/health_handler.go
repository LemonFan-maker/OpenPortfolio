package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/response"
)

func Health(store *cache.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()

		redisStatus := "disabled"
		if store.Enabled() {
			redisStatus = "ok"
			if err := store.Ping(ctx); err != nil {
				redisStatus = "error"
			}
		}

		response.OK(c, gin.H{
			"status": "ok",
			"redis":  redisStatus,
		})
	}
}
