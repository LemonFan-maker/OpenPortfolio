package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"portfolio-hub/apps/server/internal/config"
)

type Store struct {
	client *redis.Client
}

func NewStore(cfg config.Config) *Store {
	if cfg.RedisAddr == "" {
		return &Store{}
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("redis disabled: %v", err)
		return &Store{}
	}

	return &Store{client: client}
}

func (s *Store) Enabled() bool {
	return s != nil && s.client != nil
}

func (s *Store) Ping(ctx context.Context) error {
	if !s.Enabled() {
		return redis.Nil
	}
	return s.client.Ping(ctx).Err()
}

func (s *Store) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	if !s.Enabled() {
		return false, nil
	}

	raw, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, raw, ttl).Err()
}

func (s *Store) DeletePattern(ctx context.Context, pattern string) error {
	if !s.Enabled() {
		return nil
	}

	iter := s.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := s.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
