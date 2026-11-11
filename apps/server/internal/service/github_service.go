package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/model"
)

const githubAPI = "https://api.github.com"
const githubGraphQLAPI = "https://api.github.com/graphql"
const repoPageSize = 100
const maxRepoPages = 10
const repoLanguageConcurrency = 8
const maxCommitDetailItems = 8
const maxCommitDiffFiles = 6
const maxCommitPatchLength = 1400

type GitHubService struct {
	cfg             config.Config
	cache           *cache.Store
	client          *http.Client
	apiBase         string
	graphqlURL      string
	displayMu       sync.RWMutex
	featuredRepos   []string
	hiddenRepoNames []string
	hiddenRepos     map[string]bool
	flights         *flightGroup
}

func NewGitHubService(cfg config.Config, store *cache.Store) *GitHubService {
	apiBase := strings.TrimRight(cfg.GitHubAPIBase, "/")
	if apiBase == "" {
		apiBase = githubAPI
	}
	graphqlURL := cfg.GitHubGraphQL
	if graphqlURL == "" {
		graphqlURL = githubGraphQLAPI
	}
	return &GitHubService{
		cfg:             cfg,
		cache:           store,
		client:          &http.Client{Timeout: 12 * time.Second},
		apiBase:         apiBase,
		graphqlURL:      graphqlURL,
		featuredRepos:   copyRepoNameList(cfg.FeaturedRepos),
		hiddenRepoNames: copyRepoNameList(cfg.HiddenRepoNames),
		hiddenRepos:     copyRepoNameMap(cfg.HiddenRepos),
		flights:         newFlightGroup(),
	}
}

func (s *GitHubService) ListRepos(ctx context.Context, sortBy string, language string, keyword string, force bool) ([]model.ProjectCard, error) {
	repos, err := s.listRawRepos(ctx, force)
	if err != nil {
		return nil, err
	}

	cards := make([]model.ProjectCard, 0, len(repos))
	languageResults := s.allRepoLanguages(ctx, repos, force)
	for index, repo := range repos {
		card := toProjectCard(repo)
		result := languageResults[index]
		if result.err == nil {
			card.Languages = languageStatsFromMap(result.languages)
			if len(card.Languages) > 0 {
				card.Language = card.Languages[0].Name
			}
		} else if card.Language != "" {
			card.Languages = []model.LanguageStat{{Name: card.Language}}
		}
		if language != "" && !cardUsesLanguage(card, language) {
			continue
		}
		if keyword != "" && !matchesKeyword(card, keyword) {
			continue
		}
		cards = append(cards, card)
	}

	if sortBy == "featured" {
		sortFeaturedProjectCards(cards, s.FeaturedRepos())
	} else {
		sortProjectCards(cards, sortBy)
	}
	return cards, nil
}

func (s *GitHubService) FeaturedRepos() []string {
	s.displayMu.RLock()
	defer s.displayMu.RUnlock()
	return copyRepoNameList(s.featuredRepos)
}

func (s *GitHubService) HiddenRepos() []string {
	s.displayMu.RLock()
	defer s.displayMu.RUnlock()
	return copyRepoNameList(s.hiddenRepoNames)
}

func (s *GitHubService) SetFeaturedRepos(names []string) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()
	s.featuredRepos = normalizeRepoNameList(names)
}

func (s *GitHubService) SetHiddenRepos(names []string) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()
	s.hiddenRepoNames = normalizeRepoNameList(names)
	s.hiddenRepos = repoNameMap(s.hiddenRepoNames)
}

func (s *GitHubService) LanguageStats(ctx context.Context, force bool) (model.LanguageStats, error) {
	if s.cfg.GitHubUsername == "" {
		return model.LanguageStats{}, errors.New("GITHUB_USERNAME is not configured")
	}

	cacheKey := fmt.Sprintf("github:languages:%s", s.cfg.GitHubUsername)
	if !force {
		var cached model.LanguageStats
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	repos, err := s.listRawRepos(ctx, force)
	if err != nil {
		return model.LanguageStats{}, err
	}

	totals := map[string]int64{}
	var totalBytes int64
	languageResults := s.allRepoLanguages(ctx, repos, force)
	for _, result := range languageResults {
		if result.err != nil {
			continue
		}
		for name, bytes := range result.languages {
			totals[name] += bytes
			totalBytes += bytes
		}
	}

	stats := make([]model.LanguageStat, 0, len(totals))
	for name, bytes := range totals {
		stats = append(stats, model.LanguageStat{
			Name:    name,
			Bytes:   bytes,
			Percent: percent(bytes, totalBytes),
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Bytes > stats[j].Bytes
	})

	result := model.LanguageStats{TotalBytes: totalBytes, Languages: compactLanguages(stats, totalBytes, 8)}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 12*time.Hour)
	return result, nil
}

func (s *GitHubService) RepoDetail(ctx context.Context, repoName string, force bool) (model.RepoDetail, error) {
	if repoName == "" {
		return model.RepoDetail{}, errors.New("repoName is required")
	}
	if s.isHiddenRepo(repoName) {
		return model.RepoDetail{}, errors.New("repository not found")
	}

	cacheKey := fmt.Sprintf("github:repo:%s:%s", s.cfg.GitHubUsername, strings.ToLower(repoName))
	if !force {
		var cached model.RepoDetail
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	repos, err := s.listRawRepos(ctx, force)
	if err != nil {
		return model.RepoDetail{}, err
	}

	var selected *model.GitHubRepo
	for i := range repos {
		if strings.EqualFold(repos[i].Name, repoName) {
			selected = &repos[i]
			break
		}
	}
	if selected == nil {
		return model.RepoDetail{}, errors.New("repository not found")
	}

	languageMap, _ := s.repoLanguages(ctx, selected.Name, force)
	languages := languageStatsFromMap(languageMap)

	readme, _ := s.readme(ctx, selected.Name, force)
	card := toProjectCard(*selected)
	detail := model.RepoDetail{
		Name:        card.Name,
		Description: card.Description,
		Readme:      summarizeReadme(readme, 2200),
		Language:    card.Language,
		Languages:   languages,
		Stars:       card.Stars,
		Forks:       card.Forks,
		OpenIssues:  selected.OpenIssuesCount,
		Topics:      card.Topics,
		URL:         card.URL,
		Homepage:    card.Homepage,
		UpdatedAt:   card.UpdatedAt,
	}

	_ = s.cache.SetJSON(ctx, cacheKey, detail, 6*time.Hour)
	return detail, nil
}

func (s *GitHubService) CommitActivity(ctx context.Context, force bool) (model.CommitActivity, error) {
	if s.cfg.GitHubUsername == "" {
		return model.CommitActivity{}, errors.New("GITHUB_USERNAME is not configured")
	}
	if s.cfg.GitHubToken == "" {
		return model.CommitActivity{}, errors.New("GITHUB_TOKEN is required for GitHub contribution activity")
	}

	cacheKey := fmt.Sprintf("github:activity:%s", s.cfg.GitHubUsername)
	if !force {
		var cached model.CommitActivity
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	now := time.Now().UTC()
	start := now.AddDate(-1, 0, 1)
	var payload githubGraphQLResponse
	err := s.postGraphQL(ctx, githubContributionQuery, map[string]any{
		"login": s.cfg.GitHubUsername,
		"from":  start.Format(time.RFC3339),
		"to":    now.Format(time.RFC3339),
	}, &payload)
	if err != nil {
		return model.CommitActivity{}, err
	}
	if len(payload.Errors) > 0 {
		return model.CommitActivity{}, errors.New(payload.Errors[0].Message)
	}
	if payload.Data.User == nil {
		return model.CommitActivity{}, errors.New("github user not found")
	}

	calendar := payload.Data.User.ContributionsCollection.ContributionCalendar
	weeks := make([]model.ContributionWeek, 0, len(calendar.Weeks))
	maxContributions := 0
	for _, week := range calendar.Weeks {
		days := make([]model.ContributionDay, 0, len(week.ContributionDays))
		for _, day := range week.ContributionDays {
			if day.ContributionCount > maxContributions {
				maxContributions = day.ContributionCount
			}
			days = append(days, model.ContributionDay{
				Date:  day.Date,
				Count: day.ContributionCount,
				Color: day.Color,
			})
		}
		weeks = append(weeks, model.ContributionWeek{
			FirstDay: week.FirstDay,
			Days:     days,
		})
	}

	months := make([]model.ContributionMonth, 0, len(calendar.Months))
	for _, month := range calendar.Months {
		months = append(months, model.ContributionMonth{
			Name:       month.Name,
			FirstDay:   month.FirstDay,
			TotalWeeks: month.TotalWeeks,
			Year:       month.Year,
		})
	}

	result := model.CommitActivity{
		TotalContributions: calendar.TotalContributions,
		MaxContributions:   maxContributions,
		Weeks:              weeks,
		Months:             months,
	}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 3*time.Hour)
	return result, nil
}

func (s *GitHubService) CommitActivityDetail(ctx context.Context, date string, force bool) (model.CommitActivityDetail, error) {
	if s.cfg.GitHubUsername == "" {
		return model.CommitActivityDetail{}, errors.New("GITHUB_USERNAME is not configured")
	}
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return model.CommitActivityDetail{}, errors.New("date must use YYYY-MM-DD")
	}
	normalizedDate := parsedDate.Format("2006-01-02")

	cacheKey := fmt.Sprintf("github:activity-detail:%s:%s", s.cfg.GitHubUsername, normalizedDate)
	if !force {
		var cached model.CommitActivityDetail
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			if filtered, changed := filterHiddenCommitRecords(cached.Commits, s.hiddenRepoMap()); changed {
				cached.Commits = filtered
				cached.TotalCommits = len(filtered)
			}
			return cached, nil
		}
	}

	params := url.Values{}
	params.Set("q", fmt.Sprintf("author:%s author-date:%s..%s", s.cfg.GitHubUsername, normalizedDate, normalizedDate))
	params.Set("sort", "author-date")
	params.Set("order", "desc")
	params.Set("per_page", fmt.Sprintf("%d", maxCommitDetailItems))

	var search githubCommitSearchResponse
	if err := s.getJSON(ctx, fmt.Sprintf("%s/search/commits?%s", s.apiBase, params.Encode()), &search); err != nil {
		return model.CommitActivityDetail{}, err
	}

	records := make([]model.CommitRecord, 0, len(search.Items))
	filtered := false
	for _, item := range search.Items {
		repoName := item.Repository.Name
		if repoName == "" {
			repoName = repoNameFromFullName(item.Repository.FullName)
		}
		if item.Repository.Private || s.isHiddenRepo(repoName) {
			filtered = true
			continue
		}

		record := model.CommitRecord{
			SHA:         item.SHA,
			Message:     firstLine(item.Commit.Message),
			Repository:  item.Repository.FullName,
			URL:         item.HTMLURL,
			CommittedAt: item.Commit.Author.Date,
			Files:       []model.CommitFileDiff{},
		}

		if item.URL != "" {
			var detail githubCommitDetailResponse
			if err := s.getJSON(ctx, item.URL, &detail); err == nil {
				record.Additions = detail.Stats.Additions
				record.Deletions = detail.Stats.Deletions
				record.FilesChanged = len(detail.Files)
				record.Files = compactCommitFiles(detail.Files)
			}
		}
		records = append(records, record)
	}

	totalCommits := search.TotalCount
	if filtered {
		totalCommits = len(records)
	}
	result := model.CommitActivityDetail{
		Date:         normalizedDate,
		TotalCommits: totalCommits,
		Commits:      records,
		Truncated:    search.TotalCount > len(records),
	}
	_ = s.cache.SetJSON(ctx, cacheKey, result, 3*time.Hour)
	return result, nil
}

func (s *GitHubService) RefreshRepos(ctx context.Context) error {
	_, err := s.listRawRepos(ctx, true)
	return err
}

func (s *GitHubService) RefreshLanguages(ctx context.Context) error {
	_, err := s.LanguageStats(ctx, true)
	return err
}

func (s *GitHubService) RefreshActivity(ctx context.Context) error {
	_, err := s.CommitActivity(ctx, true)
	return err
}

func (s *GitHubService) listRawRepos(ctx context.Context, force bool) ([]model.GitHubRepo, error) {
	return flightDo(s.flights, fmt.Sprintf("github:repos:raw:%t", force), func() ([]model.GitHubRepo, error) {
		return s.listRawReposOnce(ctx, force)
	})
}

func (s *GitHubService) listRawReposOnce(ctx context.Context, force bool) ([]model.GitHubRepo, error) {
	if s.cfg.GitHubUsername == "" {
		return nil, errors.New("GITHUB_USERNAME is not configured")
	}

	cacheKey := fmt.Sprintf("github:repos:base:%s", s.cfg.GitHubUsername)
	if !force {
		var cached []model.GitHubRepo
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return filterHiddenRepos(cached, s.hiddenRepoMap()), nil
		}
	}

	apiRepos := make([]githubRepoResponse, 0, repoPageSize)
	for page := 1; page <= maxRepoPages; page++ {
		requestURL := fmt.Sprintf("%s/users/%s/repos?per_page=%d&page=%d&type=owner&sort=updated",
			s.apiBase,
			url.PathEscape(s.cfg.GitHubUsername),
			repoPageSize,
			page,
		)
		var pageRepos []githubRepoResponse
		if err := s.getJSON(ctx, requestURL, &pageRepos); err != nil {
			return nil, err
		}
		apiRepos = append(apiRepos, pageRepos...)
		if len(pageRepos) < repoPageSize {
			break
		}
	}

	repos := make([]model.GitHubRepo, 0, len(apiRepos))
	for _, item := range apiRepos {
		repo := item.toModel()
		if repo.Fork || repo.Archived || repo.Private {
			continue
		}
		repos = append(repos, repo)
	}

	_ = s.cache.SetJSON(ctx, cacheKey, repos, 6*time.Hour)
	return filterHiddenRepos(repos, s.hiddenRepoMap()), nil
}

func (s *GitHubService) hiddenRepoMap() map[string]bool {
	s.displayMu.RLock()
	defer s.displayMu.RUnlock()
	return copyRepoNameMap(s.hiddenRepos)
}

func (s *GitHubService) isHiddenRepo(repoName string) bool {
	s.displayMu.RLock()
	defer s.displayMu.RUnlock()
	return s.hiddenRepos[strings.ToLower(strings.TrimSpace(repoName))]
}

func (s *GitHubService) ClearRepositories(ctx context.Context, names []string) error {
	if s.cache == nil {
		return nil
	}

	patterns := make([]string, 0, len(names)*7)
	for _, name := range names {
		repoName := strings.ToLower(strings.TrimSpace(name))
		if repoName == "" {
			continue
		}
		owner := s.cfg.GitHubUsername
		patterns = append(patterns,
			fmt.Sprintf("github:repo:%s:%s", owner, repoName),
			fmt.Sprintf("github:readme:%s:%s", owner, repoName),
			fmt.Sprintf("github:languages:%s:%s", owner, repoName),
			fmt.Sprintf("github:tree:%s:%s:*", owner, repoName),
			fmt.Sprintf("github:file:%s:%s:*", owner, repoName),
			fmt.Sprintf("github:file-content:%s:%s:*", owner, repoName),
			fmt.Sprintf("ai:summary:%s:%s:*", owner, repoName),
		)
	}
	if len(patterns) == 0 {
		return nil
	}
	_, err := s.cache.DeletePatterns(ctx, patterns...)
	return err
}

type repoLanguagesResult struct {
	languages map[string]int64
	err       error
}

func (s *GitHubService) allRepoLanguages(ctx context.Context, repos []model.GitHubRepo, force bool) []repoLanguagesResult {
	results := make([]repoLanguagesResult, len(repos))
	if len(repos) == 0 {
		return results
	}

	sem := make(chan struct{}, repoLanguageConcurrency)
	var wg sync.WaitGroup
	for index := range repos {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[index] = repoLanguagesResult{err: ctx.Err()}
				return
			}
			results[index].languages, results[index].err = s.repoLanguages(ctx, repos[index].Name, force)
		}(index)
	}
	wg.Wait()
	return results
}

func (s *GitHubService) repoLanguages(ctx context.Context, repoName string, force bool) (map[string]int64, error) {
	key := fmt.Sprintf("github:languages:%t:%s", force, strings.ToLower(repoName))
	return flightDo(s.flights, key, func() (map[string]int64, error) {
		return s.repoLanguagesOnce(ctx, repoName, force)
	})
}

func (s *GitHubService) repoLanguagesOnce(ctx context.Context, repoName string, force bool) (map[string]int64, error) {
	cacheKey := fmt.Sprintf("github:languages:%s:%s", s.cfg.GitHubUsername, strings.ToLower(repoName))
	if !force {
		var cached map[string]int64
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/languages", s.apiBase, url.PathEscape(s.cfg.GitHubUsername), url.PathEscape(repoName))
	languages := map[string]int64{}
	if err := s.getJSON(ctx, requestURL, &languages); err != nil {
		return nil, err
	}
	_ = s.cache.SetJSON(ctx, cacheKey, languages, 12*time.Hour)
	return languages, nil
}

func (s *GitHubService) readme(ctx context.Context, repoName string, force bool) (string, error) {
	cacheKey := fmt.Sprintf("github:readme:%s:%s", s.cfg.GitHubUsername, strings.ToLower(repoName))
	if !force {
		var cached string
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return cached, nil
		}
	}

	requestURL := fmt.Sprintf("%s/repos/%s/%s/readme", s.apiBase, url.PathEscape(s.cfg.GitHubUsername), url.PathEscape(repoName))
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := s.getJSON(ctx, requestURL, &payload); err != nil {
		return "", err
	}

	content := strings.ReplaceAll(payload.Content, "\n", "")
	if payload.Encoding == "base64" {
		raw, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return "", err
		}
		content = string(raw)
	}

	_ = s.cache.SetJSON(ctx, cacheKey, content, 12*time.Hour)
	return content, nil
}

func (s *GitHubService) getJSON(ctx context.Context, requestURL string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "portfolio-hub")
	if s.cfg.GitHubToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.GitHubToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return json.Unmarshal(body, target)
}

func (s *GitHubService) postGraphQL(ctx context.Context, query string, variables map[string]any, target any) error {
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphqlURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "portfolio-hub")
	req.Header.Set("Authorization", "Bearer "+s.cfg.GitHubToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github graphql returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	return json.Unmarshal(raw, target)
}

func toProjectCard(repo model.GitHubRepo) model.ProjectCard {
	description := ""
	if repo.Description != nil {
		description = *repo.Description
	}
	language := ""
	if repo.Language != nil {
		language = *repo.Language
	}
	homepage := ""
	if repo.Homepage != nil {
		homepage = *repo.Homepage
	}
	return model.ProjectCard{
		Name:        repo.Name,
		Description: description,
		Language:    language,
		Stars:       repo.StargazersCount,
		Forks:       repo.ForksCount,
		Topics:      repo.Topics,
		URL:         repo.HTMLURL,
		Homepage:    homepage,
		UpdatedAt:   repo.UpdatedAt,
	}
}

func sortProjectCards(cards []model.ProjectCard, sortBy string) {
	switch sortBy {
	case "name":
		sort.Slice(cards, func(i, j int) bool { return strings.ToLower(cards[i].Name) < strings.ToLower(cards[j].Name) })
	case "updated":
		sort.Slice(cards, func(i, j int) bool {
			left := projectUpdatedTime(cards[i].UpdatedAt)
			right := projectUpdatedTime(cards[j].UpdatedAt)
			if left.Equal(right) {
				return strings.ToLower(cards[i].Name) < strings.ToLower(cards[j].Name)
			}
			return left.After(right)
		})
	default:
		sort.Slice(cards, func(i, j int) bool {
			if cards[i].Stars == cards[j].Stars {
				return projectUpdatedTime(cards[i].UpdatedAt).After(projectUpdatedTime(cards[j].UpdatedAt))
			}
			return cards[i].Stars > cards[j].Stars
		})
	}
}

func sortFeaturedProjectCards(cards []model.ProjectCard, featured []string) {
	if len(featured) == 0 {
		sortProjectCards(cards, "stars")
		return
	}

	rank := map[string]int{}
	for index, name := range featured {
		rank[strings.ToLower(name)] = index
	}

	sort.Slice(cards, func(i, j int) bool {
		leftRank, leftFeatured := rank[strings.ToLower(cards[i].Name)]
		rightRank, rightFeatured := rank[strings.ToLower(cards[j].Name)]
		if leftFeatured && rightFeatured {
			return leftRank < rightRank
		}
		if leftFeatured != rightFeatured {
			return leftFeatured
		}
		if cards[i].Stars == cards[j].Stars {
			return projectUpdatedTime(cards[i].UpdatedAt).After(projectUpdatedTime(cards[j].UpdatedAt))
		}
		return cards[i].Stars > cards[j].Stars
	})
}

func repoNameFromFullName(fullName string) string {
	trimmed := strings.TrimSpace(fullName)
	if index := strings.LastIndex(trimmed, "/"); index >= 0 && index+1 < len(trimmed) {
		return trimmed[index+1:]
	}
	return trimmed
}

func filterHiddenCommitRecords(records []model.CommitRecord, hidden map[string]bool) ([]model.CommitRecord, bool) {
	if len(hidden) == 0 {
		return records, false
	}

	filtered := make([]model.CommitRecord, 0, len(records))
	changed := false
	for _, record := range records {
		if hidden[strings.ToLower(repoNameFromFullName(record.Repository))] {
			changed = true
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered, changed
}

func filterHiddenRepos(repos []model.GitHubRepo, hidden map[string]bool) []model.GitHubRepo {
	if len(hidden) == 0 {
		return repos
	}

	filtered := make([]model.GitHubRepo, 0, len(repos))
	for _, repo := range repos {
		if hidden[strings.ToLower(repo.Name)] {
			continue
		}
		filtered = append(filtered, repo)
	}
	return filtered
}

func normalizeRepoNameList(names []string) []string {
	result := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		result = append(result, trimmed)
		seen[key] = true
	}
	return result
}

func repoNameMap(names []string) map[string]bool {
	result := map[string]bool{}
	for _, name := range normalizeRepoNameList(names) {
		result[strings.ToLower(name)] = true
	}
	return result
}

func copyRepoNameList(names []string) []string {
	return append([]string{}, normalizeRepoNameList(names)...)
}

func copyRepoNameMap(names map[string]bool) map[string]bool {
	result := map[string]bool{}
	for name, hidden := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if hidden && key != "" {
			result[key] = true
		}
	}
	return result
}

func projectUpdatedTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed
	}
	parsed, err = time.Parse("2006-01-02", value)
	if err == nil {
		return parsed
	}
	return time.Time{}
}

func cardUsesLanguage(card model.ProjectCard, language string) bool {
	for _, item := range card.Languages {
		if strings.EqualFold(item.Name, language) {
			return true
		}
	}
	return strings.EqualFold(card.Language, language)
}

func matchesKeyword(card model.ProjectCard, keyword string) bool {
	needle := strings.ToLower(strings.TrimSpace(keyword))
	languages := make([]string, 0, len(card.Languages))
	for _, item := range card.Languages {
		languages = append(languages, item.Name)
	}
	return strings.Contains(strings.ToLower(card.Name), needle) ||
		strings.Contains(strings.ToLower(card.Description), needle) ||
		strings.Contains(strings.ToLower(strings.Join(languages, " ")), needle) ||
		strings.Contains(strings.ToLower(strings.Join(card.Topics, " ")), needle)
}

func languageStatsFromMap(languageMap map[string]int64) []model.LanguageStat {
	languages := make([]model.LanguageStat, 0, len(languageMap))
	var total int64
	for _, bytes := range languageMap {
		total += bytes
	}
	for name, bytes := range languageMap {
		languages = append(languages, model.LanguageStat{Name: name, Bytes: bytes, Percent: percent(bytes, total)})
	}
	sort.Slice(languages, func(i, j int) bool { return languages[i].Bytes > languages[j].Bytes })
	return languages
}

func percent(bytes int64, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round((float64(bytes)/float64(total))*1000) / 10
}

func compactLanguages(stats []model.LanguageStat, total int64, maxItems int) []model.LanguageStat {
	if len(stats) <= maxItems {
		return stats
	}

	visible := append([]model.LanguageStat{}, stats[:maxItems]...)
	var otherBytes int64
	for _, item := range stats[maxItems:] {
		otherBytes += item.Bytes
	}
	visible = append(visible, model.LanguageStat{
		Name:    "Other",
		Bytes:   otherBytes,
		Percent: percent(otherBytes, total),
	})
	return visible
}

func compactCommitFiles(files []githubCommitFileResponse) []model.CommitFileDiff {
	limit := len(files)
	if limit > maxCommitDiffFiles {
		limit = maxCommitDiffFiles
	}

	result := make([]model.CommitFileDiff, 0, limit)
	for _, file := range files[:limit] {
		patch := ""
		if file.Patch != nil {
			patch = truncatePatch(*file.Patch)
		}
		result = append(result, model.CommitFileDiff{
			Filename:  file.Filename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Patch:     patch,
		})
	}
	return result
}

func firstLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func truncatePatch(patch string) string {
	if len(patch) <= maxCommitPatchLength {
		return patch
	}
	return strings.TrimSpace(truncateUTF8(patch, maxCommitPatchLength)) + "\n..."
}

func summarizeReadme(content string, limit int) string {
	cleaned := strings.TrimSpace(content)
	if len(cleaned) <= limit {
		return cleaned
	}
	return strings.TrimSpace(truncateUTF8(cleaned, limit)) + "..."
}

const githubContributionQuery = `
query ContributionCalendar($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar {
        totalContributions
        weeks {
          firstDay
          contributionDays {
            date
            contributionCount
            color
          }
        }
        months {
          name
          firstDay
          totalWeeks
          year
        }
      }
    }
  }
}
`

type githubGraphQLResponse struct {
	Data struct {
		User *struct {
			ContributionsCollection struct {
				ContributionCalendar struct {
					TotalContributions int `json:"totalContributions"`
					Weeks              []struct {
						FirstDay         string `json:"firstDay"`
						ContributionDays []struct {
							Date              string `json:"date"`
							ContributionCount int    `json:"contributionCount"`
							Color             string `json:"color"`
						} `json:"contributionDays"`
					} `json:"weeks"`
					Months []struct {
						Name       string `json:"name"`
						FirstDay   string `json:"firstDay"`
						TotalWeeks int    `json:"totalWeeks"`
						Year       int    `json:"year"`
					} `json:"months"`
				} `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type githubCommitSearchResponse struct {
	TotalCount int                      `json:"total_count"`
	Items      []githubCommitSearchItem `json:"items"`
}

type githubCommitSearchItem struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	URL     string `json:"url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
	} `json:"repository"`
}

type githubCommitDetailResponse struct {
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
	Files []githubCommitFileResponse `json:"files"`
}

type githubCommitFileResponse struct {
	Filename  string  `json:"filename"`
	Status    string  `json:"status"`
	Additions int     `json:"additions"`
	Deletions int     `json:"deletions"`
	Patch     *string `json:"patch"`
}

type githubRepoResponse struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	FullName        string   `json:"full_name"`
	DefaultBranch   string   `json:"default_branch"`
	Description     *string  `json:"description"`
	HTMLURL         string   `json:"html_url"`
	Homepage        *string  `json:"homepage"`
	Language        *string  `json:"language"`
	Topics          []string `json:"topics"`
	StargazersCount int      `json:"stargazers_count"`
	ForksCount      int      `json:"forks_count"`
	OpenIssuesCount int      `json:"open_issues_count"`
	Fork            bool     `json:"fork"`
	Archived        bool     `json:"archived"`
	Private         bool     `json:"private"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	PushedAt        string   `json:"pushed_at"`
}

func (r githubRepoResponse) toModel() model.GitHubRepo {
	return model.GitHubRepo{
		ID:              r.ID,
		Name:            r.Name,
		FullName:        r.FullName,
		DefaultBranch:   r.DefaultBranch,
		Description:     r.Description,
		HTMLURL:         r.HTMLURL,
		Homepage:        r.Homepage,
		Language:        r.Language,
		Topics:          r.Topics,
		StargazersCount: r.StargazersCount,
		ForksCount:      r.ForksCount,
		OpenIssuesCount: r.OpenIssuesCount,
		Fork:            r.Fork,
		Archived:        r.Archived,
		Private:         r.Private,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		PushedAt:        r.PushedAt,
	}
}
