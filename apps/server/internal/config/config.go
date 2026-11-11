package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	GitHubUsername  string
	GitHubToken     string
	FeaturedRepos   []string
	HiddenRepoNames []string
	HiddenRepos     map[string]bool

	AIAPIKey  string
	AIBaseURL string
	AIModel   string

	BlogURL         string
	SiteName        string
	SiteDescription string

	AdminPassword string
	JWTSecret     string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	ServerPort string
	WebOrigin  string
}

func Load() Config {
	loadDotEnv()

	return Config{
		GitHubUsername:  env("GITHUB_USERNAME", ""),
		GitHubToken:     env("GITHUB_TOKEN", ""),
		FeaturedRepos:   ParseCSVList(env("FEATURED_REPOS", "")),
		HiddenRepoNames: ParseCSVList(env("HIDDEN_REPOS", "")),
		HiddenRepos:     parseCSVMap(env("HIDDEN_REPOS", "")),
		AIAPIKey:        env("AI_API_KEY", ""),
		AIBaseURL:       strings.TrimRight(env("AI_BASE_URL", "https://api.openai.com/v1"), "/"),
		AIModel:         env("AI_MODEL", "gpt-4o-mini"),
		BlogURL:         env("BLOG_URL", ""),
		SiteName:        env("SITE_NAME", "OrionisLi.com"),
		SiteDescription: env(
			"SITE_DESCRIPTION",
			"代码如诗，构建无限可能",
		),
		AdminPassword: env("ADMIN_PASSWORD", "change-me"),
		JWTSecret:     env("JWT_SECRET", "dev-secret-change-me"),
		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),
		ServerPort:    env("SERVER_PORT", "7020"),
		WebOrigin:     env("WEB_ORIGIN", "http://localhost:3000"),
	}
}

func loadDotEnv() {
	path, ok := findDotEnv()
	if !ok {
		return
	}

	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func findDotEnv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		path := filepath.Join(dir, ".env")
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return path, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseCSVMap(value string) map[string]bool {
	result := map[string]bool{}
	for _, item := range ParseCSVList(value) {
		result[strings.ToLower(item)] = true
	}
	return result
}

func ParseCSVList(value string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		trimmed := strings.TrimSpace(item)
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
