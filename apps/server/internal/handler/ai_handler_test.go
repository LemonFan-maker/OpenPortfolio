package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/model"
)

func newAIStubRouter(t *testing.T) (*gin.Engine, *atomic.Int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var aiCalls atomic.Int64

	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/tester/repos":
			_, _ = w.Write([]byte(`[{
				"id": 1,
				"name": "demo",
				"full_name": "tester/demo",
				"default_branch": "main",
				"description": "demo repository",
				"html_url": "https://github.com/tester/demo",
				"language": "Go",
				"topics": ["go"],
				"stargazers_count": 3,
				"forks_count": 1,
				"open_issues_count": 0,
				"fork": false,
				"archived": false,
				"private": false,
				"updated_at": "2026-01-01T00:00:00Z"
			}]`))
		case "/repos/tester/demo/languages":
			_, _ = w.Write([]byte(`{"Go": 100}`))
		case "/repos/tester/demo/git/trees/main":
			_, _ = w.Write([]byte(`{"truncated":false,"tree":[{"path":"main.go","type":"blob","sha":"abc","size":12}]}`))
		case "/graphql":
			_, _ = w.Write([]byte(`{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":1,"weeks":[],"months":[]}}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(github.Close)

	ai := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		aiCalls.Add(1)

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
			t.Errorf("marshal ai decision: %v", err)
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

	router := gin.New()
	RegisterAIRoutes(router, aiService)
	return router, &aiCalls
}

func postJSON(t *testing.T, router http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestPublicProjectSummaryIgnoresForceRefresh(t *testing.T) {
	router, aiCalls := newAIStubRouter(t)

	first := postJSON(t, router, "/api/ai/project-summary", `{"repoName":"demo","forceRefresh":true}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if got := aiCalls.Load(); got != 1 {
		t.Fatalf("ai calls after first request = %d, want 1", got)
	}
	var firstBody struct {
		Success bool                       `json:"success"`
		Data    model.ProjectSummaryStatus `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if !firstBody.Success || firstBody.Data.Source != "generated" {
		t.Fatalf("first response = %#v, want generated", firstBody)
	}

	second := postJSON(t, router, "/api/ai/project-summary", `{"repoName":"demo","forceRefresh":true}`)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if got := aiCalls.Load(); got != 1 {
		t.Fatalf("ai calls after forced public request = %d, want 1 (force must be ignored)", got)
	}
	var secondBody struct {
		Success bool                       `json:"success"`
		Data    model.ProjectSummaryStatus `json:"data"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if secondBody.Data.Source != "cache" {
		t.Fatalf("second source = %q, want cache", secondBody.Data.Source)
	}
}

func TestPublicSiteSummaryIgnoresForceRefresh(t *testing.T) {
	router, aiCalls := newAIStubRouter(t)

	first := postJSON(t, router, "/api/ai/site-summary", `{"forceRefresh":true}`)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if got := aiCalls.Load(); got != 2 {
		t.Fatalf("ai calls after first request = %d, want 2 (project + site)", got)
	}

	second := postJSON(t, router, "/api/ai/site-summary", `{"forceRefresh":true}`)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if got := aiCalls.Load(); got != 2 {
		t.Fatalf("ai calls after forced public request = %d, want 2 (force must be ignored)", got)
	}
	var secondBody struct {
		Success bool                    `json:"success"`
		Data    model.SiteSummaryStatus `json:"data"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	if secondBody.Data.Source != "cache" {
		t.Fatalf("second source = %q, want cache", secondBody.Data.Source)
	}
}
