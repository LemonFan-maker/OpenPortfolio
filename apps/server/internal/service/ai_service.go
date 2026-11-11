package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"portfolio-hub/apps/server/internal/cache"
	"portfolio-hub/apps/server/internal/config"
	"portfolio-hub/apps/server/internal/model"
)

type AIService struct {
	cfg    config.Config
	cache  *cache.Store
	github *GitHubService
	client *http.Client

	flights                   *flightGroup
	projectSummaryConcurrency int
	projectSummaryTimeout     time.Duration
	siteSummaryTimeout        time.Duration
}

const (
	defaultProjectSummaryConcurrency = 3
	defaultProjectSummaryTimeout     = 3 * time.Minute
	defaultSiteSummaryTimeout        = 3 * time.Minute
	projectSummaryCacheTTL           = 30 * 24 * time.Hour
	siteSummaryCacheTTL              = 7 * 24 * time.Hour
)

var errEmptyAIContent = errors.New("ai api returned empty message content")

func NewAIService(cfg config.Config, store *cache.Store, github *GitHubService) *AIService {
	return &AIService{
		cfg:                       cfg,
		cache:                     store,
		github:                    github,
		client:                    &http.Client{Timeout: 30 * time.Second},
		flights:                   newFlightGroup(),
		projectSummaryConcurrency: defaultProjectSummaryConcurrency,
		projectSummaryTimeout:     defaultProjectSummaryTimeout,
		siteSummaryTimeout:        defaultSiteSummaryTimeout,
	}
}

func (s *AIService) ProjectSummary(ctx context.Context, repoName string, force bool) (model.ProjectSummary, error) {
	result, err := s.projectSummaryResult(ctx, repoName, force)
	if err != nil {
		return model.ProjectSummary{}, err
	}
	return result.Summary, nil
}

func (s *AIService) ProjectSummaryDebug(ctx context.Context, repoName string, force bool) (model.ProjectSummaryDebug, error) {
	result, err := s.projectSummaryResult(ctx, repoName, force)
	if err != nil {
		return model.ProjectSummaryDebug{}, err
	}
	return result.Debug, nil
}

func (s *AIService) SiteSummary(ctx context.Context, force bool) (model.SiteSummaryStatus, error) {
	result, err := s.siteSummaryResult(ctx, force)
	if err != nil {
		return model.SiteSummaryStatus{}, err
	}
	return result.Status, nil
}

func (s *AIService) ProjectSummaryStatus(ctx context.Context, repoName string, force bool) (model.ProjectSummaryStatus, error) {
	result, err := s.projectSummaryResult(ctx, repoName, force)
	if err != nil {
		return model.ProjectSummaryStatus{}, err
	}
	return model.ProjectSummaryStatus{
		RepoName:  result.Debug.RepoName,
		Source:    result.Debug.Source,
		Error:     result.Debug.Error,
		Cached:    result.Debug.Cached,
		Summary:   result.Summary,
		Generated: result.Debug.Generated,
	}, nil
}

type projectSummaryResult struct {
	Summary model.ProjectSummary
	Debug   model.ProjectSummaryDebug
}

type siteSummaryResult struct {
	Summary model.SiteSummary
	Status  model.SiteSummaryStatus
}

func (s *AIService) projectSummaryResult(ctx context.Context, repoName string, force bool) (projectSummaryResult, error) {
	key := fmt.Sprintf("project:%s:%t", strings.ToLower(repoName), force)
	return flightDo(s.flights, key, func() (projectSummaryResult, error) {
		timeout := s.projectSummaryTimeout
		if timeout <= 0 {
			timeout = defaultProjectSummaryTimeout
		}
		generationCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return s.projectSummaryResultOnce(generationCtx, repoName, force)
	})
}

func (s *AIService) projectSummaryResultOnce(ctx context.Context, repoName string, force bool) (projectSummaryResult, error) {
	detail, err := s.github.RepoDetail(ctx, repoName, false)
	if err != nil {
		return projectSummaryResult{}, err
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)

	if _, err := s.aiSettings(); err != nil {
		return fallbackProjectSummaryResult(repoName, detail, model.RepoTree{}, createdAt, nil, err), nil
	}

	tree, err := s.github.RepoTree(ctx, repoName, force)
	if err != nil {
		return fallbackProjectSummaryResult(repoName, detail, model.RepoTree{}, createdAt, nil, fmt.Errorf("failed to load repository tree: %w", err)), nil
	}

	cacheKey := fmt.Sprintf("ai:summary:%s:%s:%s", s.cfg.GitHubUsername, strings.ToLower(repoName), treeFingerprint(tree))
	if !force {
		var cached model.ProjectSummary
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return projectSummaryResult{
				Summary: cached,
				Debug: model.ProjectSummaryDebug{
					RepoName:  repoName,
					Source:    "cache",
					Cached:    true,
					Summary:   cached,
					Detail:    detail,
					Tree:      tree,
					Generated: createdAt,
					Turns:     []model.ProjectSummaryTurnLog{},
				},
			}, nil
		}
	}

	summary, turns, err := s.generateWithRepoBrowser(ctx, repoName, detail, tree, force)
	if err != nil {
		return fallbackProjectSummaryResult(repoName, detail, tree, createdAt, turns, err), nil
	}

	_ = s.cache.SetJSON(ctx, cacheKey, summary, projectSummaryCacheTTL)
	return projectSummaryResult{
		Summary: summary,
		Debug: model.ProjectSummaryDebug{
			RepoName:  repoName,
			Source:    "generated",
			Error:     "",
			Cached:    false,
			Summary:   summary,
			Detail:    detail,
			Tree:      tree,
			Generated: createdAt,
			Turns:     turns,
		},
	}, nil
}

func (s *AIService) siteSummaryResult(ctx context.Context, force bool) (siteSummaryResult, error) {
	key := fmt.Sprintf("site:%t", force)
	return flightDo(s.flights, key, func() (siteSummaryResult, error) {
		timeout := s.siteSummaryTimeout
		if timeout <= 0 {
			timeout = defaultSiteSummaryTimeout
		}
		generationCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return s.siteSummaryResultOnce(generationCtx, force)
	})
}

func (s *AIService) siteSummaryResultOnce(ctx context.Context, force bool) (siteSummaryResult, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)

	projects, err := s.github.ListRepos(ctx, "stars", "", "", force)
	if err != nil {
		return fallbackSiteSummaryResult(createdAt, nil, nil, nil, err), nil
	}

	var languages model.LanguageStats
	if stats, err := s.github.LanguageStats(ctx, force); err == nil {
		languages = stats
	}

	var activity model.CommitActivity
	if stats, err := s.github.CommitActivity(ctx, force); err == nil {
		activity = stats
	}

	cacheKey := fmt.Sprintf("ai:site-summary:%s:%s", s.cfg.GitHubUsername, siteFingerprint(projects, languages, activity))
	if !force {
		var cached model.SiteSummary
		if ok, err := s.cache.GetJSON(ctx, cacheKey, &cached); err == nil && ok {
			return siteSummaryResult{
				Summary: cached,
				Status: model.SiteSummaryStatus{
					Source:    "cache",
					Cached:    true,
					Summary:   cached,
					Generated: createdAt,
				},
			}, nil
		}
	}

	if _, err := s.aiSettings(); err != nil {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, err), nil
	}

	projectSummaries, err := s.collectProjectSummaries(ctx, projects, force)
	if err != nil {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, err), nil
	}

	summary, _, err := s.generateSiteSummary(ctx, projects, projectSummaries, languages, activity)
	if err != nil {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, err), nil
	}

	_ = s.cache.SetJSON(ctx, cacheKey, summary, siteSummaryCacheTTL)
	return siteSummaryResult{
		Summary: summary,
		Status: model.SiteSummaryStatus{
			Source:    "generated",
			Cached:    false,
			Summary:   summary,
			Generated: createdAt,
		},
	}, nil
}

func fallbackProjectSummaryResult(repoName string, detail model.RepoDetail, tree model.RepoTree, createdAt string, turns []model.ProjectSummaryTurnLog, err error) projectSummaryResult {
	summary := fallbackSummary(detail)
	debug := model.ProjectSummaryDebug{
		RepoName:  repoName,
		Source:    "fallback",
		Error:     "",
		Cached:    false,
		Summary:   summary,
		Detail:    detail,
		Tree:      tree,
		Generated: createdAt,
		Turns:     turns,
	}
	if err != nil {
		debug.Error = err.Error()
	}
	return projectSummaryResult{
		Summary: summary,
		Debug:   debug,
	}
}

func fallbackSiteSummaryResult(createdAt string, projects []model.ProjectCard, languages *model.LanguageStats, activity *model.CommitActivity, err error) siteSummaryResult {
	summary := fallbackSiteSummary(projects, languages, activity)
	status := model.SiteSummaryStatus{
		Source:    "fallback",
		Cached:    false,
		Summary:   summary,
		Generated: createdAt,
	}
	if err != nil {
		status.Error = err.Error()
	}
	return siteSummaryResult{
		Summary: summary,
		Status:  status,
	}
}

type projectResumeItem struct {
	Name        string
	Description string
	Language    string
	Stars       int
	Forks       int
	UpdatedAt   string
	Source      string
	Summary     model.ProjectSummary
}

func (s *AIService) collectProjectSummaries(ctx context.Context, projects []model.ProjectCard, force bool) ([]projectResumeItem, error) {
	items := make([]projectResumeItem, len(projects))
	results := make([]projectSummaryResult, len(projects))
	errs := make([]error, len(projects))

	concurrency := s.projectSummaryConcurrency
	if concurrency <= 0 {
		concurrency = defaultProjectSummaryConcurrency
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index := range projects {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errs[index] = ctx.Err()
				return
			}
			results[index], errs[index] = s.projectSummaryResult(ctx, projects[index].Name, force)
		}(index)
	}
	wg.Wait()

	for index, project := range projects {
		if errs[index] != nil {
			items[index] = projectResumeItem{
				Name:        project.Name,
				Description: project.Description,
				Language:    project.Language,
				Stars:       project.Stars,
				Forks:       project.Forks,
				UpdatedAt:   project.UpdatedAt,
				Source:      "fallback",
				Summary:     fallbackSummary(model.RepoDetail{Name: project.Name, Description: project.Description, Language: project.Language, Topics: project.Topics}),
			}
			continue
		}
		result := results[index]
		items[index] = projectResumeItem{
			Name:        result.Debug.RepoName,
			Description: result.Debug.Detail.Description,
			Language:    result.Debug.Detail.Language,
			Stars:       result.Debug.Detail.Stars,
			Forks:       result.Debug.Detail.Forks,
			UpdatedAt:   result.Debug.Detail.UpdatedAt,
			Source:      result.Debug.Source,
			Summary:     result.Summary,
		}
	}
	return items, nil
}

func (s *AIService) generateWithRepoBrowser(ctx context.Context, repoName string, detail model.RepoDetail, tree model.RepoTree, force bool) (model.ProjectSummary, []model.ProjectSummaryTurnLog, error) {
	state := repoBrowseState{
		Detail: detail,
		Tree:   tree,
		Files:  map[string]model.RepoFile{},
	}
	turnLogs := make([]model.ProjectSummaryTurnLog, 0, maxRepoBrowseTurns)
	sessionID := newAISessionID()

	for turn := 0; turn < maxRepoBrowseTurns; turn++ {
		decision, prompt, rawResponse, err := s.askRepoBrowseDecision(ctx, repoName, state, sessionID)
		if err != nil {
			turnLogs = append(turnLogs, model.ProjectSummaryTurnLog{
				Turn:     turn + 1,
				Prompt:   prompt,
				Response: rawResponse,
				Error:    err.Error(),
			})
			if isRecoverableAIFormatError(err) {
				directPrompt := buildDirectProjectSummaryPrompt(repoName, detail, tree)
				summary, directResponse, directErr := s.generateDirectProjectSummary(ctx, repoName, detail, tree, sessionID)
				directLog := model.ProjectSummaryTurnLog{
					Turn:     turn + 2,
					Prompt:   directPrompt,
					Response: directResponse,
					Action:   "finish",
					Reason:   "repo browser returned empty or invalid JSON; used direct summary prompt",
				}
				if directErr != nil {
					directLog.Error = directErr.Error()
					turnLogs = append(turnLogs, directLog)
					return model.ProjectSummary{}, turnLogs, err
				}
				turnLogs = append(turnLogs, directLog)
				return summary, turnLogs, nil
			}
			return model.ProjectSummary{}, turnLogs, err
		}
		turnLog := model.ProjectSummaryTurnLog{
			Turn:     turn + 1,
			Prompt:   prompt,
			Response: rawResponse,
			Action:   decision.Action,
			Path:     decision.Path,
			Query:    decision.Query,
			Reason:   decision.Reason,
		}

		switch decision.Action {
		case "finish":
			if decision.Summary == nil {
				turnLog.Error = "ai summary is missing required fields"
				turnLogs = append(turnLogs, turnLog)
				return model.ProjectSummary{}, turnLogs, errors.New("ai summary is missing required fields")
			}
			normalized, err := normalizeProjectSummary(*decision.Summary)
			if err != nil {
				turnLog.Error = err.Error()
				turnLogs = append(turnLogs, turnLog)
				return model.ProjectSummary{}, turnLogs, err
			}
			turnLogs = append(turnLogs, turnLog)
			return normalized, turnLogs, nil
		case "list_dir":
			entries, err := s.github.RepoTreeChildren(ctx, repoName, decision.Path, force)
			if err != nil {
				state.LastError = err.Error()
				turnLog.Error = err.Error()
				state.Notes = append(state.Notes, fmt.Sprintf("list_dir(%q) failed: %s", decision.Path, err.Error()))
				turnLogs = append(turnLogs, turnLog)
				continue
			}
			state.LastError = ""
			resultText := formatTreeEntriesNote("list_dir", decision.Path, entries)
			turnLog.ToolResult = resultText
			state.Notes = append(state.Notes, resultText)
			turnLogs = append(turnLogs, turnLog)
		case "read_file":
			file, err := s.github.RepoFile(ctx, repoName, decision.Path, force)
			if err != nil {
				state.LastError = err.Error()
				turnLog.Error = err.Error()
				state.Notes = append(state.Notes, fmt.Sprintf("read_file(%q) failed: %s", decision.Path, err.Error()))
				turnLogs = append(turnLogs, turnLog)
				continue
			}
			state.LastError = ""
			state.Files[file.Path] = file
			resultText := formatFileNote(file)
			turnLog.ToolResult = resultText
			state.Notes = append(state.Notes, resultText)
			turnLogs = append(turnLogs, turnLog)
		case "search":
			matches, err := s.github.SearchRepo(ctx, repoName, decision.Query, force)
			if err != nil {
				state.LastError = err.Error()
				turnLog.Error = err.Error()
				state.Notes = append(state.Notes, fmt.Sprintf("search(%q) failed: %s", decision.Query, err.Error()))
				turnLogs = append(turnLogs, turnLog)
				continue
			}
			state.LastError = ""
			resultText := formatSearchNote(decision.Query, matches)
			turnLog.ToolResult = resultText
			state.Searches = append(state.Searches, resultText)
			turnLogs = append(turnLogs, turnLog)
		default:
			state.LastError = fmt.Sprintf("unknown action %q", decision.Action)
			turnLog.Error = state.LastError
			state.Notes = append(state.Notes, state.LastError)
			turnLogs = append(turnLogs, turnLog)
		}
	}

	return fallbackSummary(detail), turnLogs, errors.New("ai summary did not finish in time")
}

func (s *AIService) generateDirectProjectSummary(ctx context.Context, repoName string, detail model.RepoDetail, tree model.RepoTree, sessionID string) (model.ProjectSummary, string, error) {
	prompt := buildDirectProjectSummaryPrompt(repoName, detail, tree)
	content, rawBody, err := s.requestAIContent(ctx, sessionID, "You are a repository analysis assistant. Return valid JSON only.", prompt, 0.25)
	if err != nil {
		return model.ProjectSummary{}, rawBody, err
	}

	var summary model.ProjectSummary
	if err := json.Unmarshal([]byte(jsonObjectCandidate(content)), &summary); err != nil {
		return model.ProjectSummary{}, content, fmt.Errorf("failed to parse ai JSON response: %w", err)
	}
	normalized, err := normalizeProjectSummary(summary)
	if err != nil {
		return model.ProjectSummary{}, content, err
	}
	return normalized, content, nil
}

func (s *AIService) generateSiteSummary(ctx context.Context, projects []model.ProjectCard, projectSummaries []projectResumeItem, languages model.LanguageStats, activity model.CommitActivity) (model.SiteSummary, string, error) {
	prompt := buildSiteSummaryPrompt(projects, projectSummaries, languages, activity)

	content, rawBody, err := s.requestAIContent(ctx, newAISessionID(), "You are a concise homepage copywriter. Return valid JSON only.", prompt, 0.35)
	if err != nil {
		return model.SiteSummary{}, rawBody, err
	}

	var summary model.SiteSummary
	if err := json.Unmarshal([]byte(jsonObjectCandidate(content)), &summary); err != nil {
		return model.SiteSummary{}, content, fmt.Errorf("failed to parse ai JSON response: %w", err)
	}
	normalized, err := normalizeSiteSummary(summary)
	if err != nil {
		return model.SiteSummary{}, content, err
	}
	return normalized, content, nil
}

const maxRepoBrowseTurns = 6

type repoBrowseState struct {
	Detail    model.RepoDetail
	Tree      model.RepoTree
	Files     map[string]model.RepoFile
	Searches  []string
	Notes     []string
	LastError string
}

type repoBrowseDecision struct {
	Action  string                `json:"action"`
	Path    string                `json:"path,omitempty"`
	Query   string                `json:"query,omitempty"`
	Reason  string                `json:"reason,omitempty"`
	Summary *model.ProjectSummary `json:"summary,omitempty"`
}

func (d *repoBrowseDecision) UnmarshalJSON(data []byte) error {
	var raw struct {
		Action  string          `json:"action"`
		Path    string          `json:"path,omitempty"`
		Query   string          `json:"query,omitempty"`
		Reason  string          `json:"reason,omitempty"`
		Summary json.RawMessage `json:"summary,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	d.Action = raw.Action
	d.Path = raw.Path
	d.Query = raw.Query
	d.Reason = raw.Reason
	d.Summary = nil

	if len(raw.Summary) == 0 || string(raw.Summary) == "null" {
		return nil
	}

	var summary model.ProjectSummary
	if err := json.Unmarshal(raw.Summary, &summary); err == nil {
		d.Summary = &summary
		return nil
	}

	var summaryText string
	if err := json.Unmarshal(raw.Summary, &summaryText); err == nil {
		summaryText = strings.TrimSpace(summaryText)
		if summaryText == "" {
			return nil
		}
		d.Summary = &model.ProjectSummary{
			ShortSummary: summaryText,
			LongSummary:  summaryText,
		}
		return nil
	}

	return errors.New("invalid summary payload")
}

func (s *AIService) askRepoBrowseDecision(ctx context.Context, repoName string, state repoBrowseState, sessionID string) (repoBrowseDecision, string, string, error) {
	prompt := buildRepoBrowsePrompt(repoName, state)
	content, rawBody, err := s.requestAIContent(ctx, sessionID, "You are a repository analysis assistant. Return valid JSON only.", prompt, 0.2)
	if err != nil {
		return repoBrowseDecision{}, prompt, rawBody, err
	}

	var decision repoBrowseDecision
	if err := json.Unmarshal([]byte(jsonObjectCandidate(content)), &decision); err != nil {
		return repoBrowseDecision{}, prompt, content, fmt.Errorf("failed to parse ai JSON response: %w", err)
	}
	decision.Action = strings.ToLower(strings.TrimSpace(decision.Action))
	decision.Path = strings.TrimSpace(decision.Path)
	decision.Query = strings.TrimSpace(decision.Query)
	return decision, prompt, content, nil
}

func normalizeProjectSummary(summary model.ProjectSummary) (model.ProjectSummary, error) {
	summary.ShortSummary = strings.TrimSpace(summary.ShortSummary)
	summary.LongSummary = strings.TrimSpace(summary.LongSummary)
	if summary.ShortSummary == "" || summary.LongSummary == "" {
		return model.ProjectSummary{}, errors.New("ai summary is missing required fields")
	}

	summary.Highlights = compactStringList(summary.Highlights)
	summary.TechTags = compactStringList(summary.TechTags)
	return summary, nil
}

func normalizeSiteSummary(summary model.SiteSummary) (model.SiteSummary, error) {
	summary.Title = strings.TrimSpace(summary.Title)
	summary.Subtitle = strings.TrimSpace(summary.Subtitle)
	summary.ShortSummary = strings.TrimSpace(summary.ShortSummary)
	summary.LongSummary = strings.TrimSpace(summary.LongSummary)
	if summary.Title == "" {
		summary.Title = "个人履历"
	}
	if summary.ShortSummary == "" || summary.LongSummary == "" {
		return model.SiteSummary{}, errors.New("ai summary is missing required fields")
	}

	summary.Sections = compactSiteSummarySections(summary.Sections)
	summary.TechTags = compactStringList(summary.TechTags)
	return summary, nil
}

func decodeAIContent(body []byte) (string, error) {
	var apiResp struct {
		Choices []struct {
			Text    string `json:"text"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}
	if len(apiResp.Choices) == 0 {
		return "", errors.New("ai api returned no choices")
	}

	choice := apiResp.Choices[0]
	if len(choice.Message.Content) == 0 || string(choice.Message.Content) == "null" {
		if strings.TrimSpace(choice.Text) != "" {
			return strings.TrimSpace(choice.Text), nil
		}
		return "", errEmptyAIContent
	}

	var content string
	if err := json.Unmarshal(choice.Message.Content, &content); err == nil {
		content = strings.TrimSpace(content)
		if content == "" {
			return "", errEmptyAIContent
		}
		return content, nil
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(choice.Message.Content, &parts); err == nil {
		var builder strings.Builder
		for _, part := range parts {
			if strings.TrimSpace(part.Text) != "" {
				builder.WriteString(part.Text)
			}
		}
		content = strings.TrimSpace(builder.String())
		if content == "" {
			return "", errEmptyAIContent
		}
		return content, nil
	}

	content = strings.TrimSpace(string(choice.Message.Content))
	if content == "" {
		return "", errEmptyAIContent
	}
	return content, nil
}

func jsonObjectCandidate(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}

func isRecoverableAIFormatError(err error) bool {
	if errors.Is(err, errEmptyAIContent) {
		return true
	}
	errText := err.Error()
	return strings.Contains(errText, "failed to parse ai JSON response") ||
		strings.Contains(errText, "unexpected end of JSON input") ||
		strings.Contains(errText, "invalid character")
}

func siteFingerprint(projects []model.ProjectCard, languages model.LanguageStats, activity model.CommitActivity) string {
	h := sha1.New()
	_, _ = h.Write([]byte("personal-resume-v3"))
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%d|%d|%d|%d|%d",
		len(projects),
		totalProjectStars(projects),
		totalProjectForks(projects),
		languages.TotalBytes,
		activity.TotalContributions,
		activity.MaxContributions,
	)))
	for _, project := range topProjects(projects, 8) {
		_, _ = h.Write([]byte(project.Name))
		_, _ = h.Write([]byte(project.Description))
		_, _ = h.Write([]byte(project.Language))
		_, _ = h.Write([]byte(fmt.Sprintf("%d|%d", project.Stars, project.Forks)))
	}
	for _, language := range topLanguages(languages.Languages, 5) {
		_, _ = h.Write([]byte(language.Name))
		_, _ = h.Write([]byte(fmt.Sprintf("%d|%.1f", language.Bytes, language.Percent)))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func totalProjectStars(projects []model.ProjectCard) int {
	total := 0
	for _, project := range projects {
		total += project.Stars
	}
	return total
}

func totalProjectForks(projects []model.ProjectCard) int {
	total := 0
	for _, project := range projects {
		total += project.Forks
	}
	return total
}

func topProjects(projects []model.ProjectCard, limit int) []model.ProjectCard {
	if len(projects) <= limit {
		return projects
	}
	return projects[:limit]
}

func topLanguages(languages []model.LanguageStat, limit int) []model.LanguageStat {
	if len(languages) <= limit {
		return languages
	}
	return languages[:limit]
}

func compactStringList(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func fallbackSiteSummary(projects []model.ProjectCard, languages *model.LanguageStats, activity *model.CommitActivity) model.SiteSummary {
	projectCount := len(projects)
	topLanguage := "这位开发者的主栈"
	if languages != nil && len(languages.Languages) > 0 {
		topLanguage = languages.Languages[0].Name
	}
	stars := totalProjectStars(projects)
	contributions := activityTotal(activity)

	return model.SiteSummary{
		Title:        "个人履历",
		Subtitle:     "基于项目、语言和贡献记录生成",
		ShortSummary: fmt.Sprintf("这位开发者把 %d 个项目、语言统计和贡献记录整理成了一份很完整的技术履历，%s 尤其扎实。", projectCount, topLanguage),
		LongSummary:  fmt.Sprintf("从系统内核到工具链与自动化服务，项目覆盖面很广，而且每个方向都能看出持续打磨的痕迹。累计 %d 颗星和 %d 次公开贡献，说明他不只是能做，还能长期把作品维护得很稳。", stars, contributions),
		Sections: []model.SiteSummarySection{
			{
				Title: "技术足迹",
				Items: []string{
					fmt.Sprintf("用 %d 个公开项目拼出了一条完整的技术成长线", projectCount),
					fmt.Sprintf("语言结构清晰，%s 形成稳定主栈", topLanguage),
					"长期贡献和星标积累体现了持续产出的能力",
				},
			},
		},
		TechTags: compactStringList([]string{
			topLanguage,
			"个人履历",
			"持续交付",
		}),
	}
}

func compactSiteSummarySections(sections []model.SiteSummarySection) []model.SiteSummarySection {
	result := make([]model.SiteSummarySection, 0, len(sections))
	for _, section := range sections {
		section.Title = strings.TrimSpace(section.Title)
		section.Body = strings.TrimSpace(section.Body)
		section.Items = compactStringList(section.Items)
		if section.Title == "" {
			continue
		}
		if section.Body == "" && len(section.Items) == 0 {
			continue
		}
		result = append(result, section)
	}
	return result
}

func activityTotal(activity *model.CommitActivity) int {
	if activity == nil {
		return 0
	}
	return activity.TotalContributions
}

func fallbackSummary(detail model.RepoDetail) model.ProjectSummary {
	description := detail.Description
	if description == "" {
		description = "一个公开 GitHub 项目"
	}

	tags := make([]string, 0, 4)
	if detail.Language != "" {
		tags = append(tags, detail.Language)
	}
	for _, topic := range detail.Topics {
		if len(tags) >= 4 {
			break
		}
		tags = append(tags, topic)
	}

	return model.ProjectSummary{
		ShortSummary: description,
		LongSummary:  fmt.Sprintf("%s 聚焦于 %s，适合在个人开源项目展示站中作为项目案例呈现。", detail.Name, description),
		Highlights: []string{
			"自动同步 GitHub 仓库基础信息",
			"结合 README 和仓库元数据生成展示文案",
			"保留项目链接、语言和活跃度指标",
		},
		TechTags: tags,
	}
}

func trim(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return truncateUTF8(value, max)
}
