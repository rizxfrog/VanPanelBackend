package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	ijwt "github.com/GoSimplicity/AI-CloudOps/pkg/jwt"
	"github.com/gin-gonic/gin"
)

func TestCheckAuthSkipsPublicShareRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	auth := NewAuthMiddleware(nil)
	router.Use(auth.CheckAuth())
	router.GET("/api/share/:code/download", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/share/abc/download", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("public share route should bypass auth, got status %d", resp.Code)
	}
}

func TestCheckLoginSkipsPublicShareRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	jwtMiddleware := NewJWTMiddleware(ijwt.NewJWTHandler(nil))
	router.Use(jwtMiddleware.CheckLogin())
	router.GET("/api/share/:code/download", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/share/abc/download", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("public share route should bypass login check, got status %d", resp.Code)
	}
}
