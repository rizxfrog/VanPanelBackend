package gateway

import (
	"fmt"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/rizxfrog/VanPanelBackend/pkg/jwt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// AuthHandler validates gateway connection credentials from ConnectParams.
type AuthHandler struct {
	logger         *zap.Logger
	jwtHandler     jwt.Handler
	authMode       string
	sharedToken    string
	sharedPassword string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(logger *zap.Logger, jwtHdl jwt.Handler, authMode, token, password string) *AuthHandler {
	return &AuthHandler{
		logger:         logger,
		jwtHandler:     jwtHdl,
		authMode:       authMode,
		sharedToken:    token,
		sharedPassword: password,
	}
}

// deviceTokenClaims holds the custom claims for a gateway device JWT.
type deviceTokenClaims struct {
	jwtlib.RegisteredClaims
	Role   string   `json:"role"`
	Scopes []string `json:"scopes"`
}

// defaultAuthScopes returns the full operator scope set.
func defaultAuthScopes() []string {
	return []string{
		string(ScopeRead),
		string(ScopeWrite),
		string(ScopeAdmin),
	}
}

// Validate authenticates a gateway connection based on the configured auth mode.
//
// Supported modes:
//   - "none":      no authentication required, returns operator role + scopes
//   - "token":     shared token comparison; also accepts JWT device tokens
//   - "password":  simple shared password comparison
func (a *AuthHandler) Validate(params *ConnectParams) (role string, scopes []string, err error) {
	if params == nil {
		return "", nil, fmt.Errorf("connect params are nil")
	}

	switch a.authMode {
	case "none":
		return "operator", defaultAuthScopes(), nil

	case "token":
		if params.Auth == nil {
			return "", nil, fmt.Errorf("authentication required")
		}
		// Shared token takes precedence over device token
		if params.Auth.Token != "" {
			if params.Auth.Token != a.sharedToken {
				return "", nil, fmt.Errorf("invalid token")
			}
			return "operator", defaultAuthScopes(), nil
		}
		if params.Auth.DeviceToken != "" {
			return a.parseDeviceToken(params.Auth.DeviceToken)
		}
		return "", nil, fmt.Errorf("authentication required")

	case "password":
		if params.Auth == nil || params.Auth.Password == "" {
			return "", nil, fmt.Errorf("authentication required")
		}
		if params.Auth.Password != a.sharedPassword {
			return "", nil, fmt.Errorf("invalid password")
		}
		return "operator", defaultAuthScopes(), nil

	default:
		return "", nil, fmt.Errorf("unknown auth mode: %s", a.authMode)
	}
}

// parseDeviceToken parses a JWT device token and extracts role/scopes.
func (a *AuthHandler) parseDeviceToken(tokenStr string) (string, []string, error) {
	key := viper.GetString("jwt.key1")
	if key == "" {
		return "", nil, fmt.Errorf("jwt signing key not configured")
	}

	claims := &deviceTokenClaims{}
	token, err := jwtlib.ParseWithClaims(tokenStr, claims, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(key), nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("device token parse error: %w", err)
	}
	if !token.Valid {
		return "", nil, fmt.Errorf("device token is invalid")
	}
	if claims.Role == "" {
		return "", nil, fmt.Errorf("device token missing role claim")
	}
	return claims.Role, claims.Scopes, nil
}

// HasScope checks whether a scope list contains the required scope (or admin).
func HasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required || s == string(ScopeAdmin) {
			return true
		}
	}
	return false
}

// GetAllScopes returns every known gateway scope.
func GetAllScopes() []string {
	return []string{
		string(ScopeRead),
		string(ScopeWrite),
		string(ScopeAdmin),
		string(ScopePairing),
		string(ScopeApprovals),
	}
}
