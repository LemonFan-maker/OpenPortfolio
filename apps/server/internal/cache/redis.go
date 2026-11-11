package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"portfolio-hub/apps/server/internal/config"
)

const redisDegradedTTL = 30 * time.Second

var errRedisDegraded = errors.New("redis circuit is open")

type Store struct {
	client *redis.Client
	memory *memoryStore

	mu             sync.RWMutex
	degradedUntil  time.Time
	pendingDeletes []string
	now            func() time.Time
}

type memoryItem struct {
	raw       []byte
	expiresAt time.Time
}

type memoryList struct {
	items [][]byte
}

type memoryStore struct {
	mu    sync.RWMutex
	items map[string]memoryItem
	lists map[string]memoryList
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		items: map[string]memoryItem{},
		lists: map[string]memoryList{},
	}
}

func NewStore(cfg config.Config) *Store {
	store := &Store{memory: newMemoryStore(), now: time.Now}
	if cfg.RedisAddr == "" {
		return store
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		MaxRetries:   1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	store.client = client
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("redis unavailable; using memory fallback for %s: %v", redisDegradedTTL, err)
		store.markDegraded()
	}

	return store
}

func (s *Store) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *Store) Degraded() bool {
	if s == nil || s.client == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTime().Before(s.degradedUntil)
}

func (s *Store) Fallback() bool {
	return s != nil && s.memory != nil && !s.useRedis()
}

func (s *Store) Ping(ctx context.Context) error {
	if !s.Enabled() {
		return redis.Nil
	}
	if s.Degraded() {
		return errRedisDegraded
	}
	if err := s.client.Ping(ctx).Err(); err != nil {
		s.markDegraded()
		return err
	}
	s.markHealthy()
	return nil
}

func (s *Store) useRedis() bool {
	if s == nil || s.client == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.currentTime().Before(s.degradedUntil)
}

func (s *Store) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Store) markDegraded() {
	s.mu.Lock()
	wasDegraded := s.currentTime().Before(s.degradedUntil)
	s.degradedUntil = s.currentTime().Add(redisDegradedTTL)
	s.mu.Unlock()
	if !wasDegraded {
		log.Printf("redis degraded; using memory fallback for %s", redisDegradedTTL)
	}
}

func (s *Store) markHealthy() {
	s.mu.Lock()
	s.degradedUntil = time.Time{}
	pending := s.pendingDeletes
	s.pendingDeletes = nil
	s.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, pattern := range pending {
		if _, err := s.deleteRedisPattern(ctx, pattern); err != nil {
			s.recordPendingDelete(pattern)
		}
	}
}

func (s *Store) recordPendingDelete(pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.pendingDeletes {
		if existing == pattern {
			return
		}
	}
	if len(s.pendingDeletes) >= 64 {
		s.pendingDeletes = s.pendingDeletes[1:]
	}
	s.pendingDeletes = append(s.pendingDeletes, pattern)
}

func (s *Store) deleteRedisPattern(ctx context.Context, pattern string) (int64, error) {
	var deleted int64
	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		count, err := s.client.Del(ctx, iter.Val()).Result()
		if err != nil {
			return deleted, err
		}
		deleted += count
	}
	return deleted, iter.Err()
}

func (s *Store) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	if !s.useRedis() {
		return s.getMemoryJSON(key, target)
	}

	raw, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		s.markHealthy()
		return false, nil
	}
	if err != nil {
		s.markDegraded()
		return s.getMemoryJSON(key, target)
	}
	s.markHealthy()

	if err := json.Unmarshal(raw, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !s.useRedis() {
		return s.setMemoryJSON(key, value, ttl)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, key, raw, ttl).Err(); err != nil {
		s.markDegraded()
		return s.setMemoryJSON(key, value, ttl)
	}
	s.markHealthy()
	return nil
}

func (s *Store) DeletePattern(ctx context.Context, pattern string) error {
	_, err := s.DeletePatternCount(ctx, pattern)
	return err
}

func (s *Store) DeletePatternCount(ctx context.Context, pattern string) (int64, error) {
	if !s.Enabled() {
		return s.deleteMemoryPattern(pattern), nil
	}
	if !s.useRedis() {
		s.recordPendingDelete(pattern)
		return s.deleteMemoryPattern(pattern), nil
	}

	deleted, err := s.deleteRedisPattern(ctx, pattern)
	if err != nil {
		s.markDegraded()
		s.recordPendingDelete(pattern)
		return s.deleteMemoryPattern(pattern), nil
	}
	s.markHealthy()
	return deleted, nil
}

func (s *Store) DeletePatterns(ctx context.Context, patterns ...string) (int64, error) {
	var total int64
	for _, pattern := range patterns {
		deleted, err := s.DeletePatternCount(ctx, pattern)
		if err != nil {
			return total, err
		}
		total += deleted
	}
	return total, nil
}

func (s *Store) PushJSON(ctx context.Context, key string, value any, limit int64) error {
	if !s.useRedis() {
		return s.pushMemoryJSON(key, value, limit)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.client.LPush(ctx, key, raw).Err(); err != nil {
		s.markDegraded()
		return s.pushMemoryJSON(key, value, limit)
	}
	if limit > 0 {
		if err := s.client.LTrim(ctx, key, 0, limit-1).Err(); err != nil {
			s.markDegraded()
			return nil
		}
	}
	s.markHealthy()
	return nil
}

func (s *Store) ListJSON(ctx context.Context, key string, target any, limit int64) error {
	if !s.useRedis() {
		return s.listMemoryJSON(key, target, limit)
	}

	stop := int64(-1)
	if limit > 0 {
		stop = limit - 1
	}
	rawItems, err := s.client.LRange(ctx, key, 0, stop).Result()
	if err != nil {
		s.markDegraded()
		return s.listMemoryJSON(key, target, limit)
	}
	s.markHealthy()

	raw := []byte("[" + strings.Join(rawItems, ",") + "]")
	return json.Unmarshal(raw, target)
}

func (s *Store) getMemoryJSON(key string, target any) (bool, error) {
	if s == nil || s.memory == nil {
		return false, nil
	}

	s.memory.mu.Lock()
	defer s.memory.mu.Unlock()

	item, ok := s.memory.items[key]
	if !ok {
		return false, nil
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		delete(s.memory.items, key)
		return false, nil
	}
	if err := json.Unmarshal(item.raw, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) setMemoryJSON(key string, value any, ttl time.Duration) error {
	if s == nil || s.memory == nil {
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	s.memory.mu.Lock()
	defer s.memory.mu.Unlock()
	s.memory.items[key] = memoryItem{raw: raw, expiresAt: expiresAt}
	return nil
}

func (s *Store) deleteMemoryPattern(pattern string) int64 {
	if s == nil || s.memory == nil {
		return 0
	}

	s.memory.mu.Lock()
	defer s.memory.mu.Unlock()

	var deleted int64
	for key := range s.memory.items {
		if memoryPatternMatch(pattern, key) {
			delete(s.memory.items, key)
			deleted++
		}
	}
	for key := range s.memory.lists {
		if memoryPatternMatch(pattern, key) {
			delete(s.memory.lists, key)
			deleted++
		}
	}
	return deleted
}

func (s *Store) pushMemoryJSON(key string, value any, limit int64) error {
	if s == nil || s.memory == nil {
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	s.memory.mu.Lock()
	defer s.memory.mu.Unlock()

	list := s.memory.lists[key]
	list.items = append([][]byte{raw}, list.items...)
	if limit > 0 && int64(len(list.items)) > limit {
		list.items = list.items[:limit]
	}
	s.memory.lists[key] = list
	return nil
}

func (s *Store) listMemoryJSON(key string, target any, limit int64) error {
	if s == nil || s.memory == nil {
		return nil
	}

	s.memory.mu.RLock()
	list := s.memory.lists[key]
	items := append([][]byte(nil), list.items...)
	s.memory.mu.RUnlock()

	if limit > 0 && int64(len(items)) > limit {
		items = items[:limit]
	}

	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, string(item))
	}
	raw := []byte("[" + strings.Join(parts, ",") + "]")
	return json.Unmarshal(raw, target)
}

func memoryPatternMatch(pattern string, key string) bool {
	matched, err := path.Match(pattern, key)
	return err == nil && matched
}
