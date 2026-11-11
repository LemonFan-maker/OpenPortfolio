package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func commitSearchItem(sha, repoName string, private bool) map[string]any {
	return map[string]any{
		"sha":      sha,
		"html_url": "https://github.com/tester/" + repoName + "/commit/" + sha,
		"url":      "",
		"commit": map[string]any{
			"message": "commit " + sha,
			"author":  map[string]any{"date": "2026-01-01T00:00:00Z"},
		},
		"repository": map[string]any{
			"name":      repoName,
			"full_name": "tester/" + repoName,
			"private":   private,
		},
	}
}

func TestCommitActivityDetailFiltersHiddenAndPrivateRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/commits" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 3,
			"items": []map[string]any{
				commitSearchItem("visible", "demo", false),
				commitSearchItem("hidden", "secret", false),
				commitSearchItem("private", "private-repo", true),
			},
		})
	}))
	defer server.Close()

	service := newTestGitHubService(server.URL)
	service.SetHiddenRepos([]string{"Secret"})

	detail, err := service.CommitActivityDetail(context.Background(), "2026-01-01", true)
	if err != nil {
		t.Fatalf("CommitActivityDetail: %v", err)
	}
	if len(detail.Commits) != 1 {
		t.Fatalf("commits = %#v, want only the visible repo", detail.Commits)
	}
	if detail.Commits[0].Repository != "tester/demo" {
		t.Fatalf("repository = %q, want tester/demo", detail.Commits[0].Repository)
	}
	if detail.TotalCommits != 1 {
		t.Fatalf("total commits = %d, want filtered count 1", detail.TotalCommits)
	}
}

func TestCommitActivityDetailFiltersHiddenReposFromCache(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 2,
			"items": []map[string]any{
				commitSearchItem("visible", "demo", false),
				commitSearchItem("hidden", "secret", false),
			},
		})
	}))
	defer server.Close()

	service := newTestGitHubService(server.URL)

	first, err := service.CommitActivityDetail(context.Background(), "2026-01-01", true)
	if err != nil {
		t.Fatalf("first CommitActivityDetail: %v", err)
	}
	if len(first.Commits) != 2 {
		t.Fatalf("first commits = %d, want 2", len(first.Commits))
	}

	service.SetHiddenRepos([]string{"secret"})
	second, err := service.CommitActivityDetail(context.Background(), "2026-01-01", false)
	if err != nil {
		t.Fatalf("second CommitActivityDetail: %v", err)
	}
	if len(second.Commits) != 1 || second.Commits[0].Repository != "tester/demo" {
		t.Fatalf("second commits = %#v, want only tester/demo", second.Commits)
	}
	if calls.Load() != 1 {
		t.Fatalf("search calls = %d, want 1 (second read should use cache)", calls.Load())
	}
}
