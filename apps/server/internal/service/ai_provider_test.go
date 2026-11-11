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

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
)

type capturedAIRequest struct {
	path    string
	auth    string
	ua      string
	session string
	model   string
}

func newProviderTestServer(t *testing.T, captured *capturedAIRequest, calls *atomic.Int64) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			calls.Add(1)
		}
		captured.path = r.URL.Path
		captured.auth = r.Header.Get("Authorization")
		captured.ua = r.Header.Get("User-Agent")
		captured.session = r.Header.Get("x-opencode-session")

		var payload struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		captured.model = payload.Model

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": `{"ok":true}`},
			}},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func newProviderTestService(cfg config.Config) *AIService {
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = ""
	}
	return NewAIService(cfg, cache.NewStore(cfg), nil)
}

func TestOpenCodeRequestUsesInferenceSettings(t *testing.T) {
	var captured capturedAIRequest
	server := newProviderTestServer(t, &captured, nil)

	service := newProviderTestService(config.Config{
		AIProvider:      config.AIProviderOpenCode,
		OpenCodeAPIKey:  "oc_sk_test",
		OpenCodeBaseURL: server.URL + "/",
		AIModel:         "deepseek-v4.1-flash",
		AIAPIKey:        "legacy-key",
		AIBaseURL:       "http://legacy.invalid",
	})

	content, raw, err := service.requestAIContent(context.Background(), "session-123", "system", "user", 0.2)
	if err != nil {
		t.Fatalf("requestAIContent: %v", err)
	}
	if content != `{"ok":true}` {
		t.Fatalf("content = %q, want JSON content", content)
	}
	if raw == "" {
		t.Fatal("raw response is empty")
	}
	if captured.path != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", captured.path)
	}
	if captured.auth != "Bearer oc_sk_test" {
		t.Fatalf("authorization = %q, want OpenCode key", captured.auth)
	}
	if !strings.HasPrefix(captured.ua, "portfolio-hub/") {
		t.Fatalf("user agent = %q, want portfolio-hub version", captured.ua)
	}
	if captured.session != "session-123" {
		t.Fatalf("x-opencode-session = %q, want session-123", captured.session)
	}
	if captured.model != "deepseek-v4.1-flash" {
		t.Fatalf("model = %q, want deepseek-v4.1-flash", captured.model)
	}
}

func TestOpenCodeRequestGeneratesSessionWhenMissing(t *testing.T) {
	var captured capturedAIRequest
	server := newProviderTestServer(t, &captured, nil)

	service := newProviderTestService(config.Config{
		AIProvider:      config.AIProviderOpenCode,
		OpenCodeAPIKey:  "oc_sk_test",
		OpenCodeBaseURL: server.URL,
		AIModel:         "deepseek-v4.1-flash",
	})

	if _, _, err := service.requestAIContent(context.Background(), "", "system", "user", 0.2); err != nil {
		t.Fatalf("requestAIContent: %v", err)
	}
	if captured.session == "" {
		t.Fatal("x-opencode-session is empty, want generated session id")
	}
}

func TestOpenAIProviderRequestKeepsLegacySettings(t *testing.T) {
	var captured capturedAIRequest
	server := newProviderTestServer(t, &captured, nil)

	service := newProviderTestService(config.Config{
		AIProvider:     config.AIProviderOpenAI,
		AIAPIKey:       "sk-legacy",
		AIBaseURL:      server.URL,
		AIModel:        "gpt-4o-mini",
		OpenCodeAPIKey: "oc_sk_should_not_be_used",
	})

	if _, _, err := service.requestAIContent(context.Background(), "session-123", "system", "user", 0.2); err != nil {
		t.Fatalf("requestAIContent: %v", err)
	}
	if captured.auth != "Bearer sk-legacy" {
		t.Fatalf("authorization = %q, want legacy OpenAI key", captured.auth)
	}
	if captured.session != "" {
		t.Fatalf("x-opencode-session = %q, want empty for OpenAI", captured.session)
	}
	if captured.model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", captured.model)
	}
}

func TestAIProviderDoesNotFallbackAcrossProviders(t *testing.T) {
	var captured capturedAIRequest
	var calls atomic.Int64
	server := newProviderTestServer(t, &captured, &calls)

	service := newProviderTestService(config.Config{
		AIProvider:     config.AIProviderOpenCode,
		OpenCodeAPIKey: "",
		AIAPIKey:       "sk-legacy",
		AIBaseURL:      server.URL,
		AIModel:        "deepseek-v4.1-flash",
	})

	_, _, err := service.requestAIContent(context.Background(), "session-123", "system", "user", 0.2)
	if err == nil || !strings.Contains(err.Error(), "opencode api key") {
		t.Fatalf("error = %v, want missing OpenCode key error", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("provider server calls = %d, want 0", calls.Load())
	}
}

func TestAIProviderUnsetUsesLegacyOpenAIWhenNoOpenCodeKey(t *testing.T) {
	var captured capturedAIRequest
	server := newProviderTestServer(t, &captured, nil)

	service := newProviderTestService(config.Config{
		AIAPIKey:  "sk-legacy",
		AIBaseURL: server.URL,
		AIModel:   "gpt-4o-mini",
	})

	if _, _, err := service.requestAIContent(context.Background(), "", "system", "user", 0.2); err != nil {
		t.Fatalf("requestAIContent: %v", err)
	}
	if captured.auth != "Bearer sk-legacy" {
		t.Fatalf("authorization = %q, want legacy OpenAI key", captured.auth)
	}
}

func TestProjectSummaryReusesOpenCodeSessionAcrossTurns(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users/tester/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":                1,
				"name":              "demo",
				"full_name":         "tester/demo",
				"default_branch":    "main",
				"description":       "demo repository",
				"html_url":          "https://github.com/tester/demo",
				"language":          "Go",
				"topics":            []string{"go"},
				"stargazers_count":  3,
				"forks_count":       1,
				"open_issues_count": 0,
				"fork":              false,
				"archived":          false,
				"private":           false,
				"updated_at":        "2026-01-01T00:00:00Z",
			}})
		case strings.HasSuffix(r.URL.Path, "/languages"):
			_ = json.NewEncoder(w).Encode(map[string]int64{"Go": 100})
		case strings.Contains(r.URL.Path, "/git/trees/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"truncated": false,
				"tree": []map[string]any{
					{"path": "main.go", "type": "blob", "sha": "abc", "size": 12},
				},
			})
		case r.URL.Path == "/graphql":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(github.Close)

	var mu sync.Mutex
	var sessions []string
	openCode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sessions = append(sessions, r.Header.Get("x-opencode-session"))
		call := len(sessions)
		mu.Unlock()

		content := json.RawMessage(`{"action":"list_dir","path":""}`)
		if call > 1 {
			content = json.RawMessage(`{"action":"finish","summary":{"shortSummary":"短摘要","longSummary":"长摘要","highlights":["亮点"],"techTags":["Go"]}}`)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": string(content)},
			}},
		})
	}))
	t.Cleanup(openCode.Close)

	cfg := config.Config{
		GitHubUsername:  "tester",
		GitHubToken:     "token",
		GitHubAPIBase:   github.URL,
		GitHubGraphQL:   github.URL + "/graphql",
		AIProvider:      config.AIProviderOpenCode,
		OpenCodeAPIKey:  "oc_sk_test",
		OpenCodeBaseURL: openCode.URL,
		AIModel:         "test-model",
		RedisAddr:       "",
	}
	store := cache.NewStore(cfg)
	service := NewAIService(cfg, store, NewGitHubService(cfg, store))

	summary, err := service.ProjectSummary(context.Background(), "demo", true)
	if err != nil {
		t.Fatalf("ProjectSummary: %v", err)
	}
	if summary.ShortSummary == "" {
		t.Fatal("summary is empty")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sessions) != 2 {
		t.Fatalf("ai calls = %d, want 2", len(sessions))
	}
	if sessions[0] == "" || sessions[1] == "" {
		t.Fatalf("sessions = %#v, want non-empty ids", sessions)
	}
	if sessions[0] != sessions[1] {
		t.Fatalf("sessions = %#v, want one stable conversation id", sessions)
	}
}
