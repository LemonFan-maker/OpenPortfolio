package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	pathpkg "path"
	"sort"
	"strings"
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
}

var errEmptyAIContent = errors.New("ai api returned empty message content")

func NewAIService(cfg config.Config, store *cache.Store, github *GitHubService) *AIService {
	return &AIService{
		cfg:    cfg,
		cache:  store,
		github: github,
		client: &http.Client{Timeout: 30 * time.Second},
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
	detail, err := s.github.RepoDetail(ctx, repoName, false)
	if err != nil {
		return projectSummaryResult{}, err
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)

	if s.cfg.AIAPIKey == "" {
		return fallbackProjectSummaryResult(repoName, detail, model.RepoTree{}, createdAt, nil, errors.New("ai api key is not configured")), nil
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

	_ = s.cache.SetJSON(ctx, cacheKey, summary, 0)
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

	if s.cfg.AIAPIKey == "" {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, errors.New("ai api key is not configured")), nil
	}

	projectSummaries, err := s.collectProjectSummaries(ctx, projects, force)
	if err != nil {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, err), nil
	}

	summary, _, err := s.generateSiteSummary(ctx, projects, projectSummaries, languages, activity)
	if err != nil {
		return fallbackSiteSummaryResult(createdAt, projects, &languages, &activity, err), nil
	}

	_ = s.cache.SetJSON(ctx, cacheKey, summary, 0)
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
	items := make([]projectResumeItem, 0, len(projects))
	for _, project := range projects {
		result, err := s.projectSummaryResult(ctx, project.Name, force)
		if err != nil {
			items = append(items, projectResumeItem{
				Name:        project.Name,
				Description: project.Description,
				Language:    project.Language,
				Stars:       project.Stars,
				Forks:       project.Forks,
				UpdatedAt:   project.UpdatedAt,
				Source:      "fallback",
				Summary:     fallbackSummary(model.RepoDetail{Name: project.Name, Description: project.Description, Language: project.Language, Topics: project.Topics}),
			})
			continue
		}
		items = append(items, projectResumeItem{
			Name:        result.Debug.RepoName,
			Description: result.Debug.Detail.Description,
			Language:    result.Debug.Detail.Language,
			Stars:       result.Debug.Detail.Stars,
			Forks:       result.Debug.Detail.Forks,
			UpdatedAt:   result.Debug.Detail.UpdatedAt,
			Source:      result.Debug.Source,
			Summary:     result.Summary,
		})
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

	for turn := 0; turn < maxRepoBrowseTurns; turn++ {
		decision, prompt, rawResponse, err := s.askRepoBrowseDecision(ctx, repoName, state)
		if err != nil {
			turnLogs = append(turnLogs, model.ProjectSummaryTurnLog{
				Turn:     turn + 1,
				Prompt:   prompt,
				Response: rawResponse,
				Error:    err.Error(),
			})
			if isRecoverableAIFormatError(err) {
				directPrompt := buildDirectProjectSummaryPrompt(repoName, detail, tree)
				summary, directResponse, directErr := s.generateDirectProjectSummary(ctx, repoName, detail, tree)
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

func (s *AIService) generateDirectProjectSummary(ctx context.Context, repoName string, detail model.RepoDetail, tree model.RepoTree) (model.ProjectSummary, string, error) {
	prompt := buildDirectProjectSummaryPrompt(repoName, detail, tree)
	content, rawBody, err := s.requestAIContent(ctx, "You are a repository analysis assistant. Return valid JSON only.", prompt, 0.25)
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

	content, rawBody, err := s.requestAIContent(ctx, "You are a concise homepage copywriter. Return valid JSON only.", prompt, 0.35)
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

func (s *AIService) askRepoBrowseDecision(ctx context.Context, repoName string, state repoBrowseState) (repoBrowseDecision, string, string, error) {
	prompt := buildRepoBrowsePrompt(repoName, state)
	content, rawBody, err := s.requestAIContent(ctx, "You are a repository analysis assistant. Return valid JSON only.", prompt, 0.2)
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

func (s *AIService) requestAIContent(ctx context.Context, systemContent string, userContent string, temperature float64) (string, string, error) {
	payload := map[string]any{
		"model": s.cfg.AIModel,
		"messages": []map[string]string{
			{"role": "system", "content": systemContent},
			{"role": "user", "content": userContent},
		},
		"temperature":     temperature,
		"response_format": map[string]string{"type": "json_object"},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.AIBaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.AIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	rawBody := strings.TrimSpace(string(body))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", rawBody, fmt.Errorf("ai api returned %d: %s", resp.StatusCode, rawBody)
	}

	content, err := decodeAIContent(body)
	if err != nil {
		return "", rawBody, err
	}
	return content, rawBody, nil
}

func buildRepoBrowsePrompt(repoName string, state repoBrowseState) string {
	var builder strings.Builder
	builder.WriteString("Repository browsing task.\n")
	builder.WriteString("Goal: analyze the repository source code and return a concise Chinese project summary.\n")
	builder.WriteString("Use browsing sparingly: once the project purpose, main language, and representative modules are clear, finish.\n")
	builder.WriteString("Do not exhaustively enumerate the repository; usually 1-4 tool actions are enough.\n")
	builder.WriteString("You may only request one tool action per response.\n")
	builder.WriteString("Allowed actions:\n")
	builder.WriteString("- list_dir: inspect the children of a directory. Path must point to a directory.\n")
	builder.WriteString("- read_file: inspect a file. Path must point to a file.\n")
	builder.WriteString("- search: grep-like search across file names and text files. Provide query.\n")
	builder.WriteString("- finish: return the final summary object.\n")
	builder.WriteString("Return strict JSON only with keys action, path, query, reason, summary.\n")
	builder.WriteString("Do not invent facts. Prefer source files over README when README is missing or thin.\n")
	builder.WriteString("If a tool call fails, adjust and try another path or tool.\n")
	builder.WriteString("If you want to inspect a directory, use list_dir, never read_file.\n")
	builder.WriteString("If you want to inspect a file, use read_file, never list_dir.\n")
	builder.WriteString("If you need to search for keywords across the repository, use search.\n\n")

	builder.WriteString("Repository metadata:\n")
	builder.WriteString(fmt.Sprintf("- name: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("- description: %s\n", emptyFallback(state.Detail.Description, "(none)")))
	builder.WriteString(fmt.Sprintf("- primary language: %s\n", emptyFallback(state.Detail.Language, "(none)")))
	builder.WriteString(fmt.Sprintf("- topics: %s\n", joinOrFallback(state.Detail.Topics, "(none)")))
	builder.WriteString(fmt.Sprintf("- README excerpt: %s\n", emptyFallback(trim(state.Detail.Readme, 1800), "(empty)")))
	builder.WriteString(fmt.Sprintf("- default branch: %s\n", emptyFallback(state.Tree.DefaultBranch, "(unknown)")))
	builder.WriteString(fmt.Sprintf("- tree truncated: %t\n\n", state.Tree.Truncated))

	builder.WriteString("File tree overview:\n")
	builder.WriteString(treeSummaryText(state.Tree))
	builder.WriteString("\n")

	if len(state.Notes) > 0 {
		builder.WriteString("Tool history:\n")
		for _, note := range state.Notes {
			builder.WriteString("- ")
			builder.WriteString(note)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if len(state.Searches) > 0 {
		builder.WriteString("Search results:\n")
		for _, note := range state.Searches {
			builder.WriteString("- ")
			builder.WriteString(note)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if len(state.Files) > 0 {
		builder.WriteString("Read files:\n")
		keys := make([]string, 0, len(state.Files))
		for key := range state.Files {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			file := state.Files[key]
			builder.WriteString(fmt.Sprintf("- %s (size=%d, truncated=%t)\n", file.Path, file.Size, file.Truncated))
			builder.WriteString(previewText(file.Content, 1400))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if state.LastError != "" {
		builder.WriteString(fmt.Sprintf("Last tool error: %s\n\n", state.LastError))
	}

	builder.WriteString("Final answer rules:\n")
	builder.WriteString("- When you have enough evidence, return action finish with summary.\n")
	builder.WriteString("- Summary must have shortSummary, longSummary, highlights, techTags.\n")
	builder.WriteString("- Keep the wording factual and grounded in the evidence above.\n")
	return builder.String()
}

func buildSiteSummaryPrompt(projects []model.ProjectCard, projectSummaries []projectResumeItem, languages model.LanguageStats, activity model.CommitActivity) string {
	var builder strings.Builder
	builder.WriteString("Personal resume synthesis task.\n")
	builder.WriteString("Write a Chinese personal profile for the developer behind this GitHub portfolio.\n")
	builder.WriteString("The result should read like a polished resume summary or recommendation, not a site analytics report.\n")
	builder.WriteString("Use the project AI summaries as the main evidence, and use the site statistics only as supporting context.\n")
	builder.WriteString("Do not use fixed section titles copied from the prompt. Invent the title, subtitle, and section titles yourself.\n")
	builder.WriteString("Return strict JSON only with keys title, subtitle, shortSummary, longSummary, sections, techTags.\n")
	builder.WriteString("Each section must have title and either body or items.\n")
	builder.WriteString("Keep the short summary to one warm sentence.\n")
	builder.WriteString("Keep the long summary to 2 sentences max and make it sound like a real person praising the developer.\n\n")

	builder.WriteString("Project AI summaries:\n")
	for i, project := range projectSummaries {
		builder.WriteString(fmt.Sprintf("%d. %s | source=%s | lang=%s | stars=%d | forks=%d | updated=%s\n",
			i+1,
			project.Name,
			emptyFallback(project.Source, "(unknown)"),
			emptyFallback(project.Language, "(unknown)"),
			project.Stars,
			project.Forks,
			emptyFallback(project.UpdatedAt, "(unknown)"),
		))
		builder.WriteString(fmt.Sprintf("   projectShort: %s\n", trim(project.Summary.ShortSummary, 220)))
		builder.WriteString(fmt.Sprintf("   projectLong: %s\n", trim(project.Summary.LongSummary, 420)))
		if len(project.Summary.Highlights) > 0 {
			builder.WriteString(fmt.Sprintf("   projectHighlights: %s\n", strings.Join(project.Summary.Highlights, " | ")))
		}
		if len(project.Summary.TechTags) > 0 {
			builder.WriteString(fmt.Sprintf("   projectTags: %s\n", strings.Join(project.Summary.TechTags, " | ")))
		}
	}
	builder.WriteString("\n")

	builder.WriteString("Site context:\n")
	builder.WriteString(fmt.Sprintf("- projectCount: %d\n", len(projects)))
	builder.WriteString(fmt.Sprintf("- totalStars: %d\n", totalProjectStars(projects)))
	builder.WriteString(fmt.Sprintf("- totalForks: %d\n", totalProjectForks(projects)))
	builder.WriteString(fmt.Sprintf("- contributionTotal: %d\n", activity.TotalContributions))
	builder.WriteString(fmt.Sprintf("- contributionPeak: %d\n", activity.MaxContributions))
	builder.WriteString(fmt.Sprintf("- languageTotalBytes: %d\n\n", languages.TotalBytes))

	builder.WriteString("Language stats:\n")
	for i, language := range topLanguages(languages.Languages, 5) {
		builder.WriteString(fmt.Sprintf("%d. %s | bytes=%d | percent=%.1f\n", i+1, language.Name, language.Bytes, language.Percent))
	}
	builder.WriteString("\n")

	builder.WriteString("Final answer rules:\n")
	builder.WriteString("- Write about the developer, not the website.\n")
	builder.WriteString("- Build the result from the project AI summaries first.\n")
	builder.WriteString("- Make the sections feel like a resume or personal profile, not a report.\n")
	builder.WriteString("- Use concise, human-sounding praise grounded in the evidence above.\n")
	return builder.String()
}

func buildDirectProjectSummaryPrompt(repoName string, detail model.RepoDetail, tree model.RepoTree) string {
	var builder strings.Builder
	builder.WriteString("Project summary task.\n")
	builder.WriteString("Write a concise Chinese project summary for the repository below.\n")
	builder.WriteString("Use the repository metadata, README excerpt, and file tree as evidence.\n")
	builder.WriteString("Return strict JSON only with keys shortSummary, longSummary, highlights, techTags.\n")
	builder.WriteString("Keep it factual, specific, and readable.\n\n")

	builder.WriteString("Repository metadata:\n")
	builder.WriteString(fmt.Sprintf("- name: %s\n", repoName))
	builder.WriteString(fmt.Sprintf("- description: %s\n", emptyFallback(detail.Description, "(none)")))
	builder.WriteString(fmt.Sprintf("- language: %s\n", emptyFallback(detail.Language, "(none)")))
	builder.WriteString(fmt.Sprintf("- stars: %d\n", detail.Stars))
	builder.WriteString(fmt.Sprintf("- forks: %d\n", detail.Forks))
	builder.WriteString(fmt.Sprintf("- openIssues: %d\n", detail.OpenIssues))
	builder.WriteString(fmt.Sprintf("- topics: %s\n", joinOrFallback(detail.Topics, "(none)")))
	builder.WriteString(fmt.Sprintf("- homepage: %s\n", emptyFallback(detail.Homepage, "(none)")))
	builder.WriteString(fmt.Sprintf("- updatedAt: %s\n", emptyFallback(detail.UpdatedAt, "(unknown)")))
	builder.WriteString(fmt.Sprintf("- README excerpt: %s\n\n", emptyFallback(trim(detail.Readme, 1800), "(empty)")))

	builder.WriteString("File tree overview:\n")
	builder.WriteString(treeSummaryText(tree))
	builder.WriteString("\n")

	builder.WriteString("Final answer rules:\n")
	builder.WriteString("- Use source evidence rather than generic praise.\n")
	builder.WriteString("- Keep the summary concise and grounded.\n")
	builder.WriteString("- Highlights should describe concrete strengths or modules.\n")
	builder.WriteString("- Tech tags should reflect the actual stack.\n")
	return builder.String()
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

func treeFingerprint(tree model.RepoTree) string {
	h := sha1.New()
	h.Write([]byte(tree.DefaultBranch))
	if tree.Truncated {
		h.Write([]byte("truncated"))
	}
	for _, entry := range tree.Entries {
		_, _ = h.Write([]byte(entry.Path))
		_, _ = h.Write([]byte(entry.Type))
		_, _ = h.Write([]byte(entry.SHA))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func treeSummaryText(tree model.RepoTree) string {
	rootEntries := make([]model.RepoTreeEntry, 0, 24)
	dirCounts := map[string]int{}
	for _, entry := range tree.Entries {
		parent := repoParentPath(entry.Path)
		dirCounts[parent]++
		if parent == "" {
			rootEntries = append(rootEntries, entry)
		}
	}

	sort.Slice(rootEntries, func(i, j int) bool {
		if rootEntries[i].Type == rootEntries[j].Type {
			return strings.ToLower(rootEntries[i].Path) < strings.ToLower(rootEntries[j].Path)
		}
		return rootEntries[i].Type == "dir"
	})

	var builder strings.Builder
	builder.WriteString("- root entries:\n")
	if len(rootEntries) == 0 {
		builder.WriteString("  - (none)\n")
	} else {
		limit := len(rootEntries)
		if limit > 20 {
			limit = 20
		}
		for _, entry := range rootEntries[:limit] {
			builder.WriteString(fmt.Sprintf("  - [%s] %s\n", entry.Type, entry.Path))
		}
		if len(rootEntries) > limit {
			builder.WriteString(fmt.Sprintf("  - ... (%d more root entries)\n", len(rootEntries)-limit))
		}
	}

	topDirs := make([]struct {
		path  string
		count int
	}, 0, len(dirCounts))
	for path, count := range dirCounts {
		if path == "" {
			continue
		}
		topDirs = append(topDirs, struct {
			path  string
			count int
		}{path: path, count: count})
	}
	sort.Slice(topDirs, func(i, j int) bool {
		if topDirs[i].count == topDirs[j].count {
			return strings.ToLower(topDirs[i].path) < strings.ToLower(topDirs[j].path)
		}
		return topDirs[i].count > topDirs[j].count
	})

	builder.WriteString("- notable directories:\n")
	if len(topDirs) == 0 {
		builder.WriteString("  - (none)\n")
	} else {
		limit := len(topDirs)
		if limit > 12 {
			limit = 12
		}
		for _, item := range topDirs[:limit] {
			builder.WriteString(fmt.Sprintf("  - %s (%d entries)\n", item.path, item.count))
		}
	}

	builder.WriteString("- key files:\n")
	keyFiles := make([]model.RepoTreeEntry, 0, 24)
	for _, entry := range tree.Entries {
		if entry.Type != "file" {
			continue
		}
		if isInterestingFile(entry.Path) || repoParentPath(entry.Path) == "" {
			keyFiles = append(keyFiles, entry)
		}
	}
	sort.Slice(keyFiles, func(i, j int) bool {
		if keyFiles[i].Size == keyFiles[j].Size {
			return strings.ToLower(keyFiles[i].Path) < strings.ToLower(keyFiles[j].Path)
		}
		return keyFiles[i].Size < keyFiles[j].Size
	})
	if len(keyFiles) == 0 {
		builder.WriteString("  - (none)\n")
	} else {
		limit := len(keyFiles)
		if limit > 18 {
			limit = 18
		}
		for _, entry := range keyFiles[:limit] {
			builder.WriteString(fmt.Sprintf("  - %s (size=%d)\n", entry.Path, entry.Size))
		}
	}

	return builder.String()
}

func formatTreeEntriesNote(action string, dirPath string, entries []model.RepoTreeEntry) string {
	limit := len(entries)
	if limit > 10 {
		limit = 10
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s(%q) -> %d entries", action, dirPath, len(entries)))
	if limit == 0 {
		return builder.String()
	}
	builder.WriteString(": ")
	for i := 0; i < limit; i++ {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprintf("[%s] %s", entries[i].Type, entries[i].Path))
	}
	if len(entries) > limit {
		builder.WriteString(fmt.Sprintf(", ... %d more", len(entries)-limit))
	}
	return builder.String()
}

func formatFileNote(file model.RepoFile) string {
	return fmt.Sprintf("read_file(%q) -> size=%d truncated=%t preview=%q", file.Path, file.Size, file.Truncated, previewText(file.Content, 220))
}

func formatSearchNote(query string, matches []model.RepoSearchMatch) string {
	limit := len(matches)
	if limit > 6 {
		limit = 6
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("search(%q) -> %d matches", query, len(matches)))
	if limit == 0 {
		return builder.String()
	}
	builder.WriteString(": ")
	for i := 0; i < limit; i++ {
		if i > 0 {
			builder.WriteString(" | ")
		}
		builder.WriteString(fmt.Sprintf("%s: %s", matches[i].Path, previewText(matches[i].Snippet, 90)))
	}
	if len(matches) > limit {
		builder.WriteString(fmt.Sprintf(" | ... %d more", len(matches)-limit))
	}
	return builder.String()
}

func previewText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:limit]) + "..."
}

func emptyFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func joinOrFallback(values []string, fallback string) string {
	values = compactStringList(values)
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, ", ")
}

func isInterestingFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	name := strings.ToLower(pathpkg.Base(filePath))
	if searchableNames[name] {
		return true
	}
	for _, suffix := range []string{
		".go",
		".ts",
		".tsx",
		".js",
		".jsx",
		".json",
		".md",
		".toml",
		".yml",
		".yaml",
		".py",
		".java",
		".html",
		".css",
		".sh",
		".sql",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return strings.Contains(lower, "/cmd/") || strings.Contains(lower, "/app/") || strings.Contains(lower, "/src/") || strings.Contains(lower, "/internal/")
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

func readmeHash(readme string) string {
	sum := sha1.Sum([]byte(readme))
	return hex.EncodeToString(sum[:])[:12]
}

func trim(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
