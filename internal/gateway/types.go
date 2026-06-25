package gateway

import (
	"encoding/json"
	"time"
)

// --- Wire Protocol Types ---

// ProtocolVersion is the current gateway protocol version (matches OpenClaw v4).
const ProtocolVersion = 4

// ClientID represents a gateway client identifier.
type ClientID string

const (
	ClientIDWebchatUI     ClientID = "webchat-ui"
	ClientIDControlUI     ClientID = "openclaw-control-ui"
	ClientIDTUI           ClientID = "openclaw-tui"
	ClientIDWebchat       ClientID = "webchat"
	ClientIDCLI           ClientID = "cli"
	ClientIDGatewayClient ClientID = "gateway-client"
	ClientIDMacOS         ClientID = "openclaw-macos"
	ClientIDIOS           ClientID = "openclaw-ios"
	ClientIDAndroid       ClientID = "openclaw-android"
	ClientIDNodeHost      ClientID = "node-host"
	ClientIDTest          ClientID = "test"
)

// ClientMode represents a gateway client mode.
type ClientMode string

const (
	ClientModeWebchat ClientMode = "webchat"
	ClientModeCLI     ClientMode = "cli"
	ClientModeUI      ClientMode = "ui"
	ClientModeBackend ClientMode = "backend"
	ClientModeNode    ClientMode = "node"
	ClientModeTest    ClientMode = "test"
)

// Scope represents an operator permission scope.
type Scope string

const (
	ScopeRead      Scope = "operator.read"
	ScopeWrite     Scope = "operator.write"
	ScopeAdmin     Scope = "operator.admin"
	ScopePairing   Scope = "operator.pairing"
	ScopeApprovals Scope = "operator.approvals"
)

// --- Frame Types ---

// RequestFrame is a client-to-server RPC call.
type RequestFrame struct {
	Type   string      `json:"type"`             // "req"
	ID     string      `json:"id"`               // correlation ID
	Method string      `json:"method"`           // e.g. "chat.send", "health"
	Params interface{} `json:"params,omitempty"` // method-specific
}

// ResponseFrame is a server-to-client RPC response.
type ResponseFrame struct {
	Type    string      `json:"type"`              // "res"
	ID      string      `json:"id"`                // matches request
	OK      bool        `json:"ok"`                // success indicator
	Payload interface{} `json:"payload,omitempty"` // result on success
	Error   *ErrorShape `json:"error,omitempty"`   // error on failure
}

// EventFrame is a server-to-client push notification.
type EventFrame struct {
	Type         string        `json:"type"`                   // "event"
	Event        string        `json:"event"`                  // event name
	Payload      interface{}   `json:"payload,omitempty"`      // event data
	Seq          *int          `json:"seq,omitempty"`          // monotonic sequence number
	StateVersion *StateVersion `json:"stateVersion,omitempty"` // version counters
}

// ErrorShape represents an RPC error.
type ErrorShape struct {
	Code         string      `json:"code"`
	Message      string      `json:"message"`
	Details      interface{} `json:"details,omitempty"`
	Retryable    *bool       `json:"retryable,omitempty"`
	RetryAfterMs *int        `json:"retryAfterMs,omitempty"`
}

// StateVersion carries presence and health version counters.
type StateVersion struct {
	Presence int `json:"presence"`
	Health   int `json:"health"`
}

// --- Handshake Types ---

// ClientInfo is sent by the client during the connect handshake.
type ClientInfo struct {
	ID              ClientID   `json:"id"`
	DisplayName     string     `json:"displayName,omitempty"`
	Version         string     `json:"version"`
	Platform        string     `json:"platform"`
	DeviceFamily    string     `json:"deviceFamily,omitempty"`
	ModelIdentifier string     `json:"modelIdentifier,omitempty"`
	Mode            ClientMode `json:"mode"`
	InstanceID      string     `json:"instanceId,omitempty"`
}

// DeviceIdentity is an optional Ed25519 device proof.
type DeviceIdentity struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
	SignedAt  int64  `json:"signedAt"`
	Nonce     string `json:"nonce"`
}

// AuthParams carries authentication credentials.
type AuthParams struct {
	Token                string `json:"token,omitempty"`
	BootstrapToken       string `json:"bootstrapToken,omitempty"`
	DeviceToken          string `json:"deviceToken,omitempty"`
	Password             string `json:"password,omitempty"`
	ApprovalRuntimeToken string `json:"approvalRuntimeToken,omitempty"`
}

// ConnectParams is the first message after WebSocket connect.
type ConnectParams struct {
	MinProtocol int                    `json:"minProtocol"`
	MaxProtocol int                    `json:"maxProtocol"`
	Client      ClientInfo             `json:"client"`
	Scopes      []string               `json:"scopes,omitempty"`
	Role        string                 `json:"role,omitempty"`
	PathEnv     string                 `json:"pathEnv,omitempty"`
	Device      *DeviceIdentity        `json:"device,omitempty"`
	Auth        *AuthParams            `json:"auth,omitempty"`
	Caps        []string               `json:"caps,omitempty"`
	Permissions map[string]interface{} `json:"permissions,omitempty"`
	Locale      string                 `json:"locale,omitempty"`
	UserAgent   string                 `json:"userAgent,omitempty"`
}

// HelloOk is the successful handshake response.
type HelloOk struct {
	Type              string            `json:"type"` // "hello-ok"
	Protocol          int               `json:"protocol"`
	Server            ServerInfo        `json:"server"`
	Features          FeatureList       `json:"features"`
	Snapshot          *Snapshot         `json:"snapshot"`
	PluginSurfaceURLs map[string]string `json:"pluginSurfaceUrls,omitempty"`
	Auth              HelloAuth         `json:"auth"`
	Policy            GatewayPolicy     `json:"policy"`
}

// ServerInfo identifies the server in the hello response.
type ServerInfo struct {
	Version string `json:"version"`
	ConnID  string `json:"connId"`
}

// FeatureList advertises available methods and events.
type FeatureList struct {
	Methods []string `json:"methods"`
	Events  []string `json:"events"`
}

// HelloAuth carries authentication tokens and scopes.
type HelloAuth struct {
	DeviceToken  string        `json:"deviceToken,omitempty"`
	Role         string        `json:"role"`
	Scopes       []string      `json:"scopes"`
	IssuedAtMs   int64         `json:"issuedAtMs,omitempty"`
	DeviceTokens []DeviceToken `json:"deviceTokens,omitempty"`
}

// DeviceToken is a JWT-style bootstrap token.
type DeviceToken struct {
	DeviceToken string   `json:"deviceToken"`
	Role        string   `json:"role"`
	Scopes      []string `json:"scopes"`
	IssuedAtMs  int64    `json:"issuedAtMs"`
}

// GatewayPolicy configures connection limits.
type GatewayPolicy struct {
	MaxPayload       int `json:"maxPayload"`       // 25MB
	MaxBufferedBytes int `json:"maxBufferedBytes"` // 50MB
	TickIntervalMs   int `json:"tickIntervalMs"`   // 30s
}

// --- Snapshot Types ---

// Snapshot is the initial state sent in hello-ok.
type Snapshot struct {
	Presence        []PresenceEntry `json:"presence"`
	Health          interface{}     `json:"health"`
	StateVersion    StateVersion    `json:"stateVersion"`
	UptimeMs        int64           `json:"uptimeMs"`
	ConfigPath      string          `json:"configPath,omitempty"`
	StateDir        string          `json:"stateDir,omitempty"`
	SessionDefaults SessionDefaults `json:"sessionDefaults"`
	AuthMode        string          `json:"authMode,omitempty"`
	UpdateAvailable *UpdateInfo     `json:"updateAvailable,omitempty"`
}

// PresenceEntry represents a connected client or presence entry.
type PresenceEntry struct {
	Host             string   `json:"host,omitempty"`
	IP               string   `json:"ip,omitempty"`
	Version          string   `json:"version,omitempty"`
	Platform         string   `json:"platform,omitempty"`
	DeviceFamily     string   `json:"deviceFamily,omitempty"`
	ModelIdentifier  string   `json:"modelIdentifier,omitempty"`
	Mode             string   `json:"mode,omitempty"`
	LastInputSeconds int      `json:"lastInputSeconds,omitempty"`
	Reason           string   `json:"reason,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Text             string   `json:"text,omitempty"`
	TS               int64    `json:"ts"`
	DeviceID         string   `json:"deviceId,omitempty"`
	Roles            []string `json:"roles,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	InstanceID       string   `json:"instanceId,omitempty"`
}

// SessionDefaults carries the default session configuration.
type SessionDefaults struct {
	DefaultAgentID string `json:"defaultAgentId"`
	MainKey        string `json:"mainKey"`
	MainSessionKey string `json:"mainSessionKey"`
	Scope          string `json:"scope,omitempty"`
}

// UpdateInfo represents an available software update.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Channel        string `json:"channel"`
}

// --- Chat Event Types ---

// ChatState represents the state of a chat stream event.
type ChatState string

const (
	ChatStateDelta   ChatState = "delta"
	ChatStateFinal   ChatState = "final"
	ChatStateAborted ChatState = "aborted"
	ChatStateError   ChatState = "error"
)

// ChatEvent is the base type pushed as "chat" events.
type ChatEvent struct {
	RunID      string      `json:"runId"`
	SessionKey string      `json:"sessionKey"`
	AgentID    string      `json:"agentId,omitempty"`
	SpawnedBy  string      `json:"spawnedBy,omitempty"`
	Seq        int         `json:"seq"`
	State      ChatState   `json:"state"`
	Message    interface{} `json:"message,omitempty"`
	DeltaText  string      `json:"deltaText,omitempty"`
	Replace    *bool       `json:"replace,omitempty"`
	Usage      interface{} `json:"usage,omitempty"`
	StopReason string      `json:"stopReason,omitempty"`
	ErrorMsg   string      `json:"errorMessage,omitempty"`
	ErrorKind  string      `json:"errorKind,omitempty"`
}

// ChatMessage represents an assistant message in the chat.
type ChatMessage struct {
	Role      string         `json:"role"`
	Content   []ContentBlock `json:"content"`
	Timestamp int64          `json:"timestamp,omitempty"`
}

// ContentBlock is a content part within a chat message.
// Supports text, tool_use, and tool_result content types.
type ContentBlock struct {
	Type  string          `json:"type"`            // "text", "tool_use", "tool_result"
	Text  string          `json:"text,omitempty"`  // text content or tool result text
	ID    string          `json:"id,omitempty"`    // tool call ID (for tool_use / tool_result)
	Name  string          `json:"name,omitempty"`  // tool name (for tool_use / tool_result)
	Input json.RawMessage `json:"input,omitempty"` // tool arguments (for tool_use), raw JSON object
}

// --- Agent Event Types ---

// ToolStreamData is the data payload for agent tool events.
type ToolStreamData struct {
	ToolCallID    string      `json:"toolCallId"`
	Name          string      `json:"name"`
	Phase         string      `json:"phase"`                   // "start", "update", "result"
	Args          interface{} `json:"args,omitempty"`          // tool arguments (for start)
	Result        string      `json:"result,omitempty"`        // tool result text (for result)
	PartialResult string      `json:"partialResult,omitempty"` // partial result (for update)
	IsError       bool        `json:"isError,omitempty"`       // error flag
}

// AgentToolPayload is the full payload for "agent" events with stream "tool".
type AgentToolPayload struct {
	RunID      string         `json:"runId"`
	Seq        int            `json:"seq"`
	Stream     string         `json:"stream"` // "tool"
	TS         int64          `json:"ts"`
	SessionKey string         `json:"sessionKey,omitempty"`
	AgentID    string         `json:"agentId,omitempty"`
	Data       ToolStreamData `json:"data"`
}

// --- Tick / Shutdown ---

// TickPayload is the periodic heartbeat payload.
type TickPayload struct {
	TS int64 `json:"ts"`
}

// ShutdownPayload is sent when the server is shutting down.
type ShutdownPayload struct {
	Reason            string `json:"reason"`
	RestartExpectedMs int    `json:"restartExpectedMs,omitempty"`
}

// --- Error Codes ---

const (
	ErrCodeInvalidRequest   = "INVALID_REQUEST"
	ErrCodeNotLinked        = "NOT_LINKED"
	ErrCodeNotPaired        = "NOT_PAIRED"
	ErrCodeAgentTimeout     = "AGENT_TIMEOUT"
	ErrCodeApprovalNotFound = "APPROVAL_NOT_FOUND"
	ErrCodeUnavailable      = "UNAVAILABLE"
)

// --- Time Constants ---

const (
	MaxPayloadBytes  = 25 * 1024 * 1024 // 25 MB
	MaxBufferedBytes = 50 * 1024 * 1024 // 50 MB
	TickIntervalMs   = 30000            // 30 seconds
	HealthRefreshMs  = 60000            // 60 seconds
	DedupeTTL        = 5 * time.Minute
	MaxPreAuthBytes  = 64 * 1024
	HandshakeTimeout = 30 * time.Second
)
