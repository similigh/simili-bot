package gemini

import "testing"

func TestResolveProviderPrefersGeminiWhenBothEnvKeysSet(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "gemini-env-key")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	provider, key, err := ResolveProvider("config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected provider %q, got %q", ProviderGemini, provider)
	}
	if key != "gemini-env-key" {
		t.Fatalf("expected Gemini env key, got %q", key)
	}
}

func TestResolveProviderUsesOpenAIEnvWhenGeminiMissing(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	provider, key, err := ResolveProvider("config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderOpenAI {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, provider)
	}
	if key != "sk-openai-env-key" {
		t.Fatalf("expected OpenAI env key, got %q", key)
	}
}

func TestResolveProviderFallsBackToConfigKeyInference(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	provider, key, err := ResolveProvider("sk-config-openai-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderOpenAI {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, provider)
	}
	if key != "sk-config-openai-key" {
		t.Fatalf("expected config key passthrough, got %q", key)
	}

	provider, key, err = ResolveProvider("gemini-config-key")
	if err != nil {
		t.Fatalf("ResolveProvider returned error: %v", err)
	}
	if provider != ProviderGemini {
		t.Fatalf("expected provider %q, got %q", ProviderGemini, provider)
	}
	if key != "gemini-config-key" {
		t.Fatalf("expected config key passthrough, got %q", key)
	}
}

func TestResolveProviderErrorsWhenNoKeyAvailable(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	_, _, err := ResolveProvider("")
	if err == nil {
		t.Fatal("expected error when no provider key is set")
	}
}

func TestNewLLMClientOpenAIDefaultModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	client, err := NewLLMClient("")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	if client.Provider() != string(ProviderOpenAI) {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, client.Provider())
	}
	if client.Model() != "gpt-5.2" {
		t.Fatalf("expected default OpenAI model %q, got %q", "gpt-5.2", client.Model())
	}
}

func TestNewLLMClientOpenAIExplicitModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-env-key")

	client, err := NewLLMClient("", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("NewLLMClient returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	if client.Provider() != string(ProviderOpenAI) {
		t.Fatalf("expected provider %q, got %q", ProviderOpenAI, client.Provider())
	}
	if client.Model() != "gpt-4o-mini" {
		t.Fatalf("expected model %q, got %q", "gpt-4o-mini", client.Model())
	}
}
