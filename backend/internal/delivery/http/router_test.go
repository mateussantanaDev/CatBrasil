package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	deliveryhttp "catbrasil/backend/internal/delivery/http"
	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/internal/usecase/auth"
	"catbrasil/backend/internal/usecase/garage"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
	"catbrasil/backend/internal/usecase/shop"
	"catbrasil/backend/pkg/jwt"
)

func TestRouter_HealthCheck(t *testing.T) {
	tm := jwt.NewTokenManager("test_secret_key_1234567890_enterprise", 24)
	hub := ws.NewHub()
	go hub.Run()

	authUC := auth.NewAuthUseCase(nil, nil, nil, nil, nil, tm)
	garageUC := garage.NewGarageUseCase(nil, nil, nil, nil, nil, nil, nil, nil)
	shopUC := shop.NewShopUseCase(nil, nil, nil, nil)
	paymentUC := payment.NewPaymentUseCase(nil, nil, nil, nil, nil, "secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})
	rouletteUC := roulette.NewRouletteUseCase(nil, nil, nil, nil, nil)
	rankingUC := ranking.NewRankingUseCase(nil)
	adminUC := admin.NewAdminUseCase(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	var readinessErr error

	router := deliveryhttp.NewRouter(deliveryhttp.RouterConfig{
		TokenManager:   tm,
		AuthHandler:    handler.NewAuthHandler(authUC),
		UserHandler:    handler.NewUserHandler(authUC, garageUC, nil, nil),
		GarageHandler:  handler.NewGarageHandler(garageUC, hub),
		ShopHandler:    handler.NewShopHandler(shopUC, hub),
		FinHandler:     handler.NewFinancialHandler(paymentUC, hub),
		RefHandler:     handler.NewReferralHandler(nil, nil),
		RoulHandler:    handler.NewRouletteHandler(rouletteUC, hub),
		RankHandler:    handler.NewRankingHandler(rankingUC),
		AnnHandler:     handler.NewAnnouncementHandler(nil),
		AdminHandler:   handler.NewAdminHandler(adminUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, hub),
		WSHub:          hub,
		ReadinessCheck: func(context.Context) error { return readinessErr },
	})

	t.Run("Health Endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}

		if resp["status"] != "online" || resp["service"] != "catbrasil-api-golang" {
			t.Errorf("unexpected health response: %+v", resp)
		}
	})
	t.Run("Readiness Tracks Dependencies", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/ready", nil)
		ready := httptest.NewRecorder()
		router.ServeHTTP(ready, request)
		if ready.Code != http.StatusOK {
			t.Fatalf("ready com dependências saudáveis: %d", ready.Code)
		}
		readinessErr = errors.New("database unavailable")
		unready := httptest.NewRecorder()
		router.ServeHTTP(unready, request)
		if unready.Code != http.StatusServiceUnavailable {
			t.Fatalf("ready deve responder 503 sem banco: %d", unready.Code)
		}
		readinessErr = nil
	})

	t.Run("Protected Route Without Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/user", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
		}
	})
	t.Run("Dev Simulation Routes Disabled By Default", func(t *testing.T) {
		for _, route := range []string{"/api/v1/dev/simulate-webhook", "/api/v1/garage/simulate", "/api/v1/referrals/simulate", "/api/v1/roulette/reset", "/api/v1/auth/register", "/api/v1/auth/send-sms"} {
			req := httptest.NewRequest(http.MethodPost, route, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code != http.StatusNotFound {
				t.Errorf("%s: esperado 404, recebido %d", route, rr.Code)
			}
		}
	})

	t.Run("Protected Admin Route Without Token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
		}
	})

	t.Run("Protected Admin Route With User Token (Forbidden)", func(t *testing.T) {
		regularUser := &domain.User{
			ID:       "CB-458F",
			Username: "mateus",
			Role:     domain.RoleUser,
		}
		userToken, _ := tm.GenerateToken(regularUser)

		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rr.Code)
		}
	})

	t.Run("Protected Admin Route Without User Repository (Unavailable)", func(t *testing.T) {
		manager := &domain.User{ID: "manager-no-repository", Username: "manager", Role: domain.RoleGestor}
		adminToken, _ := tm.GenerateToken(manager)
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 without user repository, got %d", rr.Code)
		}
	})
}
