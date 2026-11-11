package handler

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/response"
)

const maxPublicAILimiterIPs = 4096

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type publicAILimiter struct {
	mu sync.Mutex

	perIP      map[string]*tokenBucket
	perIPRate  float64
	perIPBurst float64

	global      tokenBucket
	globalRate  float64
	globalBurst float64

	slots chan struct{}
	now   func() time.Time
}

func newPublicAILimiter(perMinute int, globalPerMinute int, maxConcurrency int) *publicAILimiter {
	if perMinute < 1 {
		perMinute = 6
	}
	if globalPerMinute < 1 {
		globalPerMinute = 30
	}
	if maxConcurrency < 1 {
		maxConcurrency = 2
	}

	perIPRate := float64(perMinute) / 60
	globalRate := float64(globalPerMinute) / 60
	now := time.Now()
	return &publicAILimiter{
		perIP:       map[string]*tokenBucket{},
		perIPRate:   perIPRate,
		perIPBurst:  math.Max(1, float64(perMinute)/2),
		global:      tokenBucket{tokens: math.Max(1, float64(globalPerMinute)/2), last: now},
		globalRate:  globalRate,
		globalBurst: math.Max(1, float64(globalPerMinute)/2),
		slots:       make(chan struct{}, maxConcurrency),
		now:         time.Now,
	}
}

func (l *publicAILimiter) allow(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.refill(&l.global, l.globalRate, l.globalBurst, now)

	bucket, ok := l.perIP[ip]
	if !ok {
		bucket = &tokenBucket{tokens: l.perIPBurst, last: now}
		l.perIP[ip] = bucket
		l.cleanupLocked(now)
	}
	l.refill(bucket, l.perIPRate, l.perIPBurst, now)

	retry := maxDuration(bucketRetry(bucket.tokens, l.perIPRate), bucketRetry(l.global.tokens, l.globalRate))
	if bucket.tokens < 1 || l.global.tokens < 1 {
		return false, retry
	}
	bucket.tokens--
	l.global.tokens--
	return true, 0
}

func (l *publicAILimiter) tryAcquireSlot() bool {
	select {
	case l.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *publicAILimiter) releaseSlot() {
	<-l.slots
}

func (l *publicAILimiter) refill(bucket *tokenBucket, rate float64, burst float64, now time.Time) {
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = math.Min(burst, bucket.tokens+elapsed*rate)
		bucket.last = now
	}
}

func (l *publicAILimiter) cleanupLocked(now time.Time) {
	if len(l.perIP) <= maxPublicAILimiterIPs {
		return
	}
	for ip, bucket := range l.perIP {
		if now.Sub(bucket.last) > 10*time.Minute {
			delete(l.perIP, ip)
		}
	}
	if len(l.perIP) > maxPublicAILimiterIPs {
		l.perIP = map[string]*tokenBucket{}
	}
}

func bucketRetry(tokens float64, rate float64) time.Duration {
	if tokens >= 1 {
		return 0
	}
	seconds := (1 - tokens) / rate
	if seconds < 1 {
		seconds = 1
	}
	return time.Duration(seconds * float64(time.Second))
}

func maxDuration(left time.Duration, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

func publicAIRateLimit(limiter *publicAILimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed, retry := limiter.allow(c.ClientIP())
		if !allowed {
			setRetryAfter(c, retry)
			c.Abort()
			response.Error(c, http.StatusTooManyRequests, "ai request rate limit exceeded")
			return
		}
		if !limiter.tryAcquireSlot() {
			setRetryAfter(c, 5*time.Second)
			c.Abort()
			response.Error(c, http.StatusTooManyRequests, "ai service is busy, retry later")
			return
		}
		defer limiter.releaseSlot()
		c.Next()
	}
}

func setRetryAfter(c *gin.Context, retry time.Duration) {
	seconds := int(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
}
