package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	AIProviderOpenCode = "opencode"
	AIProviderOpenAI   = "openai"

	DefaultOpenCodeBaseURL = "https://opencode.ai/inference/openai/v1"
	DefaultOpenAIBaseURL   = "https://api.openai.com/v1"

	DefaultOpenCodeModel = "deepseek-v4.1-flash"
	DefaultOpenAIModel   = "gpt-4o-mini"

	DefaultAdminPassword   = "change-me"
	DefaultJWTSecret       = "dev-secret-change-me"
	MinAdminPasswordLength = 12
	MinJWTSecretLength     = 32
)

type Config struct {
	GitHubUsername  string
	GitHubToken     string
	GitHubAPIBase   string
	GitHubGraphQL   string
	FeaturedRepos   []string
	HiddenRepoNames []string
	HiddenRepos     map[string]bool

	AIProvider      string
	AIAPIKey        string
	AIBaseURL       string
	AIModel         string
	OpenCodeAPIKey  string
	OpenCodeBaseURL string

	AIPublicPerMinute       int
	AIPublicGlobalPerMinute int
	AIPublicMaxConcurrency  int

	BlogURL         string
	SiteName        string
	SiteDescription string

	AdminPassword     string
	JWTSecret         string
	AdminCookieSecure bool
	TrustedProxies    []string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	ServerPort string
	WebOrigin  string
}

func Load() Config {
	loadDotEnv()

	aiProvider := env("AI_PROVIDER", AIProviderOpenCode)
	aiDefaultModel := DefaultOpenCodeModel
	if strings.EqualFold(aiProvider, AIProviderOpenAI) {
		aiDefaultModel = DefaultOpenAIModel
	}
	webOrigin := env("WEB_ORIGIN", "http://localhost:3000")
	adminCookieSecure := envBool("ADMIN_COOKIE_SECURE", strings.HasPrefix(webOrigin, "https://"))

	return Config{
		GitHubUsername:  env("GITHUB_USERNAME", ""),
		GitHubToken:     env("GITHUB_TOKEN", ""),
		GitHubAPIBase:   strings.TrimRight(env("GITHUB_API_BASE_URL", ""), "/"),
		GitHubGraphQL:   env("GITHUB_GRAPHQL_URL", ""),
		FeaturedRepos:   ParseCSVList(env("FEATURED_REPOS", "")),
		HiddenRepoNames: ParseCSVList(env("HIDDEN_REPOS", "")),
		HiddenRepos:     parseCSVMap(env("HIDDEN_REPOS", "")),
		AIProvider:      aiProvider,
		AIAPIKey:        env("AI_API_KEY", ""),
		AIBaseURL:       strings.TrimRight(env("AI_BASE_URL", DefaultOpenAIBaseURL), "/"),
		AIModel:         env("AI_MODEL", aiDefaultModel),
		OpenCodeAPIKey:  env("OPENCODE_API_KEY", ""),
		OpenCodeBaseURL: strings.TrimRight(env("OPENCODE_BASE_URL", DefaultOpenCodeBaseURL), "/"),

		AIPublicPerMinute:       envInt("AI_PUBLIC_PER_MINUTE", 6),
		AIPublicGlobalPerMinute: envInt("AI_PUBLIC_GLOBAL_PER_MINUTE", 30),
		AIPublicMaxConcurrency:  envInt("AI_PUBLIC_MAX_CONCURRENCY", 2),
		BlogURL:                 env("BLOG_URL", ""),
		SiteName:                env("SITE_NAME", "OrionisLi.com"),
		SiteDescription: env(
			"SITE_DESCRIPTION",
			"代码如诗，构建无限可能",
		),
		AdminPassword:     env("ADMIN_PASSWORD", DefaultAdminPassword),
		JWTSecret:         env("JWT_SECRET", DefaultJWTSecret),
		AdminCookieSecure: adminCookieSecure,
		TrustedProxies:    ParseCSVList(env("TRUSTED_PROXIES", "127.0.0.1,::1")),
		RedisAddr:         env("REDIS_ADDR", ""),
		RedisPassword:     env("REDIS_PASSWORD", ""),
		RedisDB:           envInt("REDIS_DB", 0),
		ServerPort:        env("SERVER_PORT", "7020"),
		WebOrigin:         webOrigin,
	}
}

func (c Config) AdminAuthReady() bool {
	password := strings.TrimSpace(c.AdminPassword)
	secret := strings.TrimSpace(c.JWTSecret)
	return password != "" && password != DefaultAdminPassword &&
		len(password) >= MinAdminPasswordLength &&
		secret != "" && secret != DefaultJWTSecret &&
		len(secret) >= MinJWTSecretLength
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

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
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
