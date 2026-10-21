package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/jwt"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
	UserIDKey     contextKey = "user_id"
)

// ResponseHelper padroniza as respostas JSON da API REST
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondError padroniza respostas de erro JSON
func RespondError(w http.ResponseWriter, statusCode int, message string, errCode ...string) {
	resp := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	if len(errCode) > 0 && errCode[0] != "" {
		resp["errorCode"] = errCode[0]
	}
	RespondJSON(w, statusCode, resp)
}

// GetUserClaims extrai os claims JWT do contexto da requisição
func GetUserClaims(ctx context.Context) *jwt.CustomClaims {
	claims, ok := ctx.Value(UserClaimsKey).(*jwt.CustomClaims)
	if !ok {
		return nil
	}
	return claims
}

// GetUserID extrai o UserID do contexto da requisição
func GetUserID(ctx context.Context) string {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}

// AuthMiddleware valida o token JWT no cabeçalho Authorization: Bearer <token>
func AuthMiddleware(tokenManager *jwt.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			tokenStr := ""

			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenStr = parts[1]
				}
			}

			if tokenStr == "" {
				RespondError(w, http.StatusUnauthorized, "Token de autenticação ausente", "AUTH_TOKEN_MISSING")
				return
			}

			claims, err := tokenManager.ValidateToken(tokenStr)
			if err != nil {
				RespondError(w, http.StatusUnauthorized, err.Error(), "AUTH_TOKEN_INVALID")
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdmin garante que apenas administradores ou diretores acessem a rota
func RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserClaims(r.Context())
			if claims == nil || !claims.IsAdmin {
				RespondError(w, http.StatusForbidden, "Acesso restrito ao painel de gestão da diretoria", "ERP_AUTH_REQUIRED")
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}

// RequireAdminActive checks the current database role so a demoted manager cannot
// keep using an otherwise valid JWT for the rest of its lifetime.
func RequireAdminActive(users domain.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserClaims(r.Context())
			if claims == nil || !claims.IsAdmin {
				RespondError(w, http.StatusForbidden, "Acesso restrito ao painel de gestão da diretoria", "ERP_AUTH_REQUIRED")
				return
			}
			if users == nil {
				RespondError(w, http.StatusServiceUnavailable, "Sessão administrativa indisponível", "ERP_SESSION_UNAVAILABLE")
				return
			}
			user, err := users.GetByID(r.Context(), claims.UserID)
			if err != nil {
				RespondError(w, http.StatusServiceUnavailable, "Não foi possível validar a sessão administrativa", "ERP_SESSION_UNAVAILABLE")
				return
			}
			if user == nil || !user.IsAdmin() || user.Role != claims.Role || !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
				RespondError(w, http.StatusForbidden, "Sessão administrativa sem permissão atual", "ERP_AUTH_REQUIRED")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole garante que o usuário possua um dos papéis especificados
func RequireRole(roles ...domain.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserClaims(r.Context())
			if claims == nil {
				RespondError(w, http.StatusUnauthorized, "Não autenticado", "UNAUTHORIZED")
				return
			}

			allowed := false
			for _, role := range roles {
				if claims.Role == role {
					allowed = true
					break
				}
			}

			if !allowed && !claims.IsAdmin {
				RespondError(w, http.StatusForbidden, "Permissão insuficiente para esta operação", "FORBIDDEN")
				return
			}

			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}
