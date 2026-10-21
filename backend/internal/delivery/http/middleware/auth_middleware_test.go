package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/jwt"
)

func TestAuthMiddleware(t *testing.T) {
	tm := jwt.NewTokenManager("test_secret_key_1234567890_enterprise", 24)

	user := &domain.User{
		ID:       "CB-458F",
		Username: "mateus",
		Role:     domain.RoleUser,
	}
	token, err := tm.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.GetUserClaims(r.Context())
		if claims == nil || claims.UserID != "CB-458F" {
			t.Errorf("expected claims UserID to be CB-458F, got %v", claims)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	mw := middleware.AuthMiddleware(tm)(testHandler)

	t.Run("Valid Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()

		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("Missing Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/user", nil)
		rr := httptest.NewRecorder()

		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
		}
	})

	t.Run("Invalid Token Format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/user", nil)
		req.Header.Set("Authorization", "Bearer invalid_token_123")
		rr := httptest.NewRecorder()

		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
		}
	})
}

func TestRequireAdmin(t *testing.T) {
	tm := jwt.NewTokenManager("test_secret_key_1234567890_enterprise", 24)

	adminUser := &domain.User{
		ID:       "gst-001",
		Username: "admin",
		Role:     domain.RoleDiretoria,
	}
	adminToken, _ := tm.GenerateToken(adminUser)

	regularUser := &domain.User{
		ID:       "CB-458F",
		Username: "mateus",
		Role:     domain.RoleUser,
	}
	regularToken, _ := tm.GenerateToken(regularUser)

	targetHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	protectedHandler := middleware.AuthMiddleware(tm)(middleware.RequireAdmin()(targetHandler))

	t.Run("Admin User Allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin, got %d", rr.Code)
		}
	})

	t.Run("Regular User Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+regularToken)
		rr := httptest.NewRecorder()

		protectedHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for regular user, got %d", rr.Code)
		}
	})
}
