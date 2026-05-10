package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type EnvFile struct {
	Path    string     `json:"path"`
	Entries []EnvEntry `json:"entries"`
}

type EnvEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type envLine struct {
	raw   string
	key   string
	value string
	kind  string
}

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ReadDotEnvFile() (EnvFile, error) {
	path, ok := findDotEnv()
	if !ok {
		return EnvFile{}, fmt.Errorf(".env file not found")
	}

	lines, err := readEnvLines(path)
	if err != nil {
		return EnvFile{}, err
	}

	entries := make([]EnvEntry, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		if line.kind != "entry" || seen[line.key] {
			continue
		}
		entries = append(entries, EnvEntry{Key: line.key, Value: line.value})
		seen[line.key] = true
	}

	return EnvFile{Path: path, Entries: entries}, nil
}

func SaveDotEnvFile(entries []EnvEntry) (EnvFile, error) {
	path, ok := findDotEnv()
	if !ok {
		return EnvFile{}, fmt.Errorf(".env file not found")
	}

	normalized, err := normalizeEnvEntries(entries)
	if err != nil {
		return EnvFile{}, err
	}

	lines, err := readEnvLines(path)
	if err != nil {
		return EnvFile{}, err
	}

	byKey := make(map[string]string, len(normalized))
	for _, entry := range normalized {
		byKey[entry.Key] = entry.Value
	}

	existingKeys := map[string]bool{}
	used := map[string]bool{}
	out := make([]string, 0, len(lines)+len(normalized))
	for _, line := range lines {
		if line.kind != "entry" {
			out = append(out, line.raw)
			continue
		}
		existingKeys[line.key] = true
		value, ok := byKey[line.key]
		if !ok {
			continue
		}
		out = append(out, line.key+"="+formatEnvValue(value))
		used[line.key] = true
	}

	for _, entry := range normalized {
		if !used[entry.Key] {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			out = append(out, entry.Key+"="+formatEnvValue(entry.Value))
			used[entry.Key] = true
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return EnvFile{}, err
	}
	tmpPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	content := strings.Join(out, "\n") + "\n"
	if err := os.WriteFile(tmpPath, []byte(content), info.Mode().Perm()); err != nil {
		return EnvFile{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return EnvFile{}, err
	}

	for _, entry := range normalized {
		_ = os.Setenv(entry.Key, entry.Value)
	}
	for key := range existingKeys {
		if _, ok := byKey[key]; !ok {
			_ = os.Unsetenv(key)
		}
	}

	return ReadDotEnvFile()
}

func SaveDotEnvValues(updates map[string]string) (EnvFile, error) {
	current, err := ReadDotEnvFile()
	if err != nil {
		return EnvFile{}, err
	}

	entries := make([]EnvEntry, 0, len(current.Entries)+len(updates))
	updated := map[string]bool{}
	for _, entry := range current.Entries {
		if value, ok := updates[entry.Key]; ok {
			entries = append(entries, EnvEntry{Key: entry.Key, Value: value})
			updated[entry.Key] = true
			continue
		}
		entries = append(entries, entry)
	}

	keys := make([]string, 0, len(updates))
	for key := range updates {
		if !updated[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries = append(entries, EnvEntry{Key: key, Value: updates[key]})
	}

	return SaveDotEnvFile(entries)
}

func readEnvLines(path string) ([]envLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []envLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			lines = append(lines, envLine{raw: raw, kind: "blank"})
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			lines = append(lines, envLine{raw: raw, kind: "comment"})
			continue
		}

		key, value, ok := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKeyPattern.MatchString(key) {
			lines = append(lines, envLine{raw: raw, kind: "raw"})
			continue
		}
		lines = append(lines, envLine{
			raw:   raw,
			key:   key,
			value: parseEnvValue(value),
			kind:  "entry",
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func normalizeEnvEntries(entries []EnvEntry) ([]EnvEntry, error) {
	result := make([]EnvEntry, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			continue
		}
		if !envKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid env key: %s", key)
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicated env key: %s", key)
		}
		result = append(result, EnvEntry{Key: key, Value: entry.Value})
		seen[key] = true
	}
	return result, nil
}

func parseEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 {
		return value
	}
	quote := value[0]
	if (quote != '"' && quote != '\'') || value[len(value)-1] != quote {
		return value
	}
	unquoted := value[1 : len(value)-1]
	if quote == '"' {
		unquoted = strings.ReplaceAll(unquoted, `\"`, `"`)
		unquoted = strings.ReplaceAll(unquoted, `\\`, `\`)
	}
	return unquoted
}

func formatEnvValue(value string) string {
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t#\"'\\") {
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return value
}
