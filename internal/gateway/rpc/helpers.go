package rpc

import "fmt"

// requireAgentSvc checks that agentSvc is initialized and returns an error if not.
func requireAgentSvc() error {
	if agentSvc == nil {
		return fmt.Errorf("AgentService 未初始化")
	}
	return nil
}

// buildOKResponse creates a standard OK response, merging any extra fields.
func buildOKResponse(extra ...map[string]interface{}) map[string]interface{} {
	resp := map[string]interface{}{"ok": true}
	for _, m := range extra {
		for k, v := range m {
			resp[k] = v
		}
	}
	return resp
}
