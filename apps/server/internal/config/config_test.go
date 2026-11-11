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
