package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LLMAuditorConfig holds the static connection config for the auditor LLM.
// The dynamic config (system prompt, model, temperature) is loaded from DB at query time.
type LLMAuditorConfig struct {
	BaseURL string
	APIKey  string
}

// LLMAuditor 独立 LLM 注入审查器
type LLMAuditor struct {
	cfg    LLMAuditorConfig
	client *http.Client
}

// LLMAuditResult represents the structured output from the auditor LLM
type LLMAuditResult struct {
	Safe   bool   `json:"safe"`
	Reason string `json:"reason"`
	Intent string `json:"intent"`
}

// NewLLMAuditor creates an LLM auditor with the given base URL and API key.
// The timeout is fixed at 10s for auditing (should be fast).
func NewLLMAuditor(cfg LLMAuditorConfig) *LLMAuditor {
	return &LLMAuditor{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Audit sends the user input to the auditor LLM for safety review.
// model, systemPrompt, temperature, maxTokens are passed per-call (loaded from DB).
func (a *LLMAuditor) Audit(ctx context.Context, userInput string, model string, systemPrompt string, temperature float64, maxTokens int, maxRetries int) (*LLMAuditResult, error) {
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userInput},
		},
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := a.doRequest(ctx, payload)
		if err == nil {
			return result, nil
		}
		if attempt == maxRetries {
			return nil, fmt.Errorf("LLM auditor max retries exceeded: %w", err)
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return nil, fmt.Errorf("LLM auditor max retries exceeded")
}

func (a *LLMAuditor) doRequest(ctx context.Context, payload map[string]any) (*LLMAuditResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := a.cfg.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.APIKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auditor request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auditor HTTP %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("auditor decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("auditor empty response")
	}

	var ar LLMAuditResult
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &ar); err != nil {
		return nil, fmt.Errorf("auditor parse: %w (content: %s)", err, result.Choices[0].Message.Content)
	}
	return &ar, nil
}
