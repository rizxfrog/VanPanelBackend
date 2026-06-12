package main

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
)

//go:embed webui/dist/*
var webuiEmbedFS embed.FS

var webuiSubFS fs.FS

// gatewayServerInstance is the running gateway server, set during startup.
var gatewayServerInstance *gateway.GatewayServer

// handleGatewayUpgrade upgrades an HTTP request to a WebSocket connection
// and hands it to the gateway server for RPC handling.
func handleGatewayUpgrade(c *gin.Context) {
	if gatewayServerInstance != nil {
		gatewayServerInstance.ServeWS(c.Writer, c.Request)
	}
}

func init() {
	sub, err := fs.Sub(webuiEmbedFS, "webui/dist")
	if err != nil {
		log.Printf("webui dist not found, running without embedded UI: %v", err)
		return
	}
	webuiSubFS = sub
}

func registerWebUI(router *gin.Engine, gwServer *gateway.GatewayServer) {
	if webuiSubFS == nil {
		return
	}

	// Store gateway server reference for WebSocket upgrade handler
	gatewayServerInstance = gwServer

	// Serve static assets with proper MIME types
	router.GET("/assets/*filepath", func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		f, err := webuiSubFS.Open(path)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, contentTypeByExt(filepath.Ext(path)), data)
	})

	// Serve root-level static files
	router.GET("/sw.js", func(c *gin.Context) { serveFile(c, "sw.js", "application/javascript; charset=utf-8") })
	router.GET("/control-ui-config.json", func(c *gin.Context) { serveConfigJSON(c) })
	router.GET("/manifest.webmanifest", func(c *gin.Context) { serveFile(c, "manifest.webmanifest", "application/manifest+json") })
	router.GET("/favicon.svg", func(c *gin.Context) {
		serveFile(c, "favicon.svg", "image/svg+xml")
	})

	// Dedicated WebSocket endpoint for gateway
	router.GET("/ws", func(c *gin.Context) {
		handleGatewayUpgrade(c)
	})

	// Root → index.html or WebSocket upgrade
	router.GET("/", func(c *gin.Context) {
		if c.IsWebsocket() {
			handleGatewayUpgrade(c)
			return
		}
		serveFile(c, "index.html", "text/html; charset=utf-8")
	})

	// SPA fallback for all other paths (also catches WS upgrades for other paths)
	router.NoRoute(func(c *gin.Context) {
		if c.IsWebsocket() {
			handleGatewayUpgrade(c)
			return
		}
		// Don't intercept API calls
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		serveFile(c, "index.html", "text/html; charset=utf-8")
	})

	log.Printf("OpenClaw web UI registered at /")
}

func serveFile(c *gin.Context, name, contentType string) {
	f, err := webuiSubFS.Open(name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, contentType, data)
}

func webuiFSReady(sub fs.FS) bool {
	_, err := sub.Open("index.html")
	return err == nil
}

// serveConfigJSON serves the control-ui-config.json bootstrap config.
func serveConfigJSON(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"basePath":         "/",
		"assistantName":    "VanPanel",
		"assistantAvatar":  "",
		"assistantAgentId": "main",
		"serverVersion":    "vanpanel-0.1.0",
	})
}

func contentTypeByExt(ext string) string {
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	case ".webmanifest":
		return "application/manifest+json"
	default:
		return "application/octet-stream"
	}
}
