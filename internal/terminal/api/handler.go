package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	terminalmodel "github.com/rizxfrog/VanPanelBackend/internal/terminal/model"
	terminalservice "github.com/rizxfrog/VanPanelBackend/internal/terminal/service"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
	"github.com/rizxfrog/VanPanelBackend/pkg/jwt"
	"go.uber.org/zap"
)

type TerminalHandler struct {
	service terminalservice.TerminalService
	logger  *zap.Logger
}

func NewTerminalHandler(service terminalservice.TerminalService, logger *zap.Logger) *TerminalHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TerminalHandler{service: service, logger: logger}
}

func (h *TerminalHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/system/terminal")
	group.GET("/targets", h.ListTargets)
	group.GET("/connect", h.Connect)
	group.GET("/sessions", h.ListSessions)
	group.POST("/sessions/:id/close", h.CloseSession)
}

func (h *TerminalHandler) ListTargets(ctx *gin.Context) {
	if h.service == nil {
		base.ErrorWithMessage(ctx, "terminal service unavailable")
		return
	}
	targets, err := h.service.ListTargets(ctx)
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, targets)
}

func (h *TerminalHandler) ListSessions(ctx *gin.Context) {
	if h.service == nil {
		base.ErrorWithMessage(ctx, "terminal service unavailable")
		return
	}
	user := ctx.MustGet("user").(jwt.UserClaims)
	base.SuccessWithData(ctx, h.service.ListSessions(user.Uid))
}

func (h *TerminalHandler) CloseSession(ctx *gin.Context) {
	if h.service == nil {
		base.ErrorWithMessage(ctx, "terminal service unavailable")
		return
	}
	if err := h.service.Close(ctx.Param("id"), "api_closed"); err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithMessage(ctx, "closed")
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (h *TerminalHandler) Connect(ctx *gin.Context) {
	if h.service == nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "terminal service unavailable"})
		return
	}
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(msg terminalmodel.Message) {
		writeMu.Lock()
		defer writeMu.Unlock()
		if err := conn.WriteJSON(msg); err != nil {
			h.logger.Debug("write terminal websocket message failed", zap.Error(err))
		}
	}

	var first terminalmodel.Message
	if err := conn.ReadJSON(&first); err != nil {
		writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeError, Message: "invalid connect message"})
		return
	}
	if err := first.ValidateFirst(); err != nil {
		writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeError, Message: err.Error()})
		return
	}

	user := ctx.MustGet("user").(jwt.UserClaims)
	session, err := h.service.Start(ctx, terminalservice.StartRequest{
		UserID:     user.Uid,
		Username:   user.Username,
		SourceIP:   ctx.ClientIP(),
		TargetType: first.TargetType,
		TargetID:   first.TargetID,
		Cols:       first.Cols,
		Rows:       first.Rows,
	})
	if err != nil {
		writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeError, Message: err.Error()})
		return
	}
	defer h.service.Close(session.ID, "websocket_closed")

	writeJSON(terminalmodel.Message{
		Type:       terminalmodel.MessageTypeConnected,
		SessionID:  session.ID,
		TargetType: session.TargetType,
		TargetName: session.TargetName,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := session.Stream.Read(buf)
			if err != nil {
				writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeClosed, SessionID: session.ID, Reason: err.Error()})
				return
			}
			writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeOutput, SessionID: session.ID, Data: string(buf[:n])})
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
		}
		var msg terminalmodel.Message
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if err := msg.Validate(); err != nil {
			writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypeError, SessionID: session.ID, Message: err.Error()})
			continue
		}
		switch msg.Type {
		case terminalmodel.MessageTypeInput:
			_, _ = session.Stream.Write([]byte(msg.Data))
		case terminalmodel.MessageTypeResize:
			_ = session.Stream.Resize(msg.Cols, msg.Rows)
		case terminalmodel.MessageTypePing:
			writeJSON(terminalmodel.Message{Type: terminalmodel.MessageTypePong, SessionID: session.ID})
		case terminalmodel.MessageTypeClose:
			return
		}
	}
}
