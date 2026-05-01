package planner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentmodel "github.com/GoSimplicity/AI-CloudOps/internal/agent/model"
)

type Planner interface {
	Plan(ctx context.Context, message string, history []agentmodel.AgentMessage) (agentmodel.PlanResponse, error)
	Respond(ctx context.Context, message string, plan agentmodel.PlanResponse, results []agentmodel.ToolResult) (string, error)
}

type HTTPPlanner struct {
	baseURL string
	client  *http.Client
}

func NewHTTPPlanner(baseURL string) *HTTPPlanner {
	return &HTTPPlanner{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *HTTPPlanner) Plan(ctx context.Context, message string, history []agentmodel.AgentMessage) (agentmodel.PlanResponse, error) {
	var response agentmodel.PlanResponse
	err := p.post(ctx, "/api/v1/agent/plan", map[string]any{
		"message": message,
		"history": history,
	}, &response)
	if err != nil {
		return agentmodel.PlanResponse{}, err
	}
	return response, nil
}

func (p *HTTPPlanner) Respond(ctx context.Context, message string, plan agentmodel.PlanResponse, results []agentmodel.ToolResult) (string, error) {
	var response struct {
		Message string `json:"message"`
	}
	err := p.post(ctx, "/api/v1/agent/respond", map[string]any{
		"message": message,
		"plan":    plan,
		"results": results,
	}, &response)
	if err != nil {
		return "", err
	}
	return response.Message, nil
}

func (p *HTTPPlanner) post(ctx context.Context, path string, payload any, out any) error {
	if p == nil || p.baseURL == "" {
		return fmt.Errorf("agent planner is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("agent planner returned %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
