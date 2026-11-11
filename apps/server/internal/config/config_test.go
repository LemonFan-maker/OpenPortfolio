package config

import (
	"os"
	"testing"
)

func chdirTemp(t *testing.T) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func clearAITestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AI_PROVIDER",
		"OPENCODE_API_KEY",
		"OPENCODE_BASE_URL",
		"AI_API_KEY",
		"AI_BASE_URL",
		"AI_MODEL",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaultsToOpenCodeProvider(t *testing.T) {
	chdirTemp(t)
	clearAITestEnv(t)

	cfg := Load()
	if cfg.AIProvider != AIProviderOpenCode {
		t.Fatalf("AIProvider = %q, want %q", cfg.AIProvider, AIProviderOpenCode)
	}
	if cfg.OpenCodeBaseURL != DefaultOpenCodeBaseURL {
		t.Fatalf("OpenCodeBaseURL = %q, want %q", cfg.OpenCodeBaseURL, DefaultOpenCodeBaseURL)
	}
	if cfg.AIModel != DefaultOpenCodeModel {
		t.Fatalf("AIModel = %q, want %q", cfg.AIModel, DefaultOpenCodeModel)
	}
}

func TestLoadUsesOpenAIDefaultsWhenSelected(t *testing.T) {
	chdirTemp(t)
	clearAITestEnv(t)
	t.Setenv("AI_PROVIDER", "openai")

	cfg := Load()
	if cfg.AIProvider != AIProviderOpenAI {
		t.Fatalf("AIProvider = %q, want %q", cfg.AIProvider, AIProviderOpenAI)
	}
	if cfg.AIModel != DefaultOpenAIModel {
		t.Fatalf("AIModel = %q, want %q", cfg.AIModel, DefaultOpenAIModel)
	}
}

func TestLoadReadsExplicitOpenCodeSettings(t *testing.T) {
	chdirTemp(t)
	clearAITestEnv(t)
	t.Setenv("AI_PROVIDER", "opencode")
	t.Setenv("OPENCODE_API_KEY", "oc_sk_test")
	t.Setenv("OPENCODE_BASE_URL", "https://example.test/inference/openai/v1/")
	t.Setenv("AI_MODEL", "glm-5.3")

	cfg := Load()
	if cfg.OpenCodeAPIKey != "oc_sk_test" {
		t.Fatalf("OpenCodeAPIKey = %q", cfg.OpenCodeAPIKey)
	}
	if cfg.OpenCodeBaseURL != "https://example.test/inference/openai/v1" {
		t.Fatalf("OpenCodeBaseURL = %q, want trailing slash removed", cfg.OpenCodeBaseURL)
	}
	if cfg.AIModel != "glm-5.3" {
		t.Fatalf("AIModel = %q, want glm-5.3", cfg.AIModel)
	}
}

func clearAdminTestEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ADMIN_PASSWORD",
		"JWT_SECRET",
		"ADMIN_COOKIE_SECURE",
		"TRUSTED_PROXIES",
		"WEB_ORIGIN",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadAdminAuthDefaultsToUnavailable(t *testing.T) {
	chdirTemp(t)
	clearAdminTestEnv(t)

	cfg := Load()
	if cfg.AdminAuthReady() {
		t.Fatal("default admin credentials should not be ready")
	}
	if cfg.AdminCookieSecure {
		t.Fatal("admin cookie should not be secure for an HTTP origin")
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "127.0.0.1" {
		t.Fatalf("trusted proxies = %#v", cfg.TrustedProxies)
	}
}

func TestLoadAdminCookieSecureFollowsHTTPSOrigin(t *testing.T) {
	chdirTemp(t)
	clearAdminTestEnv(t)
	t.Setenv("WEB_ORIGIN", "https://portfolio.example.com")

	cfg := Load()
	if !cfg.AdminCookieSecure {
		t.Fatal("admin cookie should be secure for an HTTPS origin")
	}
}

func TestLoadAdminCookieSecureCanBeOverridden(t *testing.T) {
	chdirTemp(t)
	clearAdminTestEnv(t)
	t.Setenv("WEB_ORIGIN", "https://portfolio.example.com")
	t.Setenv("ADMIN_COOKIE_SECURE", "false")

	cfg := Load()
	if cfg.AdminCookieSecure {
		t.Fatal("explicit ADMIN_COOKIE_SECURE=false was ignored")
	}
}

func TestAdminAuthReadyRequiresNonDefaultStrongSecrets(t *testing.T) {
	base := Config{
		AdminPassword: "a-long-admin-password",
		JWTSecret:     "0123456789abcdef0123456789abcdef",
	}
	if !base.AdminAuthReady() {
		t.Fatal("strong custom credentials should be ready")
	}

	cases := []Config{
		{AdminPassword: DefaultAdminPassword, JWTSecret: base.JWTSecret},
		{AdminPassword: base.AdminPassword, JWTSecret: DefaultJWTSecret},
		{AdminPassword: "short", JWTSecret: base.JWTSecret},
		{AdminPassword: base.AdminPassword, JWTSecret: "short"},
		{AdminPassword: "", JWTSecret: base.JWTSecret},
		{AdminPassword: base.AdminPassword, JWTSecret: ""},
	}
	for _, cfg := range cases {
		if cfg.AdminAuthReady() {
			t.Fatalf("insecure credentials were accepted: %#v", cfg)
		}
	}
}

func TestLoadPublicAIRateLimitDefaults(t *testing.T) {
	chdirTemp(t)
	clearAITestEnv(t)
	for _, key := range []string{
		"AI_PUBLIC_PER_MINUTE",
		"AI_PUBLIC_GLOBAL_PER_MINUTE",
		"AI_PUBLIC_MAX_CONCURRENCY",
	} {
		t.Setenv(key, "")
	}

	cfg := Load()
	if cfg.AIPublicPerMinute != 6 {
		t.Fatalf("AIPublicPerMinute = %d, want 6", cfg.AIPublicPerMinute)
	}
	if cfg.AIPublicGlobalPerMinute != 30 {
		t.Fatalf("AIPublicGlobalPerMinute = %d, want 30", cfg.AIPublicGlobalPerMinute)
	}
	if cfg.AIPublicMaxConcurrency != 2 {
		t.Fatalf("AIPublicMaxConcurrency = %d, want 2", cfg.AIPublicMaxConcurrency)
	}
}
