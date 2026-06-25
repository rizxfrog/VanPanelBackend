package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultClawHubBaseURL = "https://clawhub.ai"

// ClawHubConfig configures the ClawHub registry client.
type ClawHubConfig struct {
	BaseURL  string
	APIKey   string
	Registry string
}

// ClawHubClient provides a thin HTTP client for the ClawHub public registry.
type ClawHubClient struct {
	cfg    ClawHubConfig
	client *http.Client
}

// NewClawHubClient creates a new client. Empty BaseURL defaults to https://clawhub.ai.
func NewClawHubClient(cfg ClawHubConfig) *ClawHubClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultClawHubBaseURL
	}
	if cfg.Registry == "" {
		cfg.Registry = "clawhub"
	}
	return &ClawHubClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ClawHubSearchResult matches the frontend ClawHubSearchResult type.
type ClawHubSearchResult struct {
	Score       float64 `json:"score"`
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	Summary     string  `json:"summary,omitempty"`
	Version     string  `json:"version,omitempty"`
	UpdatedAt   int64   `json:"updatedAt,omitempty"`
	OwnerHandle string  `json:"ownerHandle,omitempty"`
	Owner       *struct {
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Image       string `json:"image,omitempty"`
	} `json:"owner,omitempty"`
}

// ClawHubSkillDetail matches the frontend ClawHubSkillDetail type.
type ClawHubSkillDetail struct {
	Skill *struct {
		Slug        string            `json:"slug"`
		DisplayName string            `json:"displayName"`
		Summary     string            `json:"summary,omitempty"`
		Tags        map[string]string `json:"tags,omitempty"`
		CreatedAt   int64             `json:"createdAt"`
		UpdatedAt   int64             `json:"updatedAt"`
	} `json:"skill"`
	LatestVersion *struct {
		Version   string `json:"version"`
		CreatedAt int64  `json:"createdAt"`
		Changelog string `json:"changelog,omitempty"`
	} `json:"latestVersion,omitempty"`
	Metadata *struct {
		OS      []string `json:"os,omitempty"`
		Systems []string `json:"systems,omitempty"`
	} `json:"metadata,omitempty"`
	Owner *struct {
		Handle      string `json:"handle,omitempty"`
		DisplayName string `json:"displayName,omitempty"`
		Image       string `json:"image,omitempty"`
	} `json:"owner,omitempty"`
}

// ClawHubSecurityVerdictRequest is used for the batch verdict endpoint.
type ClawHubSecurityVerdictRequest struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
}

// ClawHubSecurityVerdict matches the frontend ClawHubSkillSecurityVerdict type.
type ClawHubSecurityVerdict struct {
	Registry             string   `json:"registry"`
	OK                   bool     `json:"ok"`
	Decision             string   `json:"decision"`
	Reasons              []string `json:"reasons"`
	RequestedSlug        string   `json:"requestedSlug"`
	RequestedVersion     string   `json:"requestedVersion"`
	Slug                 string   `json:"slug,omitempty"`
	Version              string   `json:"version,omitempty"`
	DisplayName          string   `json:"displayName,omitempty"`
	PublisherHandle      string   `json:"publisherHandle,omitempty"`
	PublisherDisplayName string   `json:"publisherDisplayName,omitempty"`
	CreatedAt            int64    `json:"createdAt,omitempty"`
	CheckedAt            int64    `json:"checkedAt,omitempty"`
	SkillURL             string   `json:"skillUrl,omitempty"`
	SecurityAuditURL     string   `json:"securityAuditUrl,omitempty"`
	SecurityStatus       string   `json:"securityStatus,omitempty"`
	SecurityPassed       bool     `json:"securityPassed,omitempty"`
	Error                *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
}

// ClawHubSecurityVerdictsResponse wraps the batch verdict endpoint.
type ClawHubSecurityVerdictsResponse struct {
	Schema string                   `json:"schema"`
	Items  []ClawHubSecurityVerdict `json:"items"`
}

// Search queries ClawHub for skills matching the given query.
func (c *ClawHubClient) Search(ctx context.Context, query string, limit int) ([]ClawHubSearchResult, error) {
	u, err := url.Parse(c.cfg.BaseURL + "/api/v1/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clawhub search returned %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Results []ClawHubSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode clawhub search response: %w", err)
	}
	return payload.Results, nil
}

// Detail fetches the detail page for a single skill slug.
func (c *ClawHubClient) Detail(ctx context.Context, slug string) (*ClawHubSkillDetail, error) {
	u := c.cfg.BaseURL + "/api/v1/skills/" + url.PathEscape(slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub detail request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &ClawHubSkillDetail{Skill: nil}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clawhub detail returned %d: %s", resp.StatusCode, string(body))
	}

	var detail ClawHubSkillDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decode clawhub detail response: %w", err)
	}
	return &detail, nil
}

// Verify fetches a single skill security verdict via the public verify endpoint.
func (c *ClawHubClient) Verify(ctx context.Context, slug, version string) (*ClawHubSecurityVerdict, error) {
	u, err := url.Parse(c.cfg.BaseURL + "/api/v1/skills/" + url.PathEscape(slug) + "/verify")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if version != "" {
		q.Set("version", version)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &ClawHubSecurityVerdict{
			Registry:         c.cfg.Registry,
			RequestedSlug:    slug,
			RequestedVersion: version,
			OK:               false,
			Decision:         "not_found",
			Reasons:          []string{"skill not found"},
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clawhub verify returned %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		OK       bool     `json:"ok"`
		Decision string   `json:"decision"`
		Reasons  []string `json:"reasons"`
		Security *struct {
			Status string `json:"status"`
			Passed bool   `json:"passed"`
		} `json:"security,omitempty"`
		SkillURL         string `json:"skillUrl,omitempty"`
		SecurityAuditURL string `json:"securityAuditUrl,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode clawhub verify response: %w", err)
	}

	verdict := &ClawHubSecurityVerdict{
		Registry:         c.cfg.Registry,
		RequestedSlug:    slug,
		RequestedVersion: version,
		Slug:             slug,
		Version:          version,
		OK:               raw.OK,
		Decision:         raw.Decision,
		Reasons:          raw.Reasons,
		SkillURL:         raw.SkillURL,
		SecurityAuditURL: raw.SecurityAuditURL,
	}
	if raw.Security != nil {
		verdict.SecurityStatus = raw.Security.Status
		verdict.SecurityPassed = raw.Security.Passed
	}
	return verdict, nil
}

// SecurityVerdicts fetches batch verdicts for the requested skill versions.
// Falls back to per-skill Verify when no API key is configured.
func (c *ClawHubClient) SecurityVerdicts(ctx context.Context, items []ClawHubSecurityVerdictRequest) ([]ClawHubSecurityVerdict, error) {
	if c.cfg.APIKey == "" || len(items) == 0 {
		// Fallback to per-skill verify endpoint.
		results := make([]ClawHubSecurityVerdict, 0, len(items))
		for _, it := range items {
			v, err := c.Verify(ctx, it.Slug, it.Version)
			if err != nil {
				results = append(results, ClawHubSecurityVerdict{
					Registry:         c.cfg.Registry,
					RequestedSlug:    it.Slug,
					RequestedVersion: it.Version,
					OK:               false,
					Decision:         "error",
					Reasons:          []string{err.Error()},
				})
				continue
			}
			results = append(results, *v)
		}
		return results, nil
	}

	u := c.cfg.BaseURL + "/api/v1/skills/-/security-verdicts"
	body, err := json.Marshal(map[string]interface{}{"items": items})
	if err != nil {
		return nil, fmt.Errorf("marshal security verdicts request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub security verdicts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clawhub security verdicts returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var payload ClawHubSecurityVerdictsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode clawhub security verdicts response: %w", err)
	}
	// Ensure registry field is populated for callers.
	for i := range payload.Items {
		if payload.Items[i].Registry == "" {
			payload.Items[i].Registry = c.cfg.Registry
		}
	}
	return payload.Items, nil
}

// DownloadResult holds either ZIP bytes or a GitHub source handoff.
type DownloadResult struct {
	ContentType string
	Body        io.ReadCloser
	// Handoff is set when ClawHub returns a GitHub source reference instead of bytes.
	Handoff *ClawHubGitHubHandoff
}

// ClawHubGitHubHandoff is returned for GitHub-backed skills.
type ClawHubGitHubHandoff struct {
	SourceRef   string `json:"sourceRef"`
	Repo        string `json:"repo"`
	Commit      string `json:"commit"`
	Path        string `json:"path"`
	ContentHash string `json:"contentHash"`
	ArchiveURL  string `json:"archiveUrl"`
}

// Download fetches a skill archive from ClawHub. Supports ZIP bytes and GitHub handoffs.
func (c *ClawHubClient) Download(ctx context.Context, slug, version string) (*DownloadResult, error) {
	u, err := url.Parse(c.cfg.BaseURL + "/api/v1/download")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("slug", slug)
	if version != "" {
		q.Set("version", version)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/zip, application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clawhub download request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("clawhub download returned %d: %s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		defer resp.Body.Close()
		var handoff ClawHubGitHubHandoff
		if err := json.NewDecoder(resp.Body).Decode(&handoff); err != nil {
			return nil, fmt.Errorf("decode github handoff: %w", err)
		}
		return nil, fmt.Errorf("github handoff not supported yet (repo=%s, path=%s)", handoff.Repo, handoff.Path)
	}

	return &DownloadResult{
		ContentType: ct,
		Body:        resp.Body,
	}, nil
}
