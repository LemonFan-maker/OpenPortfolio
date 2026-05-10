package service

import (
	"testing"

	"portfolio-hub/apps/server/internal/model"
)

func TestNormalizeRepoPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "root", input: " ", want: ""},
		{name: "nested", input: "src/app/page.tsx", want: "src/app/page.tsx"},
		{name: "trims slashes", input: "/src/app/", want: "src/app"},
		{name: "rejects traversal", input: "../secret", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeRepoPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeRepoPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRepoTreeChildren(t *testing.T) {
	tree := model.RepoTree{
		Entries: []model.RepoTreeEntry{
			{Path: "README.md", Type: "file"},
			{Path: "apps", Type: "dir"},
			{Path: "apps/web", Type: "dir"},
			{Path: "apps/web/app", Type: "dir"},
			{Path: "apps/web/app/page.tsx", Type: "file"},
			{Path: "apps/server", Type: "dir"},
			{Path: "apps/server/main.go", Type: "file"},
			{Path: "packages", Type: "dir"},
		},
	}

	root, err := repoTreeChildren(tree, "")
	if err != nil {
		t.Fatalf("repoTreeChildren root: %v", err)
	}
	if len(root) != 3 {
		t.Fatalf("root children length = %d, want 3", len(root))
	}
	if root[0].Path != "apps" || root[0].Type != "dir" {
		t.Fatalf("first root child = %#v, want apps dir", root[0])
	}

	apps, err := repoTreeChildren(tree, "apps")
	if err != nil {
		t.Fatalf("repoTreeChildren apps: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("apps children length = %d, want 2", len(apps))
	}
	if apps[0].Path != "apps/server" || apps[1].Path != "apps/web" {
		t.Fatalf("apps children order = %#v", apps)
	}
}

func TestRepoTreeEntryByPath(t *testing.T) {
	tree := model.RepoTree{
		Entries: []model.RepoTreeEntry{
			{Path: "apps/web", Type: "dir"},
			{Path: "apps/web/app/page.tsx", Type: "file", SHA: "abc"},
		},
	}

	entry, ok := repoTreeEntryByPath(tree, "/apps/web/app/page.tsx")
	if !ok {
		t.Fatalf("expected path match")
	}
	if entry.Type != "file" || entry.SHA != "abc" {
		t.Fatalf("unexpected entry: %#v", entry)
	}

	if _, ok := repoTreeEntryByPath(tree, "apps/web"); !ok {
		t.Fatalf("expected directory match")
	}
}
