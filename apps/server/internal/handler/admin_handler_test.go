package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/service"
)

func TestSaveHiddenProjectsClearsRepoCaches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	envDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(envDir, ".env"), []byte("HIDDEN_REPOS=\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.Chdir(envDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	t.Setenv("HIDDEN_REPOS", "")

	cfg := config.Config{GitHubUsername: "tester", RedisAddr: ""}
	store := cache.NewStore(cfg)
	githubService := service.NewGitHubService(cfg, store)
	ctx := context.Background()

	if err := store.SetJSON(ctx, "github:repo:tester:demo", map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("seed repo cache: %v", err)
	}
	if err := store.SetJSON(ctx, "github:readme:tester:demo", map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("seed readme cache: %v", err)
	}
	if err := store.SetJSON(ctx, "github:activity:tester", map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("seed activity cache: %v", err)
	}
	if err := store.SetJSON(ctx, "github:activity-detail:tester:2026-01-01", map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("seed activity detail cache: %v", err)
	}
	if err := store.SetJSON(ctx, "github:repo:tester:keep", map[string]string{"value": "cached"}, time.Minute); err != nil {
		t.Fatalf("seed keep cache: %v", err)
	}

	router := gin.New()
	router.POST("/api/admin/hidden-projects", saveHiddenProjects(githubService, store))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/hidden-projects", strings.NewReader(`{"projects":["demo"]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	for _, key := range []string{"github:repo:tester:demo", "github:readme:tester:demo", "github:activity:tester", "github:activity-detail:tester:2026-01-01"} {
		var value map[string]string
		ok, err := store.GetJSON(ctx, key, &value)
		if err != nil {
			t.Fatalf("read %s: %v", key, err)
		}
		if ok {
			t.Fatalf("%s remains after hiding the repo", key)
		}
	}

	var value map[string]string
	ok, err := store.GetJSON(ctx, "github:repo:tester:keep", &value)
	if err != nil {
		t.Fatalf("read keep cache: %v", err)
	}
	if !ok {
		t.Fatal("unrelated repo cache was removed")
	}

	if repos := githubService.HiddenRepos(); len(repos) != 1 || repos[0] != "demo" {
		t.Fatalf("hidden repos = %#v, want [demo]", repos)
	}
	if _, err := githubService.RepoDetail(ctx, "demo", false); err == nil {
		t.Fatal("hidden repo detail returned no error")
	}
}
