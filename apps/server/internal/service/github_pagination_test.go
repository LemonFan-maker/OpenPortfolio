package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
)

func newTestGitHubService(apiBase string) *GitHubService {
	cfg := config.Config{GitHubUsername: "tester", GitHubAPIBase: apiBase, RedisAddr: ""}
	return NewGitHubService(cfg, cache.NewStore(cfg))
}

func testRepoPayload(name string) map[string]any {
	return map[string]any{
		"id":                1,
		"name":              name,
		"full_name":         "tester/" + name,
		"default_branch":    "main",
		"description":       "demo repository",
		"html_url":          "https://github.com/tester/" + name,
		"language":          "Go",
		"topics":            []string{"go"},
		"stargazers_count":  1,
		"forks_count":       0,
		"open_issues_count": 0,
		"fork":              false,
		"archived":          false,
		"private":           false,
		"updated_at":        "2026-01-01T00:00:00Z",
	}
}

func TestListRawReposFollowsPagination(t *testing.T) {
	var mu sync.Mutex
	var pages []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/tester/repos" {
			http.NotFound(w, r)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		mu.Lock()
		pages = append(pages, strconv.Itoa(page))
		mu.Unlock()

		repos := make([]map[string]any, 0, repoPageSize)
		switch page {
		case 1:
			for i := range repoPageSize {
				repos = append(repos, testRepoPayload(fmt.Sprintf("repo-%d", i)))
			}
		case 2:
			repos = append(repos, testRepoPayload("repo-100"))
		}
		_ = json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	service := newTestGitHubService(server.URL)
	repos, err := service.listRawRepos(context.Background(), false)
	if err != nil {
		t.Fatalf("listRawRepos: %v", err)
	}
	if len(repos) != repoPageSize+1 {
		t.Fatalf("repos length = %d, want %d", len(repos), repoPageSize+1)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("requested pages = %#v, want [1 2]", pages)
	}
}

func TestListReposFetchesLanguagesWithBoundedConcurrency(t *testing.T) {
	var current, peak, requests atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/tester/repos":
			repos := make([]map[string]any, 24)
			for i := range repos {
				repos[i] = testRepoPayload(fmt.Sprintf("repo-%d", i))
			}
			_ = json.NewEncoder(w).Encode(repos)
		case strings.HasSuffix(r.URL.Path, "/languages"):
			requests.Add(1)
			now := current.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			current.Add(-1)
			_ = json.NewEncoder(w).Encode(map[string]int64{"Go": 100})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newTestGitHubService(server.URL)
	cards, err := service.ListRepos(context.Background(), "stars", "", "", false)
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(cards) != 24 {
		t.Fatalf("cards length = %d, want 24", len(cards))
	}
	if got := requests.Load(); got != 24 {
		t.Fatalf("language requests = %d, want 24", got)
	}
	if got := peak.Load(); got > int64(repoLanguageConcurrency) {
		t.Fatalf("peak concurrency = %d, want at most %d", got, repoLanguageConcurrency)
	}
	if got := peak.Load(); got < 2 {
		t.Fatalf("peak concurrency = %d, want at least 2", got)
	}
	for _, card := range cards {
		if len(card.Languages) != 1 || card.Languages[0].Name != "Go" {
			t.Fatalf("card %q languages = %#v, want Go", card.Name, card.Languages)
		}
	}
}
