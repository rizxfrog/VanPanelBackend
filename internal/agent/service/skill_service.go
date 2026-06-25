package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	agentskill "github.com/rizxfrog/VanPanelBackend/internal/agent/skill"
)

// SkillsConfig configures the skill service.
type SkillsConfig struct {
	BaseDir        string
	ClawHubBaseURL string
	ClawHubAPIKey  string
	Registry       string
}

// SkillService provides the Gateway-facing skill management layer.
type SkillService struct {
	store   *agentskill.SkillStore
	clawHub *agentskill.ClawHubClient
	cfg     SkillsConfig
	logger  *zap.Logger
}

// NewSkillService creates a skill service.
func NewSkillService(store *agentskill.SkillStore, clawHub *agentskill.ClawHubClient, cfg SkillsConfig, logger *zap.Logger) *SkillService {
	if cfg.BaseDir == "" {
		cfg.BaseDir = "data/skills"
	}
	if cfg.Registry == "" {
		cfg.Registry = "clawhub"
	}
	return &SkillService{
		store:   store,
		clawHub: clawHub,
		cfg:     cfg,
		logger:  logger,
	}
}

// SkillStatusReport mirrors the frontend SkillStatusReport type.
type SkillStatusReport struct {
	WorkspaceDir     string             `json:"workspaceDir"`
	ManagedSkillsDir string             `json:"managedSkillsDir"`
	AgentID          string             `json:"agentId,omitempty"`
	AgentSkillFilter []string           `json:"agentSkillFilter,omitempty"`
	Skills           []SkillStatusEntry `json:"skills"`
}

// SkillStatusEntry mirrors the frontend SkillStatusEntry type.
type SkillStatusEntry struct {
	Name                 string                    `json:"name"`
	Description          string                    `json:"description"`
	Source               string                    `json:"source"`
	FilePath             string                    `json:"filePath"`
	BaseDir              string                    `json:"baseDir"`
	SkillKey             string                    `json:"skillKey"`
	Bundled              bool                      `json:"bundled,omitempty"`
	PrimaryEnv           string                    `json:"primaryEnv,omitempty"`
	Emoji                string                    `json:"emoji,omitempty"`
	Homepage             string                    `json:"homepage,omitempty"`
	Always               bool                      `json:"always"`
	Disabled             bool                      `json:"disabled"`
	BlockedByAllowlist   bool                      `json:"blockedByAllowlist"`
	BlockedByAgentFilter bool                      `json:"blockedByAgentFilter,omitempty"`
	Eligible             bool                      `json:"eligible"`
	ModelVisible         bool                      `json:"modelVisible,omitempty"`
	UserInvocable        bool                      `json:"userInvocable,omitempty"`
	CommandVisible       bool                      `json:"commandVisible,omitempty"`
	Requirements         SkillRequirements         `json:"requirements"`
	Missing              SkillMissing              `json:"missing"`
	ConfigChecks         []SkillsStatusConfigCheck `json:"configChecks"`
	Install              []SkillInstallOption      `json:"install"`
	ClawHub              *SkillClawHubLink         `json:"clawhub,omitempty"`
	SkillCard            *SkillCardStatus          `json:"skillCard,omitempty"`
}

// SkillRequirements mirrors the frontend requirements shape.
type SkillRequirements struct {
	AnyBins []string `json:"anyBins,omitempty"`
	Bins    []string `json:"bins"`
	Env     []string `json:"env"`
	Config  []string `json:"config"`
	OS      []string `json:"os"`
}

// SkillMissing mirrors the frontend missing shape.
type SkillMissing struct {
	Bins   []string `json:"bins"`
	Env    []string `json:"env"`
	Config []string `json:"config"`
	OS     []string `json:"os"`
}

// SkillsStatusConfigCheck mirrors the frontend config check shape.
type SkillsStatusConfigCheck struct {
	Path      string `json:"path"`
	Satisfied bool   `json:"satisfied"`
}

// SkillInstallOption mirrors the frontend SkillInstallOption type.
type SkillInstallOption struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"`
	Label string   `json:"label"`
	Bins  []string `json:"bins"`
}

// SkillClawHubLink mirrors the frontend SkillClawHubLink type.
type SkillClawHubLink struct {
	Status           string `json:"status"`
	Valid            bool   `json:"valid"`
	Reason           string `json:"reason,omitempty"`
	Registry         string `json:"registry,omitempty"`
	Slug             string `json:"slug,omitempty"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstalledAt      int64  `json:"installedAt,omitempty"`
	OriginPath       string `json:"originPath,omitempty"`
	LockPath         string `json:"lockPath,omitempty"`
}

// SkillCardStatus mirrors the frontend SkillCardStatus type.
type SkillCardStatus struct {
	Present   bool   `json:"present"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

// SkillCardResponse mirrors the frontend skillCard response shape.
type SkillCardResponse struct {
	Schema    string `json:"schema"`
	SkillKey  string `json:"skillKey"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	Content   string `json:"content"`
}

// UpdateSkillRequest supports enable toggle and API key storage.
type UpdateSkillRequest struct {
	SkillKey string `json:"skillKey"`
	Enabled  *bool  `json:"enabled,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
}

// InstallSkillRequest supports both local and ClawHub installs.
type InstallSkillRequest struct {
	Name                          string `json:"name"`
	InstallID                     string `json:"installId,omitempty"`
	DangerouslyForceUnsafeInstall bool   `json:"dangerouslyForceUnsafeInstall"`
	TimeoutMs                     int    `json:"timeoutMs,omitempty"`
	Source                        string `json:"source,omitempty"`
	Slug                          string `json:"slug,omitempty"`
	Version                       string `json:"version,omitempty"`
}

// Status returns a skill status report compatible with the OpenClaw UI.
func (s *SkillService) Status(ctx context.Context, agentID string) (*SkillStatusReport, error) {
	skills, err := s.store.ListSkills(ctx)
	if err != nil {
		return nil, fmt.Errorf("list skills failed: %w", err)
	}

	absBase, err := filepath.Abs(s.cfg.BaseDir)
	if err != nil {
		absBase = s.cfg.BaseDir
	}

	entries := make([]SkillStatusEntry, 0, len(skills))
	for _, sk := range skills {
		entries = append(entries, s.buildStatusEntry(sk, agentID))
	}

	// Sort by name for stable UI order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SkillKey < entries[j].SkillKey
	})

	return &SkillStatusReport{
		WorkspaceDir:     absBase,
		ManagedSkillsDir: absBase,
		AgentID:          agentID,
		Skills:           entries,
	}, nil
}

func (s *SkillService) buildStatusEntry(sk *agentskill.Skill, agentID string) SkillStatusEntry {
	reqs := SkillRequirements{
		AnyBins: sk.Meta.Requirements.AnyBins,
		Bins:    sk.Meta.Requirements.Bins,
		Env:     sk.Meta.Requirements.Env,
		Config:  sk.Meta.Requirements.Config,
		OS:      sk.Meta.Requirements.OS,
	}
	missing := computeMissing(reqs)
	eligible := len(missing.Bins) == 0 && len(missing.Env) == 0 && len(missing.Config) == 0 && len(missing.OS) == 0

	configChecks := make([]SkillsStatusConfigCheck, 0, len(sk.Meta.Requirements.Config))
	for _, cfgPath := range sk.Meta.Requirements.Config {
		configChecks = append(configChecks, SkillsStatusConfigCheck{
			Path:      cfgPath,
			Satisfied: fileExists(cfgPath),
		})
	}

	install := make([]SkillInstallOption, 0, len(sk.Meta.Install))
	for _, opt := range sk.Meta.Install {
		install = append(install, SkillInstallOption{
			ID:    opt.ID,
			Kind:  opt.Kind,
			Label: opt.Label,
			Bins:  opt.Bins,
		})
	}

	source := sk.Meta.Source
	if source == "" {
		source = string(sk.Source)
	}

	entry := SkillStatusEntry{
		Name:               sk.Meta.Name,
		Description:        sk.Meta.Description,
		Source:             source,
		FilePath:           filepath.Join(sk.Path, "SKILL.md"),
		BaseDir:            sk.Path,
		SkillKey:           sk.Meta.Name,
		Bundled:            sk.Meta.Bundled,
		PrimaryEnv:         sk.Meta.PrimaryEnv,
		Emoji:              sk.Meta.Emoji,
		Homepage:           sk.Meta.Homepage,
		Always:             sk.Meta.Always,
		Disabled:           sk.State == agentskill.SkillStateArchived,
		BlockedByAllowlist: false,
		Eligible:           eligible,
		ModelVisible:       eligible,
		UserInvocable:      eligible,
		CommandVisible:     eligible,
		Requirements:       reqs,
		Missing:            missing,
		ConfigChecks:       configChecks,
		Install:            install,
	}

	if agentID != "" {
		entry.BlockedByAgentFilter = false
	}

	if sk.Meta.ClawHub != nil {
		entry.ClawHub = &SkillClawHubLink{
			Status:           "linked",
			Valid:            true,
			Registry:         sk.Meta.ClawHub.Registry,
			Slug:             sk.Meta.ClawHub.Slug,
			InstalledVersion: sk.Meta.ClawHub.InstalledVersion,
			InstalledAt:      sk.Meta.ClawHub.InstalledAt,
		}
	}

	if sk.Meta.SkillCard != nil {
		fullPath := filepath.Join(sk.Path, sk.Meta.SkillCard.Path)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			entry.SkillCard = &SkillCardStatus{
				Present:   true,
				Path:      sk.Meta.SkillCard.Path,
				SizeBytes: info.Size(),
			}
		}
	}

	return entry
}

func computeMissing(reqs SkillRequirements) SkillMissing {
	var m SkillMissing
	for _, bin := range reqs.Bins {
		if !commandExists(bin) {
			m.Bins = append(m.Bins, bin)
		}
	}
	for _, env := range reqs.Env {
		if os.Getenv(env) == "" {
			m.Env = append(m.Env, env)
		}
	}
	for _, cfg := range reqs.Config {
		if !fileExists(cfg) {
			m.Config = append(m.Config, cfg)
		}
	}
	if len(reqs.OS) > 0 {
		matched := false
		for _, osName := range reqs.OS {
			if strings.EqualFold(osName, runtime.GOOS) {
				matched = true
				break
			}
		}
		if !matched {
			m.OS = append(m.OS, runtime.GOOS)
		}
	}
	return m
}

func commandExists(name string) bool {
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	// Also accept absolute paths.
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	// Expand ~ to home directory.
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if _, err := os.Stat(filepath.Join(home, path[2:])); err == nil {
				return true
			}
		}
	}
	return false
}

// GetSkillCard returns the full content of a skill's SKILL.md file.
func (s *SkillService) GetSkillCard(ctx context.Context, skillKey string) (*SkillCardResponse, error) {
	sk, err := s.store.GetSkill(ctx, skillKey)
	if err != nil {
		return nil, fmt.Errorf("get skill failed: %w", err)
	}

	fullPath := filepath.Join(sk.Path, "SKILL.md")
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("stat skill card failed: %w", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read skill card failed: %w", err)
	}

	return &SkillCardResponse{
		Schema:    "agentops.skills.skill-card.v1",
		SkillKey:  sk.Meta.Name,
		Path:      fullPath,
		SizeBytes: info.Size(),
		Content:   string(content),
	}, nil
}

// UpdateSkill toggles a skill's enabled/archived state or saves an API key.
func (s *SkillService) UpdateSkill(ctx context.Context, req UpdateSkillRequest) error {
	if req.SkillKey == "" {
		return fmt.Errorf("skillKey is required")
	}

	if req.Enabled != nil {
		state := agentskill.SkillStateActive
		if !*req.Enabled {
			state = agentskill.SkillStateArchived
		}
		if err := s.store.UpdateState(ctx, req.SkillKey, state); err != nil {
			return fmt.Errorf("update skill state failed: %w", err)
		}
	}

	if req.APIKey != "" {
		if err := s.saveSkillAPIKey(ctx, req.SkillKey, req.APIKey); err != nil {
			return fmt.Errorf("save skill api key failed: %w", err)
		}
	}

	return nil
}

func (s *SkillService) saveSkillAPIKey(ctx context.Context, skillKey, apiKey string) error {
	secretsPath := filepath.Join(s.cfg.BaseDir, ".secrets.json")
	data := map[string]interface{}{}
	if b, err := os.ReadFile(secretsPath); err == nil {
		_ = json.Unmarshal(b, &data)
	}

	skills, ok := data["skills"].(map[string]interface{})
	if !ok {
		skills = map[string]interface{}{}
		data["skills"] = skills
	}
	entries, ok := skills["entries"].(map[string]interface{})
	if !ok {
		entries = map[string]interface{}{}
		skills["entries"] = entries
	}
	entries[skillKey] = map[string]interface{}{"apiKey": apiKey}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(secretsPath, b, 0600)
}

// InstallSkill installs a skill either from a local install option or from ClawHub.
func (s *SkillService) InstallSkill(ctx context.Context, req InstallSkillRequest) (string, error) {
	if req.Source == "clawhub" || req.Slug != "" {
		return s.installFromClawHub(ctx, req.Slug, req.Version)
	}
	return s.installLocal(ctx, req)
}

func (s *SkillService) installLocal(ctx context.Context, req InstallSkillRequest) (string, error) {
	sk, err := s.store.GetSkill(ctx, req.Name)
	if err != nil {
		return "", fmt.Errorf("skill not found: %w", err)
	}

	var spec *agentskill.SkillInstallSpec
	for i := range sk.Meta.Install {
		if sk.Meta.Install[i].ID == req.InstallID {
			spec = &sk.Meta.Install[i]
			break
		}
	}
	if spec == nil {
		return "", fmt.Errorf("install option %q not found for skill %q", req.InstallID, req.Name)
	}

	cmd, err := buildInstallCommand(spec)
	if err != nil {
		return "", err
	}
	if cmd == "" {
		return "No install command configured", nil
	}

	timeout := 120 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	splitCmd := strings.Fields(cmd)
	if len(splitCmd) == 0 {
		return "", fmt.Errorf("empty install command")
	}
	c := exec.CommandContext(execCtx, splitCmd[0], splitCmd[1:]...)
	out, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("install failed: %w\noutput: %s", err, string(out))
	}

	msg := fmt.Sprintf("Installed %s via %s", req.Name, spec.Kind)
	if len(out) > 0 {
		msg += "\n" + string(out)
	}
	return msg, nil
}

func buildInstallCommand(spec *agentskill.SkillInstallSpec) (string, error) {
	if spec.Kind == "" {
		return "", fmt.Errorf("install option missing kind")
	}
	// If the skill metadata includes an explicit command, use it.
	if spec.Command != "" {
		return spec.Command, nil
	}

	switch spec.Kind {
	case "brew":
		if len(spec.Bins) > 0 {
			return fmt.Sprintf("brew install %s", strings.Join(spec.Bins, " ")), nil
		}
		return fmt.Sprintf("brew install %s", spec.Label), nil
	case "node":
		if len(spec.Bins) > 0 {
			return fmt.Sprintf("npm install -g %s", strings.Join(spec.Bins, " ")), nil
		}
		return fmt.Sprintf("npm install -g %s", spec.Label), nil
	case "go":
		if len(spec.Bins) > 0 {
			return fmt.Sprintf("go install %s", strings.Join(spec.Bins, " ")), nil
		}
		return fmt.Sprintf("go install %s", spec.Label), nil
	case "uv":
		if len(spec.Bins) > 0 {
			return fmt.Sprintf("uv tool install %s", strings.Join(spec.Bins, " ")), nil
		}
		return fmt.Sprintf("uv tool install %s", spec.Label), nil
	case "download":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported install kind: %s", spec.Kind)
	}
}

func (s *SkillService) installFromClawHub(ctx context.Context, slug, version string) (string, error) {
	if slug == "" {
		return "", fmt.Errorf("slug is required for clawhub install")
	}

	detail, err := s.clawHub.Detail(ctx, slug)
	if err != nil {
		return "", fmt.Errorf("fetch clawhub detail failed: %w", err)
	}
	if detail.Skill == nil {
		return "", fmt.Errorf("skill %q not found on ClawHub", slug)
	}
	if version == "" && detail.LatestVersion != nil {
		version = detail.LatestVersion.Version
	}

	result, err := s.clawHub.Download(ctx, slug, version)
	if err != nil {
		return "", fmt.Errorf("download skill failed: %w", err)
	}
	defer result.Body.Close()

	category := "clawhub"
	targetDir := filepath.Join(s.cfg.BaseDir, category, slug)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("create target dir failed: %w", err)
	}

	if err := unzipToDir(result.Body, targetDir); err != nil {
		return "", fmt.Errorf("extract skill archive failed: %w", err)
	}

	// Create or update SKILL.md frontmatter with clawhub metadata.
	if err := s.updateClawHubMetadata(targetDir, slug, version, detail); err != nil {
		s.logger.Warn("failed to update clawhub metadata", zap.Error(err))
	}

	return fmt.Sprintf("Installed %s (%s) from ClawHub", slug, version), nil
}

func unzipToDir(r io.ReadCloser, targetDir string) error {
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		// Trim a single top-level directory if present.
		name := f.Name
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			name = parts[1]
		}
		if name == "" {
			continue
		}
		if strings.Contains(name, "..") {
			continue
		}

		path := filepath.Join(targetDir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		fr, err := f.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, fr)
		fr.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SkillService) updateClawHubMetadata(targetDir, slug, version string, detail *agentskill.ClawHubSkillDetail) error {
	skillPath := filepath.Join(targetDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return err
	}
	meta, content, err := agentskill.ParseFullSkillMD(data)
	if err != nil {
		return err
	}
	if meta.ClawHub == nil {
		meta.ClawHub = &agentskill.SkillClawHubMeta{}
	}
	meta.ClawHub.Registry = s.cfg.Registry
	meta.ClawHub.Slug = slug
	meta.ClawHub.Version = version
	meta.ClawHub.InstalledVersion = version
	meta.ClawHub.InstalledAt = time.Now().Unix()
	return agentskill.WriteSkillMD(skillPath, *meta, content)
}

// SearchClawHub forwards the search to the ClawHub registry.
func (s *SkillService) SearchClawHub(ctx context.Context, query string, limit int) ([]agentskill.ClawHubSearchResult, error) {
	if s.clawHub == nil {
		return []agentskill.ClawHubSearchResult{}, nil
	}
	return s.clawHub.Search(ctx, query, limit)
}

// GetClawHubDetail forwards the detail request to the ClawHub registry.
func (s *SkillService) GetClawHubDetail(ctx context.Context, slug string) (*agentskill.ClawHubSkillDetail, error) {
	if s.clawHub == nil {
		return &agentskill.ClawHubSkillDetail{Skill: nil}, nil
	}
	return s.clawHub.Detail(ctx, slug)
}

// SecurityVerdictsRequest allows callers to request verdicts for specific skills.
type SecurityVerdictsRequest struct {
	Items []agentskill.ClawHubSecurityVerdictRequest `json:"items,omitempty"`
}

// SecurityVerdicts returns ClawHub security verdicts for the requested skills.
// If no items are provided, verdicts are computed for locally installed ClawHub-linked skills.
func (s *SkillService) SecurityVerdicts(ctx context.Context, req SecurityVerdictsRequest) (*agentskill.ClawHubSecurityVerdictsResponse, error) {
	if s.clawHub == nil {
		return &agentskill.ClawHubSecurityVerdictsResponse{
			Schema: "agentops.skills.security-verdicts.v1",
			Items:  []agentskill.ClawHubSecurityVerdict{},
		}, nil
	}

	items := req.Items
	if len(items) == 0 {
		// Collect linked skills.
		skills, err := s.store.ListSkills(ctx)
		if err != nil {
			return nil, fmt.Errorf("list skills failed: %w", err)
		}
		for _, sk := range skills {
			if sk.Meta.ClawHub != nil && sk.Meta.ClawHub.Slug != "" {
				items = append(items, agentskill.ClawHubSecurityVerdictRequest{
					Slug:    sk.Meta.ClawHub.Slug,
					Version: sk.Meta.ClawHub.InstalledVersion,
				})
			}
		}
	}

	verdicts, err := s.clawHub.SecurityVerdicts(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("fetch security verdicts failed: %w", err)
	}
	return &agentskill.ClawHubSecurityVerdictsResponse{
		Schema: "agentops.skills.security-verdicts.v1",
		Items:  verdicts,
	}, nil
}

// Bins returns an empty map for now (placeholder for future per-skill binary introspection).
func (s *SkillService) Bins(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
