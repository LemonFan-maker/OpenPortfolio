package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	pathpkg "path"
	"sort"
	"strings"
	"time"

	"portfolio-hub/apps/server/internal/model"
)

const (
	maxRepoFileContentChars = 12_000
	maxRepoSearchMatches    = 12
	maxRepoSearchCandidates = 32
)

func (s *GitHubService) RepoTree(ctx context.Context, repoName string, force bool) (model.RepoTree, error) {
	if s.isHiddenRepo(repoName) {
		return model.RepoTree{}, errors.New("repository not found")
	}

	repo, err := s.repoByName(ctx, repoName, force)
	if err != nil {
		return model.RepoTree{}, err
	}

	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	cacheKey := repoTreeCacheKey(s.cfg.GitHubUsername, repoName, branch)
	if !force {
		var cached model.RepoTree
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s?recursive=1",
		s.apiBase,
		url.PathEscape(s.cfg.GitHubUsername),
		url.PathEscape(repoName),
		url.PathEscape(branch),
	)

	var payload githubTreeResponse
	if err := s.getJSON(ctx, requestURL, &payload); err != nil {
		return model.RepoTree{}, err
	}

	entries := make([]model.RepoTreeEntry, 0, len(payload.Tree))
	for _, item := range payload.Tree {
		entries = append(entries, model.RepoTreeEntry{
			Path: item.Path,
			Type: githubTreeEntryType(item.Type),
			Size: item.Size,
			SHA:  item.SHA,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type == entries[j].Type {
			return strings.ToLower(entries[i].Path) < strings.ToLower(entries[j].Path)
		}
		return entries[i].Type == "dir"
	})

	result := model.RepoTree{
		DefaultBranch: branch,
		Truncated:     payload.Truncated,
		Entries:       entries,
	}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 6*time.Hour)
	return result, nil
}

func (s *GitHubService) RepoTreeChildren(ctx context.Context, repoName string, dirPath string, force bool) ([]model.RepoTreeEntry, error) {
	tree, err := s.RepoTree(ctx, repoName, force)
	if err != nil {
		return nil, err
	}
	return repoTreeChildren(tree, dirPath)
}

func (s *GitHubService) RepoFile(ctx context.Context, repoName string, filePath string, force bool) (model.RepoFile, error) {
	tree, err := s.RepoTree(ctx, repoName, force)
	if err != nil {
		return model.RepoFile{}, err
	}

	entry, ok := repoTreeEntryByPath(tree, filePath)
	if !ok {
		return model.RepoFile{}, errors.New("path not found in repository tree")
	}
	if entry.Type == "dir" {
		return model.RepoFile{}, errors.New("path is a directory")
	}

	normalizedPath, err := normalizeRepoPath(filePath)
	if err != nil {
		return model.RepoFile{}, err
	}

	cacheKey := repoFileCacheKey(s.cfg.GitHubUsername, repoName, normalizedPath, entry.SHA)
	if !force {
		var cached model.RepoFile
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	content, size, truncated, err := s.readRepoFileContent(ctx, repoName, normalizedPath, entry.SHA)
	if err != nil {
		return model.RepoFile{}, err
	}
	if len(content) > maxRepoFileContentChars {
		content = strings.TrimSpace(content[:maxRepoFileContentChars]) + "\n..."
		truncated = true
	}

	result := model.RepoFile{
		Path:      normalizedPath,
		Type:      "file",
		Size:      size,
		Content:   content,
		Truncated: truncated,
	}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 6*time.Hour)
	return result, nil
}

func (s *GitHubService) SearchRepo(ctx context.Context, repoName string, query string, force bool) ([]model.RepoSearchMatch, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, errors.New("query is required")
	}

	tree, err := s.RepoTree(ctx, repoName, force)
	if err != nil {
		return nil, err
	}

	matches := make([]model.RepoSearchMatch, 0, maxRepoSearchMatches)
	seen := map[string]bool{}

	for _, entry := range tree.Entries {
		if entry.Type != "file" {
			continue
		}
		if strings.Contains(strings.ToLower(entry.Path), needle) {
			matches = append(matches, model.RepoSearchMatch{
				Path:    entry.Path,
				Type:    entry.Type,
				Snippet: "path matches query",
			})
			seen[entry.Path] = true
			if len(matches) >= maxRepoSearchMatches {
				return matches, nil
			}
		}
	}

	candidates := make([]model.RepoTreeEntry, 0, len(tree.Entries))
	for _, entry := range tree.Entries {
		if entry.Type != "file" || seen[entry.Path] {
			continue
		}
		if !looksSearchable(entry.Path, entry.Size) {
			continue
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Size == candidates[j].Size {
			return strings.ToLower(candidates[i].Path) < strings.ToLower(candidates[j].Path)
		}
		return candidates[i].Size < candidates[j].Size
	})
	if len(candidates) > maxRepoSearchCandidates {
		candidates = candidates[:maxRepoSearchCandidates]
	}

	for _, entry := range candidates {
		content, _, _, err := s.readRepoFileContent(ctx, repoName, entry.Path, entry.SHA)
		if err != nil {
			continue
		}
		snippet, ok := searchSnippet(content, needle)
		if !ok {
			continue
		}
		matches = append(matches, model.RepoSearchMatch{
			Path:    entry.Path,
			Type:    entry.Type,
			Snippet: snippet,
		})
		if len(matches) >= maxRepoSearchMatches {
			break
		}
	}

	return matches, nil
}

func (s *GitHubService) repoByName(ctx context.Context, repoName string, force bool) (model.GitHubRepo, error) {
	if repoName == "" {
		return model.GitHubRepo{}, errors.New("repoName is required")
	}

	repos, err := s.listRawRepos(ctx, force)
	if err != nil {
		return model.GitHubRepo{}, err
	}

	for i := range repos {
		if strings.EqualFold(repos[i].Name, repoName) {
			return repos[i], nil
		}
	}
	return model.GitHubRepo{}, errors.New("repository not found")
}

func (s *GitHubService) readRepoFileContent(ctx context.Context, repoName string, filePath string, sha string) (string, int64, bool, error) {
	cacheKey := repoFileContentCacheKey(s.cfg.GitHubUsername, repoName, filePath, sha)
	var cached model.RepoFile
	if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
		return cached.Content, cached.Size, cached.Truncated, nil
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s",
		s.apiBase,
		url.PathEscape(s.cfg.GitHubUsername),
		url.PathEscape(repoName),
		url.PathEscape(filePath),
	)

	var payload githubContentResponse
	if err := s.getJSON(ctx, requestURL, &payload); err != nil {
		return "", 0, false, err
	}

	if payload.Type == "dir" {
		return "", 0, false, errors.New("path is a directory")
	}

	content, truncated, err := decodeGitHubContent(payload.Content, payload.Encoding)
	if err != nil || content == "" {
		content, truncated, err = s.readRepoBlobContent(ctx, repoName, sha)
		if err != nil {
			return "", 0, false, err
		}
	}

	result := model.RepoFile{
		Path:      filePath,
		Type:      "file",
		Size:      payload.Size,
		Content:   content,
		Truncated: truncated || payload.Truncated,
	}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 6*time.Hour)
	return result.Content, result.Size, result.Truncated, nil
}

func (s *GitHubService) readRepoBlobContent(ctx context.Context, repoName string, sha string) (string, bool, error) {
	if sha == "" {
		return "", false, errors.New("file content is unavailable")
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/git/blobs/%s",
		s.apiBase,
		url.PathEscape(s.cfg.GitHubUsername),
		url.PathEscape(repoName),
		url.PathEscape(sha),
	)

	var payload githubBlobResponse
	if err := s.getJSON(ctx, requestURL, &payload); err != nil {
		return "", false, err
	}

	content, truncated, err := decodeGitHubContent(payload.Content, payload.Encoding)
	if err != nil {
		return "", false, err
	}
	return content, truncated || payload.Truncated, nil
}

func repoTreeCacheKey(owner string, repoName string, branch string) string {
	return fmt.Sprintf("github:tree:%s:%s:%s", owner, strings.ToLower(repoName), sanitizeCacheToken(branch))
}

func repoFileCacheKey(owner string, repoName string, filePath string, sha string) string {
	return fmt.Sprintf("github:file:%s:%s:%s:%s", owner, strings.ToLower(repoName), sanitizeCacheToken(filePath), sanitizeCacheToken(sha))
}

func repoFileContentCacheKey(owner string, repoName string, filePath string, sha string) string {
	return fmt.Sprintf("github:file-content:%s:%s:%s:%s", owner, strings.ToLower(repoName), sanitizeCacheToken(filePath), sanitizeCacheToken(sha))
}

func sanitizeCacheToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, ":", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}

func repoTreeEntryByPath(tree model.RepoTree, filePath string) (model.RepoTreeEntry, bool) {
	normalized, err := normalizeRepoPath(filePath)
	if err != nil {
		return model.RepoTreeEntry{}, false
	}
	for _, entry := range tree.Entries {
		if entry.Path == normalized {
			return entry, true
		}
	}
	return model.RepoTreeEntry{}, false
}

func repoTreeChildren(tree model.RepoTree, dirPath string) ([]model.RepoTreeEntry, error) {
	normalized, err := normalizeRepoPath(dirPath)
	if err != nil {
		return nil, err
	}

	children := make([]model.RepoTreeEntry, 0, 16)
	for _, entry := range tree.Entries {
		parent := repoParentPath(entry.Path)
		if parent != normalized {
			continue
		}
		children = append(children, entry)
	}

	sort.Slice(children, func(i, j int) bool {
		if children[i].Type == children[j].Type {
			return strings.ToLower(children[i].Path) < strings.ToLower(children[j].Path)
		}
		return children[i].Type == "dir"
	})
	return children, nil
}

func repoParentPath(repoPath string) string {
	parent := pathpkg.Dir(repoPath)
	if parent == "." {
		return ""
	}
	return parent
}

func normalizeRepoPath(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" || trimmed == "." {
		return "", nil
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "." || segment == ".." {
			return "", errors.New("invalid repo path")
		}
	}
	cleaned := pathpkg.Clean("/" + trimmed)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return "", nil
	}
	return cleaned, nil
}

func githubTreeEntryType(value string) string {
	switch value {
	case "tree":
		return "dir"
	default:
		return "file"
	}
}

func decodeGitHubContent(content string, encoding string) (string, bool, error) {
	if content == "" {
		return "", false, nil
	}

	if encoding != "base64" {
		return strings.TrimSpace(content), false, nil
	}

	cleaned := strings.ReplaceAll(content, "\n", "")
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", false, err
	}
	return string(raw), false, nil
}

func looksSearchable(filePath string, size int64) bool {
	if size > 64_000 {
		return false
	}

	lower := strings.ToLower(filePath)
	name := strings.ToLower(pathpkg.Base(filePath))
	if searchableNames[name] {
		return true
	}
	for _, ext := range searchableExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func searchSnippet(content string, needle string) (string, bool) {
	lowerContent := strings.ToLower(content)
	index := strings.Index(lowerContent, needle)
	if index < 0 {
		return "", false
	}

	start := index - 80
	if start < 0 {
		start = 0
	}
	end := index + len(needle) + 120
	if end > len(content) {
		end = len(content)
	}

	snippet := strings.TrimSpace(content[start:end])
	if snippet == "" {
		return "", false
	}
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet += "..."
	}
	return snippet, true
}

var searchableExtensions = []string{
	".go",
	".ts",
	".tsx",
	".js",
	".jsx",
	".mjs",
	".cjs",
	".json",
	".md",
	".txt",
	".yml",
	".yaml",
	".toml",
	".py",
	".java",
	".c",
	".cc",
	".cpp",
	".h",
	".hpp",
	".css",
	".html",
	".sh",
	".sql",
	".xml",
}

var searchableNames = map[string]bool{
	"dockerfile": true,
	"makefile":   true,
	"procfile":   true,
	"readme":     true,
}

type githubTreeResponse struct {
	Truncated bool `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Type string `json:"type"`
		SHA  string `json:"sha"`
		Size int64  `json:"size"`
	} `json:"tree"`
}

type githubContentResponse struct {
	Type      string `json:"type"`
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
}

type githubBlobResponse struct {
	Encoding  string `json:"encoding"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}
