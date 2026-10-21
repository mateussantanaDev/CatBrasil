package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	deliveryhttp "catbrasil/backend/internal/delivery/http"
	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/http/middleware"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/internal/usecase/auth"
	"catbrasil/backend/internal/usecase/garage"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/internal/usecase/ranking"
	"catbrasil/backend/internal/usecase/roulette"
	"catbrasil/backend/internal/usecase/shop"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/jwt"
)

type mockUserRepoAPI struct {
	user  *domain.User
	admin *domain.User
}

func (m *mockUserRepoAPI) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.admin != nil && id == m.admin.ID {
		return m.admin, nil
	}
	return m.user, nil
}
func (m *mockUserRepoAPI) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.user, nil
}
func (m *mockUserRepoAPI) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.admin != nil && username == m.admin.Username {
		return m.admin, nil
	}
	return m.user, nil
}
func (m *mockUserRepoAPI) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoAPI) GetByFingerprintOrIP(ctx context.Context, fp, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoAPI) Create(ctx context.Context, u *domain.User) error {
	m.user = u
	return nil
}
func (m *mockUserRepoAPI) Update(ctx context.Context, u *domain.User) error {
	m.user = u
	return nil
}
func (m *mockUserRepoAPI) AdjustBalance(ctx context.Context, id string, db, dy float64) (*domain.User, error) {
	m.user.Balance += db
	m.user.YieldBalanceAvailable += dy
	return m.user, nil
}
func (m *mockUserRepoAPI) List(ctx context.Context, q, s string, l, o int) ([]*domain.User, int, error) {
	return []*domain.User{m.user}, 1, nil
}
func (m *mockUserRepoAPI) Count(ctx context.Context) (int64, error)  { return 1, nil }
func (m *mockUserRepoAPI) ResetDailySpins(ctx context.Context) error { return nil }

func TestAdminRoleRevocationInvalidatesExistingToken(t *testing.T) {
	tokens := jwt.NewTokenManager("test_secret_for_admin_role_revocation_2026", 24)
	manager := &domain.User{ID: "manager-revoked", Username: "manager", Role: domain.RoleGestor, TwoFactorEnabled: true, TwoFactorSecret: "configured-totp-secret"}
	token, err := tokens.GenerateToken(manager)
	if err != nil {
		t.Fatal(err)
	}
	repo := &mockUserRepoAPI{user: manager}
	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	protected := middleware.AuthMiddleware(tokens)(middleware.RequireAdminActive(repo)(target))
	request := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		recorder := httptest.NewRecorder()
		protected.ServeHTTP(recorder, req)
		return recorder.Code
	}
	if code := request(); code != http.StatusOK {
		t.Fatalf("gestor ativo: HTTP %d", code)
	}
	manager.TwoFactorEnabled = false
	if code := request(); code != http.StatusForbidden {
		t.Fatalf("gestor sem 2FA: HTTP %d", code)
	}
	manager.TwoFactorEnabled = true
	manager.Role = domain.RoleUser
	if code := request(); code != http.StatusForbidden {
		t.Fatalf("gestor revogado: HTTP %d", code)
	}
}

func TestAdminRouteWithoutUserRepositoryFailsClosed(t *testing.T) {
	tokens := jwt.NewTokenManager("test_secret_for_missing_admin_repository_2026", 24)
	manager := &domain.User{ID: "manager-no-repo", Username: "manager", Role: domain.RoleGestor, TwoFactorEnabled: true, TwoFactorSecret: "configured-totp-secret"}
	token, err := tokens.GenerateToken(manager)
	if err != nil {
		t.Fatal(err)
	}
	protected := middleware.AuthMiddleware(tokens)(middleware.RequireAdminActive(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	protected.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("repositório ausente deve bloquear ERP, obtido HTTP %d", recorder.Code)
	}
}

type mockCatalogRepoAPI struct{}

func (m *mockCatalogRepoAPI) GetAll(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return []*domain.FleetCatalogItem{
		{ID: "cat_320", Name: "Escavadeira CAT 320", Price: 2500.0, DailyYield: 110.0, Active: true},
	}, nil
}
func (m *mockCatalogRepoAPI) GetActive(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return m.GetAll(ctx)
}
func (m *mockCatalogRepoAPI) GetByID(ctx context.Context, id string) (*domain.FleetCatalogItem, error) {
	return &domain.FleetCatalogItem{ID: "cat_320", Name: "Escavadeira CAT 320", Price: 2500.0, DailyYield: 110.0, Active: true}, nil
}
func (m *mockCatalogRepoAPI) Upsert(ctx context.Context, item *domain.FleetCatalogItem) error {
	return nil
}
func (m *mockCatalogRepoAPI) SaveCatalog(ctx context.Context, items []*domain.FleetCatalogItem) error {
	return nil
}
func (m *mockCatalogRepoAPI) Delete(ctx context.Context, id string) error { return nil }

type mockMachineRepoAPI struct{}

func (m *mockMachineRepoAPI) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepoAPI) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepoAPI) GetByUserID(ctx context.Context, uid string) ([]*domain.Machine, error) {
	return []*domain.Machine{
		{ID: "mach-1", UserID: uid, Name: "Escavadeira CAT 320", YieldAmount: 110.0, State: domain.MachineStateAguardando},
	}, nil
}
func (m *mockMachineRepoAPI) Create(ctx context.Context, mach *domain.Machine) error { return nil }
func (m *mockMachineRepoAPI) Update(ctx context.Context, mach *domain.Machine) error { return nil }
func (m *mockMachineRepoAPI) CollectYield(ctx context.Context, mid, uid string, b float64) (*domain.Machine, float64, error) {
	return nil, 0, nil
}
func (m *mockMachineRepoAPI) ResetDailyMachines(ctx context.Context) error { return nil }
func (m *mockMachineRepoAPI) Delete(ctx context.Context, id string) error  { return nil }

type mockTxRepoAPI struct{}

func (m *mockTxRepoAPI) Create(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepoAPI) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id, UserID: "CB-458F", Tipo: "Depósito PIX", Valor: 300.0, Status: domain.TxStatusPendente}, nil
}
func (m *mockTxRepoAPI) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id, UserID: "CB-458F", Tipo: "Depósito PIX", Valor: 300.0, Status: domain.TxStatusPendente}, nil
}
func (m *mockTxRepoAPI) GetByUserID(ctx context.Context, uid, filter string) ([]*domain.Transaction, error) {
	return []*domain.Transaction{
		{ID: "tx-1", UserID: uid, Tipo: "Depósito PIX", Valor: 300.0, Status: domain.TxStatusSucesso},
	}, nil
}
func (m *mockTxRepoAPI) GetAll(ctx context.Context, status string, l, o int) ([]*domain.Transaction, int, error) {
	return []*domain.Transaction{
		{ID: "tx-1", UserID: "CB-458F", Tipo: "Saque PIX", Valor: -150.0, Status: domain.TxStatusPendente},
	}, 1, nil
}
func (m *mockTxRepoAPI) Update(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepoAPI) ApproveWithdrawal(ctx context.Context, id string) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id, Status: domain.TxStatusSucesso}, nil
}
func (m *mockTxRepoAPI) RejectWithdrawal(ctx context.Context, id, reason string, lock bool) (*domain.Transaction, error) {
	return &domain.Transaction{ID: id, Status: domain.TxStatusRejeitado}, nil
}

type mockRouletteRepoAPI struct{}

func (m *mockRouletteRepoAPI) GetSlices(ctx context.Context) ([]*domain.RouletteSlice, error) {
	return []*domain.RouletteSlice{
		{ID: 1, Label: "Motor C15", Probability: 20.0, Color: "#ffb200", TextColor: "#111111"},
		{ID: 2, Label: "Esteira", Probability: 20.0, Color: "#111111", TextColor: "#ffffff"},
		{ID: 3, Label: "Filtro", Probability: 25.0, Color: "#ffb200", TextColor: "#111111"},
		{ID: 4, Label: "Turbo", Probability: 10.0, Color: "#111111", TextColor: "#ffffff"},
		{ID: 5, Label: "Caixa", Probability: 15.0, Color: "#ffb200", TextColor: "#111111"},
		{ID: 6, Label: "R$ 5,00", Amount: 5.0, Probability: 10.0, Color: "#2e7d32", TextColor: "#ffffff"},
	}, nil
}
func (m *mockRouletteRepoAPI) SaveSlices(ctx context.Context, s []*domain.RouletteSlice) error {
	return nil
}
func (m *mockRouletteRepoAPI) GetSliceByID(ctx context.Context, id int) (*domain.RouletteSlice, error) {
	return nil, nil
}

type mockAnnounceRepoAPI struct{}

func (m *mockAnnounceRepoAPI) GetAll(ctx context.Context) ([]*domain.Announcement, error) {
	return []*domain.Announcement{
		{ID: "evt-1", Title: "Verão CAT", Active: true},
	}, nil
}
func (m *mockAnnounceRepoAPI) GetActive(ctx context.Context) ([]*domain.Announcement, error) {
	return m.GetAll(ctx)
}
func (m *mockAnnounceRepoAPI) GetByID(ctx context.Context, id string) (*domain.Announcement, error) {
	return nil, nil
}
func (m *mockAnnounceRepoAPI) Create(ctx context.Context, a *domain.Announcement) error { return nil }
func (m *mockAnnounceRepoAPI) Update(ctx context.Context, a *domain.Announcement) error { return nil }
func (m *mockAnnounceRepoAPI) ToggleActive(ctx context.Context, id string) (*domain.Announcement, error) {
	return &domain.Announcement{ID: id, Active: true}, nil
}
func (m *mockAnnounceRepoAPI) Delete(ctx context.Context, id string) error         { return nil }
func (m *mockAnnounceRepoAPI) IncrementViews(ctx context.Context, id string) error { return nil }

type mockDRERepoAPI struct{}

func (m *mockDRERepoAPI) GetMetrics(ctx context.Context) (*domain.DREMetrics, error) {
	return &domain.DREMetrics{
		TotalDeposits:    100000.0,
		TotalWithdrawals: 20000.0,
		NetBankProfit:    80000.0,
		ProfitMargin:     80.0,
	}, nil
}

type mockConfigRepoAPI struct{}

func (m *mockConfigRepoAPI) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	return &domain.SystemConfig{GlobalWithdrawalsLocked: false}, nil
}
func (m *mockConfigRepoAPI) UpdateConfig(ctx context.Context, config *domain.SystemConfig) error {
	return nil
}
func (m *mockConfigRepoAPI) ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error {
	return nil
}
func (m *mockConfigRepoAPI) UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error {
	return nil
}
func (m *mockConfigRepoAPI) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, graceMinutes int) error {
	return nil
}
func (m *mockConfigRepoAPI) UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error {
	return nil
}
func (m *mockConfigRepoAPI) ResetSeason(ctx context.Context, durationDays int) error {
	return nil
}
func (m *mockConfigRepoAPI) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	return nil
}

func TestDoubleCheck_HTTP_RouterAndEndpoints(t *testing.T) {
	tm := jwt.NewTokenManager("super_jwt_secret_api_enterprise_key_2026", 24)
	hub := ws.NewHub()
	go hub.Run()

	passHash, _ := crypto.HashPassword("123456")
	user := &domain.User{
		ID:               "CB-458F",
		Username:         "mateus",
		PasswordHash:     passHash,
		Role:             domain.RoleUser,
		Balance:          2000.0,
		SecurityPassword: passHash,
	}
	userRepo := &mockUserRepoAPI{user: user}

	adminPassHash, _ := crypto.HashPassword("admin123")
	adminUser := &domain.User{
		ID:               "gst-001",
		Username:         "admin",
		PasswordHash:     adminPassHash,
		Role:             domain.RoleDiretoria,
		TwoFactorEnabled: true,
		TwoFactorSecret:  "configured-totp-secret",
	}
	userRepo.admin = adminUser
	adminToken, _ := tm.GenerateToken(adminUser)
	userToken, _ := tm.GenerateToken(user)

	catalogRepo := &mockCatalogRepoAPI{}
	machineRepo := &mockMachineRepoAPI{}
	txRepo := &mockTxRepoAPI{}
	rouletteRepo := &mockRouletteRepoAPI{}
	announceRepo := &mockAnnounceRepoAPI{}
	dreRepo := &mockDRERepoAPI{}
	configRepo := &mockConfigRepoAPI{}

	authUC := auth.NewAuthUseCase(userRepo, machineRepo, nil, nil, nil, tm)
	garageUC := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, nil, nil, nil, nil, nil)
	shopUC := shop.NewShopUseCase(userRepo, catalogRepo, machineRepo, txRepo)
	paymentUC := payment.NewPaymentUseCase(userRepo, txRepo, nil, configRepo, nil, "webhook_secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})
	rouletteUC := roulette.NewRouletteUseCase(userRepo, rouletteRepo, machineRepo, txRepo, nil)
	rankingUC := ranking.NewRankingUseCase(nil)
	adminUC := admin.NewAdminUseCase(userRepo, machineRepo, txRepo, dreRepo, catalogRepo, rouletteRepo, announceRepo, configRepo, nil)

	router := deliveryhttp.NewRouter(deliveryhttp.RouterConfig{
		TokenManager:    tm,
		AuthHandler:     handler.NewAuthHandler(authUC),
		UserHandler:     handler.NewUserHandler(authUC, garageUC, userRepo, nil),
		GarageHandler:   handler.NewGarageHandler(garageUC, hub),
		ShopHandler:     handler.NewShopHandler(shopUC, hub),
		FinHandler:      handler.NewFinancialHandler(paymentUC, hub),
		RefHandler:      handler.NewReferralHandler(nil, userRepo),
		RoulHandler:     handler.NewRouletteHandler(rouletteUC, hub),
		RankHandler:     handler.NewRankingHandler(rankingUC),
		AnnHandler:      handler.NewAnnouncementHandler(announceRepo),
		AdminHandler:    handler.NewAdminHandler(adminUC, userRepo, machineRepo, catalogRepo, txRepo, nil, rouletteRepo, announceRepo, configRepo, nil, nil, nil, hub),
		UserRepo:        userRepo,
		WSHub:           hub,
		EnableDevRoutes: true,
		SMSEnabled:      true,
	})

	// 1. Healthcheck
	t.Run("GET /health -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	// 2. Rotas Públicas
	t.Run("GET /api/v1/store/machines -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/store/machines", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/roulette/config -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/roulette/config", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/auth/send-sms sem repositório -> falha fechada", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]string{"phone": "11999999999"})
		req := httptest.NewRequest("POST", "/api/v1/auth/send-sms", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503 Service Unavailable, got %d", rr.Code)
		}
	})

	// 3. Rotas Protegidas do Operador com Token JWT
	t.Run("GET /api/v1/user com Bearer Token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/user", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/garage com Bearer Token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/garage", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/financial/transactions com Bearer Token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/financial/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rr.Code)
		}
	})

	// 4. Rotas Protegidas do ERP (Admin)
	t.Run("GET /api/v1/admin/dre com Admin Token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK para admin, got %d", rr.Code)
		}
	})

	t.Run("GET /api/v1/admin/dre com User Token -> 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/admin/dre", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("esperado 403 Forbidden para operador comum no painel ERP, got %d", rr.Code)
		}
	})

	t.Run("POST /api/v1/admin/withdrawals/tx-1/approve com Admin Token -> 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/withdrawals/tx-1/approve", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK para aprovação de saque pelo admin, got %d", rr.Code)
		}
	})

	// 5. Webhook PIX com Assinatura HMAC
	t.Run("POST /api/v1/webhooks/pix com assinatura HMAC válida -> 200 OK", func(t *testing.T) {
		txPayload := payment.WebhookPayload{
			TxID:       "tx-1",
			Status:     "paid",
			PaidAmount: 300.0,
		}
		rawBody, _ := json.Marshal(txPayload)
		payloadString := "tx-1:paid:300.00"
		sig := crypto.GenerateHMACSHA256(payloadString, "webhook_secret")

		req := httptest.NewRequest("POST", "/api/v1/webhooks/pix", bytes.NewBuffer(rawBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Signature", sig)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK no webhook assinado, got %d", rr.Code)
		}
	})
}
