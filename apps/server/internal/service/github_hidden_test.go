package service

import (
	"context"
	"testing"
	"time"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
)

func TestHiddenRepoBlocksDetailAndTree(t *testing.T) {
	cfg := config.Config{GitHubUsername: "tester", RedisAddr: ""}
	store := cache.NewStore(cfg)
	service := NewGitHubService(cfg, store)

	service.SetHiddenRepos([]string{"Demo"})
	if _, err := service.RepoDetail(context.Background(), "demo", false); err == nil {
		t.Fatal("hidden repo detail returned no error")
	}
	if _, err := service.RepoTree(context.Background(), "demo", false); err == nil {
		t.Fatal("hidden repo tree returned no error")
	}
}

func TestClearRepositoriesDeletesDerivedCaches(t *testing.T) {
	cfg := config.Config{GitHubUsername: "tester", RedisAddr: ""}
	store := cache.NewStore(cfg)
	service := NewGitHubService(cfg, store)
	ctx := context.Background()

	keys := []struct {
		key  string
		keep bool
	}{
		{"github:repo:tester:demo", false},
		{"github:readme:tester:demo", false},
		{"github:languages:tester:demo", false},
		{"github:tree:tester:demo:main", false},
		{"github:file:tester:demo:main.go:abc", false},
		{"github:file-content:tester:demo:main.go:abc", false},
		{"ai:summary:tester:demo:abc", false},
		{"github:repo:tester:keep", true},
		{"github:readme:tester:keep", true},
	}
	for _, item := range keys {
		if err := store.SetJSON(ctx, item.key, map[string]string{"value": "cached"}, time.Minute); err != nil {
			t.Fatalf("seed %s: %v", item.key, err)
		}
	}

	if err := service.ClearRepositories(ctx, []string{"demo"}); err != nil {
		t.Fatalf("ClearRepositories: %v", err)
	}

	for _, item := range keys {
		var value map[string]string
		ok, err := store.GetJSON(ctx, item.key, &value)
		if err != nil {
			t.Fatalf("read %s: %v", item.key, err)
		}
		if item.keep && !ok {
			t.Fatalf("%s was deleted but should remain", item.key)
		}
		if !item.keep && ok {
			t.Fatalf("%s remains but should be deleted", item.key)
		}
	}
}
