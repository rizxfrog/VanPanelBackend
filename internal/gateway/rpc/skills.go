package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	// Read methods
	gateway.RegisterMethod("skills.status", string(gateway.ScopeRead), handleSkillsStatus)
	gateway.RegisterMethod("skills.search", string(gateway.ScopeRead), handleSkillsSearch)
	gateway.RegisterMethod("skills.detail", string(gateway.ScopeRead), handleSkillsDetail)
	gateway.RegisterMethod("skills.securityVerdicts", string(gateway.ScopeRead), handleSkillsSecurityVerdicts)
	gateway.RegisterMethod("skills.skillCard", string(gateway.ScopeRead), handleSkillsSkillCard)
	gateway.RegisterMethod("skills.bins", string(gateway.ScopeRead), handleSkillsBins)
	gateway.RegisterMethod("skills.proposals.list", string(gateway.ScopeRead), handleSkillsProposalsList)
	gateway.RegisterMethod("skills.proposals.inspect", string(gateway.ScopeRead), handleSkillsProposalsInspect)

	// Admin methods
	gateway.RegisterMethod("skills.upload.begin", string(gateway.ScopeAdmin), handleSkillsUploadBegin)
	gateway.RegisterMethod("skills.upload.chunk", string(gateway.ScopeAdmin), handleSkillsUploadChunk)
	gateway.RegisterMethod("skills.upload.commit", string(gateway.ScopeAdmin), handleSkillsUploadCommit)
	gateway.RegisterMethod("skills.install", string(gateway.ScopeAdmin), handleSkillsInstall)
	gateway.RegisterMethod("skills.update", string(gateway.ScopeAdmin), handleSkillsUpdate)
	gateway.RegisterMethod("skills.proposals.create", string(gateway.ScopeAdmin), handleSkillsProposalsCreate)
	gateway.RegisterMethod("skills.proposals.update", string(gateway.ScopeAdmin), handleSkillsProposalsUpdate)
	gateway.RegisterMethod("skills.proposals.revise", string(gateway.ScopeAdmin), handleSkillsProposalsRevise)
	gateway.RegisterMethod("skills.proposals.requestRevision", string(gateway.ScopeAdmin), handleSkillsProposalsRequestRevision)
	gateway.RegisterMethod("skills.proposals.apply", string(gateway.ScopeAdmin), handleSkillsProposalsApply)
	gateway.RegisterMethod("skills.proposals.reject", string(gateway.ScopeAdmin), handleSkillsProposalsReject)
	gateway.RegisterMethod("skills.proposals.quarantine", string(gateway.ScopeAdmin), handleSkillsProposalsQuarantine)
}

func handleSkillsStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"status": "ok", "skills": []interface{}{}}, nil
}
func handleSkillsSearch(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"results": []interface{}{}}, nil
}
func handleSkillsDetail(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"skill": nil}, nil
}
func handleSkillsSecurityVerdicts(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"verdicts": map[string]interface{}{}}, nil
}
func handleSkillsSkillCard(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"card": nil}, nil
}
func handleSkillsBins(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{}, nil
}
func handleSkillsUploadBegin(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsUploadChunk(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsUploadCommit(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsInstall(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsList(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"proposals": []interface{}{}}, nil
}
func handleSkillsProposalsInspect(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"proposal": nil}, nil
}
func handleSkillsProposalsCreate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsRevise(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsRequestRevision(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsApply(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsReject(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleSkillsProposalsQuarantine(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
