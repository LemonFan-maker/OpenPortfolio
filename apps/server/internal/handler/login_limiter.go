package handler

import (
	"sync"
	"time"
)

const (
	loginAttemptLimit  = 5
	loginAttemptWindow = 5 * time.Minute
	maxLoginAttempts   = 4096
)

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	limit    int
	window   time.Duration
	now      func() time.Time
}

type loginAttempt struct {
	count int
	reset time.Time
}

func newLoginLimiter(limit int, window time.Duration, now func() time.Time) *loginLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &loginLimiter{
		attempts: map[string]loginAttempt{},
		limit:    limit,
		window:   window,
		now:      now,
	}
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if !ok {
		return true
	}
	if !l.now().Before(attempt.reset) {
		delete(l.attempts, key)
		return true
	}
	return attempt.count < l.limit
}

func (l *loginLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	attempt, ok := l.attempts[key]
	if !ok || !now.Before(attempt.reset) {
		attempt = loginAttempt{reset: now.Add(l.window)}
	}
	attempt.count++
	l.attempts[key] = attempt
	l.cleanupLocked(now)
}

func (l *loginLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func (l *loginLimiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, ok := l.attempts[key]
	if !ok {
		return 0
	}
	now := l.now()
	if !now.Before(attempt.reset) {
		return 0
	}
	return attempt.reset.Sub(now)
}

func (l *loginLimiter) cleanupLocked(now time.Time) {
	if len(l.attempts) < 1024 {
		return
	}
	for key, attempt := range l.attempts {
		if !now.Before(attempt.reset) {
			delete(l.attempts, key)
		}
	}
	if len(l.attempts) < maxLoginAttempts {
		return
	}
	for key := range l.attempts {
		delete(l.attempts, key)
		break
	}
}
