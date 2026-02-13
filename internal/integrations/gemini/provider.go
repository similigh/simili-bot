package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider identifies the active AI provider.
type Provider string

const (
	ProviderGemini Provider = "gemini"
	ProviderOpenAI Provider = "openai"
)

const openAIBaseURL = "https://api.openai.com"

// ResolveProvider selects provider/key using environment variables and config key.
//
// Selection order:
// 1. If both GEMINI_API_KEY and OPENAI_API_KEY are set, Gemini wins.
// 2. If only one env key is set, that provider is selected.
// 3. If no env keys are set, fallback to config api key.
func ResolveProvider(apiKey string) (Provider, string, error) {
	geminiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	configKey := strings.TrimSpace(apiKey)

	switch {
	case geminiKey != "" && openAIKey != "":
		return ProviderGemini, geminiKey, nil
	case geminiKey != "":
		return ProviderGemini, geminiKey, nil
	case openAIKey != "":
		return ProviderOpenAI, openAIKey, nil
	case configKey != "":
		return inferProviderFromKey(configKey), configKey, nil
	default:
		return "", "", fmt.Errorf("no AI API key found (set GEMINI_API_KEY or OPENAI_API_KEY)")
	}
}

func inferProviderFromKey(apiKey string) Provider {
	// OpenAI keys commonly use sk-* prefixes. Fall back to Gemini for compatibility.
	if strings.HasPrefix(strings.TrimSpace(apiKey), "sk-") {
		return ProviderOpenAI
	}
	return ProviderGemini
}

func callOpenAIJSON(ctx context.Context, httpClient *http.Client, apiKey, endpoint string, in, out interface{}) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("OPENAI_API_KEY is required")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIBaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read OpenAI response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, extractOpenAIErrorMessage(respBody))
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	return nil
}

func extractOpenAIErrorMessage(body []byte) string {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil {
		msg := strings.TrimSpace(errResp.Error.Message)
		if msg != "" {
			return msg
		}
	}
	return strings.TrimSpace(string(body))
}
