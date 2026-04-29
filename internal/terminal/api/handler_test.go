package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewTerminalHandler(nil, zap.NewNop())
	handler.RegisterRouters(router)

	routes := router.Routes()
	want := map[string]bool{
		"GET /api/system/terminal/targets":             false,
		"GET /api/system/terminal/connect":             false,
		"GET /api/system/terminal/sessions":            false,
		"POST /api/system/terminal/sessions/:id/close": false,
	}
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("route %s not registered; routes=%+v", key, routes)
		}
	}
}

func TestTargetsFailsWithoutService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewTerminalHandler(nil, zap.NewNop()).RegisterRouters(router)
	req := httptest.NewRequest(http.MethodGet, "/api/system/terminal/targets", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	var body base.ApiResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != base.StatusError {
		t.Fatalf("code = %d, want error", body.Code)
	}
}
