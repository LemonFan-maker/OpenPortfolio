package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPublicAILimiterLimitsPerIPAndRefills(t *testing.T) {
	limiter := newPublicAILimiter(2, 100, 2)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	limiter.global.last = now

	if allowed, _ := limiter.allow("203.0.113.1"); !allowed {
		t.Fatal("first request should be allowed")
	}
	if allowed, retry := limiter.allow("203.0.113.1"); allowed || retry <= 0 {
		t.Fatalf("second request allowed=%t retry=%v, want blocked with retry", allowed, retry)
	}
	if allowed, _ := limiter.allow("203.0.113.2"); !allowed {
		t.Fatal("a different IP should be allowed")
	}

	now = now.Add(31 * time.Second)
	if allowed, _ := limiter.allow("203.0.113.1"); !allowed {
		t.Fatal("IP should be allowed after the bucket refills")
	}
}

func TestPublicAILimiterLimitsGlobally(t *testing.T) {
	limiter := newPublicAILimiter(100, 2, 2)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	limiter.global.last = now

	if allowed, _ := limiter.allow("203.0.113.1"); !allowed {
		t.Fatal("first global request should be allowed")
	}
	if allowed, retry := limiter.allow("203.0.113.2"); allowed || retry <= 0 {
		t.Fatalf("second global request allowed=%t retry=%v, want blocked", allowed, retry)
	}
	now = now.Add(31 * time.Second)
	if allowed, _ := limiter.allow("203.0.113.2"); !allowed {
		t.Fatal("global bucket should refill")
	}
}

func TestPublicAIRateLimitMiddlewareReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newPublicAILimiter(1, 10, 2)
	router := gin.New()
	router.POST("/test", publicAIRateLimit(limiter), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/test", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusOK)
	}

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/test", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response has no Retry-After header")
	}
}

func TestPublicAIRateLimitMiddlewareRejectsBusyService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := newPublicAILimiter(100, 100, 1)
	limiter.slots <- struct{}{}
	defer limiter.releaseSlot()

	router := gin.New()
	router.POST("/test", publicAIRateLimit(limiter), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/test", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("busy status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}
