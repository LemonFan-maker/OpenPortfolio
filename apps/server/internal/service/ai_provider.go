package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"portfolio-hub/apps/server/internal/config"
)

type aiSettings struct {
	provider string
	apiKey   string
	baseURL  string
	model    string
}

func (s *AIService) aiSettings() (aiSettings, error) {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.AIProvider))
	if provider == "" {
		if strings.TrimSpace(s.cfg.OpenCodeAPIKey) != "" {
			provider = config.AIProviderOpenCode
		} else {
			provider = config.AIProviderOpenAI
		}
	}

	switch provider {
	case config.AIProviderOpenCode:
		apiKey := strings.TrimSpace(s.cfg.OpenCodeAPIKey)
		if apiKey == "" {
			return aiSettings{}, errors.New("opencode api key is not configured (set OPENCODE_API_KEY)")
		}
		baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.OpenCodeBaseURL), "/")
		if baseURL == "" {
			baseURL = config.DefaultOpenCodeBaseURL
		}
		model := strings.TrimSpace(s.cfg.AIModel)
		if model == "" {
			model = config.DefaultOpenCodeModel
		}
		return aiSettings{provider: provider, apiKey: apiKey, baseURL: baseURL, model: model}, nil
	case config.AIProviderOpenAI:
		apiKey := strings.TrimSpace(s.cfg.AIAPIKey)
		if apiKey == "" {
			return aiSettings{}, errors.New("ai api key is not configured (set AI_API_KEY)")
		}
		baseURL := strings.TrimRight(strings.TrimSpace(s.cfg.AIBaseURL), "/")
		if baseURL == "" {
			baseURL = config.DefaultOpenAIBaseURL
		}
		model := strings.TrimSpace(s.cfg.AIModel)
		if model == "" {
			model = config.DefaultOpenAIModel
		}
		return aiSettings{provider: provider, apiKey: apiKey, baseURL: baseURL, model: model}, nil
	default:
		return aiSettings{}, fmt.Errorf("unsupported AI_PROVIDER %q (expected %s or %s)", s.cfg.AIProvider, config.AIProviderOpenCode, config.AIProviderOpenAI)
	}
}

func newAISessionID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return fmt.Sprintf("portfolio-hub-%d", time.Now().UnixNano())
	}
	return "portfolio-hub-" + hex.EncodeToString(id[:])
}

func (s *AIService) requestAIContent(ctx context.Context, sessionID string, systemContent string, userContent string, temperature float64) (string, string, error) {
	settings, err := s.aiSettings()
	if err != nil {
		return "", "", err
	}
	if settings.provider == config.AIProviderOpenCode && strings.TrimSpace(sessionID) == "" {
		sessionID = newAISessionID()
	}

	payload := map[string]any{
		"model": settings.model,
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

	endpoint := settings.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+settings.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "portfolio-hub/1.0")
	if settings.provider == config.AIProviderOpenCode {
		req.Header.Set("x-opencode-session", sessionID)
	}

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
		return "", rawBody, fmt.Errorf("%s api returned %d: %s", settings.provider, resp.StatusCode, rawBody)
	}

	content, err := decodeAIContent(body)
	if err != nil {
		return "", rawBody, err
	}
	return content, rawBody, nil
}
