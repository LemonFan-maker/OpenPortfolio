package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/model"
)

type aiTestEnv struct {
	service *AIService
	calls   *atomic.Int64
	peak    *atomic.Int64
}

func newAITestEnv(t *testing.T, repoNames []string, aiDelay time.Duration) *aiTestEnv {
	t.Helper()

	repos := make([]map[string]any, len(repoNames))
	for index, name := range repoNames {
		repos[index] = testRepoPayload(name)
	}

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users/tester/repos":
			_ = json.NewEncoder(w).Encode(repos)
		case strings.HasSuffix(r.URL.Path, "/languages"):
			_ = json.NewEncoder(w).Encode(map[string]int64{"Go": 100})
		case strings.HasSuffix(r.URL.Path, "/git/trees/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"truncated": false,
				"tree": []map[string]any{
					{"path": "main.go", "type": "blob", "sha": "abc", "size": 12},
				},
			})
		case r.URL.Path == "/graphql":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"user": map[string]any{
						"contributionsCollection": map[string]any{
							"contributionCalendar": map[string]any{
								"totalContributions": 1,
								"weeks":              []any{},
								"months":             []any{},
							},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(github.Close)

	var current, peak atomic.Int64
	calls := &atomic.Int64{}
	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		now := current.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(aiDelay)
		current.Add(-1)

		decision := map[string]any{
			"action": "finish",
			"summary": map[string]any{
				"shortSummary": "项目短摘要",
				"longSummary":  "项目长摘要",
				"highlights":   []string{"亮点"},
				"techTags":     []string{"Go"},
			},
			"title":        "站点标题",
			"subtitle":     "站点副标题",
			"shortSummary": "站点短摘要",
			"longSummary":  "站点长摘要",
			"sections":     []map[string]any{{"title": "章节", "items": []string{"条目"}}},
			"techTags":     []string{"Go"},
		}
		content, err := json.Marshal(decision)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": string(content)},
			}},
		})
	}))
	t.Cleanup(ai.Close)

	cfg := config.Config{
		GitHubUsername: "tester",
		GitHubToken:    "token",
		GitHubAPIBase:  github.URL,
		GitHubGraphQL:  github.URL + "/graphql",
		AIAPIKey:       "key",
		AIBaseURL:      ai.URL,
		AIModel:        "test-model",
		RedisAddr:      "",
	}
	store := cache.NewStore(cfg)
	githubService := NewGitHubService(cfg, store)
	aiService := NewAIService(cfg, store, githubService)
	return &aiTestEnv{service: aiService, calls: calls, peak: &peak}
}

func TestProjectSummarySingleFlight(t *testing.T) {
	env := newAITestEnv(t, []string{"demo"}, 60*time.Millisecond)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			summary, err := env.service.ProjectSummary(context.Background(), "demo", false)
			if err != nil {
				t.Errorf("ProjectSummary: %v", err)
				return
			}
			if summary.ShortSummary == "" {
				t.Errorf("ProjectSummary returned an empty summary")
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := env.calls.Load(); got != 1 {
		t.Fatalf("ai calls = %d, want 1 after single-flight deduplication", got)
	}
}

func TestCollectProjectSummariesBoundedConcurrency(t *testing.T) {
	names := []string{"repo-0", "repo-1", "repo-2", "repo-3", "repo-4", "repo-5"}
	env := newAITestEnv(t, names, 30*time.Millisecond)
	env.service.projectSummaryConcurrency = 2

	projects := make([]model.ProjectCard, len(names))
	for index, name := range names {
		projects[index] = model.ProjectCard{Name: name}
	}

	items, err := env.service.collectProjectSummaries(context.Background(), projects, false)
	if err != nil {
		t.Fatalf("collectProjectSummaries: %v", err)
	}
	if len(items) != len(names) {
		t.Fatalf("items length = %d, want %d", len(items), len(names))
	}
	if got := env.calls.Load(); got != int64(len(names)) {
		t.Fatalf("ai calls = %d, want %d", got, len(names))
	}
	if got := env.peak.Load(); got > 2 {
		t.Fatalf("peak concurrency = %d, want at most 2", got)
	}
	if got := env.peak.Load(); got < 2 {
		t.Fatalf("peak concurrency = %d, want at least 2", got)
	}
	for _, item := range items {
		if item.Name == "" || item.Source == "" {
			t.Fatalf("unexpected summary item: %#v", item)
		}
	}
}

func TestProjectSummaryTimeoutFallsBack(t *testing.T) {
	env := newAITestEnv(t, []string{"demo"}, 300*time.Millisecond)
	env.service.projectSummaryTimeout = 40 * time.Millisecond

	debug, err := env.service.ProjectSummaryDebug(context.Background(), "demo", false)
	if err != nil {
		t.Fatalf("ProjectSummaryDebug: %v", err)
	}
	if debug.Source != "fallback" {
		t.Fatalf("source = %q, want fallback", debug.Source)
	}
	if !strings.Contains(debug.Error, "context deadline exceeded") {
		t.Fatalf("error = %q, want a context deadline", debug.Error)
	}
}

func TestSiteSummaryTimeoutFallsBack(t *testing.T) {
	env := newAITestEnv(t, []string{"demo"}, 300*time.Millisecond)
	env.service.siteSummaryTimeout = 60 * time.Millisecond

	status, err := env.service.SiteSummary(context.Background(), false)
	if err != nil {
		t.Fatalf("SiteSummary: %v", err)
	}
	if status.Source != "fallback" {
		t.Fatalf("source = %q, want fallback", status.Source)
	}
	if !strings.Contains(status.Error, "context deadline exceeded") {
		t.Fatalf("error = %q, want a context deadline", status.Error)
	}
}
