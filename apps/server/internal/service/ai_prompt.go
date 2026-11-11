package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	pathpkg "path"
	"sort"
	"strings"

	"portfolio-hub/apps/server/internal/model"
)

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
	return strings.TrimSpace(truncateUTF8(trimmed, limit)) + "..."
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
