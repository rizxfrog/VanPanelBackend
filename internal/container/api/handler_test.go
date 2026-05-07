package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	"github.com/gin-gonic/gin"
)

type fakeContainerService struct {
	operation string
}

func (f *fakeContainerService) List(context.Context, containermodel.ListQuery) (containermodel.ListResult, error) {
	return containermodel.ListResult{Items: []containermodel.Container{{ID: "abc", Name: "web"}}, Total: 1}, nil
}

func (f *fakeContainerService) Operate(_ context.Context, id string, operation string) error {
	f.operation = operation + ":" + id
	return nil
}

func (f *fakeContainerService) Stats(context.Context, string) (containermodel.Stats, error) {
	return containermodel.Stats{CPUPercent: 1.5}, nil
}

func (f *fakeContainerService) Logs(context.Context, string, containermodel.LogOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("ok\n")), nil
}

func TestRegisterRouters(t *testing.T) {
	router := gin.New()
	NewContainerHandler(&fakeContainerService{}).RegisterRouters(router)
	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		"GET /api/system/containers",
		"POST /api/system/containers/:id/start",
		"POST /api/system/containers/:id/stop",
		"POST /api/system/containers/:id/restart",
		"DELETE /api/system/containers/:id",
		"GET /api/system/containers/:id/stats",
		"GET /api/system/containers/:id/logs",
	} {
		if !routes[expected] {
			t.Fatalf("missing route %s", expected)
		}
	}
}

func TestListContainers(t *testing.T) {
	router := gin.New()
	NewContainerHandler(&fakeContainerService{}).RegisterRouters(router)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/containers?state=running", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), `"name":"web"`) {
		t.Fatalf("unexpected body: %s", resp.Body.String())
	}
}

func TestStartContainer(t *testing.T) {
	service := &fakeContainerService{}
	router := gin.New()
	NewContainerHandler(service).RegisterRouters(router)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/containers/abc/start", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if service.operation != "start:abc" {
		t.Fatalf("unexpected operation: %s", service.operation)
	}
}

func TestPlainLogs(t *testing.T) {
	router := gin.New()
	NewContainerHandler(&fakeContainerService{}).RegisterRouters(router)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/containers/abc/logs", nil)
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if resp.Body.String() != "ok\n" {
		t.Fatalf("unexpected logs: %q", resp.Body.String())
	}
}
