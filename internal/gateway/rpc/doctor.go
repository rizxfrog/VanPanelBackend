package rpc

import (
	"context"
	"encoding/json"

	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

func init() {
	gateway.RegisterMethod("doctor.memory.status", string(gateway.ScopeRead), handleDoctorMemoryStatus)
	gateway.RegisterMethod("doctor.memory.dreamDiary", string(gateway.ScopeRead), handleDoctorMemoryDreamDiary)
	gateway.RegisterMethod("doctor.memory.store", string(gateway.ScopeAdmin), handleDoctorMemoryStore)
	gateway.RegisterMethod("doctor.memory.update", string(gateway.ScopeAdmin), handleDoctorMemoryUpdate)
	gateway.RegisterMethod("doctor.memory.delete", string(gateway.ScopeAdmin), handleDoctorMemoryDelete)
	gateway.RegisterMethod("doctor.memory.clear", string(gateway.ScopeAdmin), handleDoctorMemoryClear)
	gateway.RegisterMethod("doctor.memory.compact", string(gateway.ScopeAdmin), handleDoctorMemoryCompact)
	gateway.RegisterMethod("doctor.memory.backup", string(gateway.ScopeAdmin), handleDoctorMemoryBackup)
	gateway.RegisterMethod("doctor.memory.restore", string(gateway.ScopeAdmin), handleDoctorMemoryRestore)
}

func handleDoctorMemoryStatus(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]string{"status": "ok"}, nil
}
func handleDoctorMemoryDreamDiary(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]interface{}{"entries": []interface{}{}}, nil
}
func handleDoctorMemoryStore(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryUpdate(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryDelete(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryClear(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryCompact(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryBackup(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
func handleDoctorMemoryRestore(ctx context.Context, conn *gateway.GatewayConnection, params json.RawMessage) (interface{}, error) {
	return map[string]bool{"ok": true}, nil
}
