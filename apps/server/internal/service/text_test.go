package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	cases := []struct {
		value string
		max   int
		want  string
	}{
		{"abc", 2, "ab"},
		{"abc", 5, "abc"},
		{"你好世界", 0, ""},
		{"你好世界", 1, ""},
		{"你好世界", 3, "你"},
		{"你好世界", 4, "你"},
		{"你好世界", 6, "你好"},
		{"a界b", 2, "a"},
		{"a界b", 3, "a"},
		{"a界b", 4, "a界"},
	}

	for _, tc := range cases {
		got := truncateUTF8(tc.value, tc.max)
		if got != tc.want {
			t.Errorf("truncateUTF8(%q, %d) = %q, want %q", tc.value, tc.max, got, tc.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncateUTF8(%q, %d) is not valid UTF-8: %q", tc.value, tc.max, got)
		}
	}
}

func TestTextTruncationKeepsValidUTF8(t *testing.T) {
	value := "项目简介"

	if got, want := previewText(value, 7), "项目..."; got != want {
		t.Errorf("previewText = %q, want %q", got, want)
	}
	if got, want := summarizeReadme(value, 7), "项目..."; got != want {
		t.Errorf("summarizeReadme = %q, want %q", got, want)
	}
	patchInput := strings.Repeat("项", 467)
	patchWant := strings.Repeat("项", 466) + "\n..."
	if got := truncatePatch(patchInput); got != patchWant {
		t.Errorf("truncatePatch = %q, want %q", got, patchWant)
	}
	if got, want := trim("项目", 4), "项"; got != want {
		t.Errorf("trim = %q, want %q", got, want)
	}
}

func TestRepoFileTruncationKeepsValidUTF8(t *testing.T) {
	content := "a" + strings.Repeat("你", 4000)
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users/tester/repos":
			_ = json.NewEncoder(w).Encode([]map[string]any{testRepoPayload("demo")})
		case strings.HasSuffix(r.URL.Path, "/git/trees/main"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"truncated": false,
				"tree": []map[string]any{
					{"path": "main.go", "type": "blob", "sha": "abc", "size": len(content)},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/contents/main.go"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":      "file",
				"encoding":  "base64",
				"content":   encoded,
				"size":      len(content),
				"truncated": false,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newTestGitHubService(server.URL)
	file, err := service.RepoFile(context.Background(), "demo", "main.go", false)
	if err != nil {
		t.Fatalf("RepoFile: %v", err)
	}
	if !file.Truncated {
		t.Fatal("file was not marked truncated")
	}
	if !utf8.ValidString(file.Content) {
		t.Fatalf("truncated file is not valid UTF-8: %q", file.Content)
	}
	want := "a" + strings.Repeat("你", 3999) + "\n..."
	if file.Content != want {
		t.Fatalf("truncated file = %q, want %q", file.Content, want)
	}
}
