package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catbrasil/backend/internal/delivery/http/handler"
	"catbrasil/backend/internal/delivery/ws"
	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/pkg/jwt"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// newWSObservationHub starts a real hub with a connected operator client so
// tests can observe exactly which realtime events are emitted.
func newWSObservationHub(t *testing.T, userID string) (*ws.Hub, *websocket.Conn) {
	t.Helper()
	hub := ws.NewHub()
	go hub.Run()
	tokens := jwt.NewTokenManager("admin-handler-ws-test-secret-123456", 1)
	server := httptest.NewServer(hub.HandleWS(tokens))
	t.Cleanup(server.Close)

	token, err := tokens.GenerateToken(&domain.User{ID: userID, Role: domain.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	dialer := websocket.Dialer{Subprotocols: []string{"bearer", token}}
	conn, _, err := dialer.Dial("ws"+server.URL[4:], nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	// Aguarda o registro do cliente no hub antes de emitir eventos.
	time.Sleep(30 * time.Millisecond)
	return hub, conn
}

// expectNoWSMessage fails if any realtime event arrives before the deadline.
func expectNoWSMessage(t *testing.T, conn *websocket.Conn, event string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(350 * time.Millisecond))
	var message ws.WebSocketMessage
	err := conn.ReadJSON(&message)
	if err == nil {
		t.Fatalf("%s NUNCA pode emitir evento WS, obteve: %+v", event, message)
	}
	if netErr, ok := err.(interface{ Timeout() bool }); !ok || !netErr.Timeout() {
		t.Fatalf("esperava timeout silencioso, obteve erro de leitura: %v", err)
	}
}

func expectWSMessage(t *testing.T, conn *websocket.Conn, wantEvent, wantUserID string) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var message ws.WebSocketMessage
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("evento %s deveria ser emitido: %v", wantEvent, err)
	}
	if message.Event != wantEvent || message.UserID != wantUserID {
		t.Fatalf("esperado %s para %s, obteve: %+v", wantEvent, wantUserID, message)
	}
}

func newWithdrawalAdminHandler(txRepo domain.TransactionRepository, hub *ws.Hub) *handler.AdminHandler {
	adminUC := admin.NewAdminUseCase(nil, nil, txRepo, nil, nil, nil, nil, nil, nil)
	return handler.NewAdminHandler(adminUC, nil, nil, nil, txRepo, nil, nil, nil, nil, nil, nil, nil, hub)
}

func callAdminWithdrawalEndpoint(t *testing.T, fn http.HandlerFunc, txID string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"txId": txID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/withdrawals/approve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	fn(rec, req)
	return rec
}

func TestAdminHandler_ApproveWithdrawal_EmitsNoWithdrawApprovedEvent(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	hub, conn := newWSObservationHub(t, "usr-wdr-1")

	net := 95.0
	txRepo := &mockTxRepo{
		approveWithdrawalFn: func(ctx context.Context, txID string) (*domain.Transaction, error) {
			return &domain.Transaction{
				ID:        txID,
				UserID:    "usr-wdr-1",
				Tipo:      "Saque PIX",
				Valor:     -100.0,
				NetAmount: &net,
				Status:    domain.TxStatusProcessando,
			}, nil
		},
	}
	h := newWithdrawalAdminHandler(txRepo, hub)

	rec := callAdminWithdrawalEndpoint(t, h.ApproveWithdrawal, "WDR-APPROVE-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK para aprovação, obteve %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Success bool               `json:"success"`
		Message string             `json:"message"`
		Tx      domain.Transaction `json:"tx"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatal("resposta HTTP deve confirmar o sucesso do envio para processamento")
	}
	if resp.Tx.Status != domain.TxStatusProcessando {
		t.Fatalf("aprovação deve devolver o estado real (processando), obteve: %s", resp.Tx.Status)
	}

	// Enviar/processar um saque não é pagamento: nenhum withdraw_approved (nem
	// nenhum outro evento) pode chegar ao cliente.
	expectNoWSMessage(t, conn, "withdraw_approved")
}

func TestAdminHandler_ApproveWithdrawal_RepeatedApprovalStillEmitsNoEvent(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	hub, conn := newWSObservationHub(t, "usr-wdr-2")

	net := 95.0
	txRepo := &mockTxRepo{
		approveWithdrawalFn: func(ctx context.Context, txID string) (*domain.Transaction, error) {
			// Estado já avançado: reenvio/repetição não move dinheiro de novo.
			return &domain.Transaction{
				ID:        txID,
				UserID:    "usr-wdr-2",
				Tipo:      "Saque PIX",
				Valor:     -100.0,
				NetAmount: &net,
				Status:    domain.TxStatusSucesso,
			}, nil
		},
	}
	h := newWithdrawalAdminHandler(txRepo, hub)

	for i := 0; i < 2; i++ {
		rec := callAdminWithdrawalEndpoint(t, h.ApproveWithdrawal, "WDR-APPROVE-2")
		if rec.Code != http.StatusOK {
			t.Fatalf("repetição %d deve responder 200, obteve %d", i+1, rec.Code)
		}
	}

	expectNoWSMessage(t, conn, "withdraw_approved")
}

func TestAdminHandler_RejectWithdrawal_EmitsWithdrawRejected(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	hub, conn := newWSObservationHub(t, "usr-wdr-3")

	txRepo := &mockTxRepo{
		rejectWithdrawalFn: func(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
			return &domain.Transaction{
				ID:     txID,
				UserID: "usr-wdr-3",
				Tipo:   "Saque PIX",
				Valor:  -100.0,
				Status: domain.TxStatusCancelado,
			}, nil
		},
	}
	h := newWithdrawalAdminHandler(txRepo, hub)

	rec := callAdminWithdrawalEndpoint(t, h.RejectWithdrawal, "WDR-REJECT-1")
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK para rejeição, obteve %d: %s", rec.Code, rec.Body.String())
	}

	// Controle positivo: a rejeição real da diretoria continua notificando.
	expectWSMessage(t, conn, "withdraw_rejected", "usr-wdr-3")
}

// mockERPConfigRepo registra o estado do gateway e a última chamada de
// UpdateGatewayConfig para assegurar que segredos não vazem no GET e que
// segredo vazio chegue intacto ao repositório (semântica "vazio preserva").
type mockERPConfigRepo struct {
	cfg              *domain.SystemConfig
	lastGateway      domain.GatewayConfig
	lastGraceMinutes int
	updateCalls      int
}

func (m *mockERPConfigRepo) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	return m.cfg, nil
}
func (m *mockERPConfigRepo) UpdateConfig(ctx context.Context, cfg *domain.SystemConfig) error {
	return nil
}
func (m *mockERPConfigRepo) ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error {
	return nil
}
func (m *mockERPConfigRepo) UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error {
	return nil
}
func (m *mockERPConfigRepo) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, graceMinutes int) error {
	m.lastGateway = gw
	m.lastGraceMinutes = graceMinutes
	m.updateCalls++
	return nil
}
func (m *mockERPConfigRepo) UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error {
	return nil
}
func (m *mockERPConfigRepo) ResetSeason(ctx context.Context, durationDays int) error { return nil }
func (m *mockERPConfigRepo) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	return nil
}

func newERPConfigAdminHandler(repo domain.ConfigRepository) *handler.AdminHandler {
	return handler.NewAdminHandler(nil, nil, nil, nil, nil, nil, nil, nil, repo, nil, nil, nil, nil)
}

func TestAdminHandler_GetSystemConfig_NeverExposesSecretsOrCredentials(t *testing.T) {
	const (
		canarySuitPayID       = "canary-suitpay-client-id-12345"
		canarySuitPaySecret   = "canary-suitpay-secret-67890"
		canarySuitPayCallback = "https://canary.domain/hook?token=canary_token_abc"
		canaryParadiseKey     = "canary-paradise-key-99999"
		canaryParadiseHash    = "canary-paradise-hash-88888"
		canaryParadisePost    = "https://canary.domain/postback?auth=canary_auth_xyz"
	)

	repo := &mockERPConfigRepo{cfg: &domain.SystemConfig{
		GlobalWithdrawalsLocked: false,
		Gateway: domain.GatewayConfig{
			Name:                "Gateway PIX Padrão",
			SuitPayClientID:     canarySuitPayID,
			SuitPayClientSecret: canarySuitPaySecret,
			SuitPayCallbackURL:  canarySuitPayCallback,
			SuitPayEnabled:      true,
			SuitPaySandbox:      false,
			ParadiseSecretKey:   canaryParadiseKey,
			ParadiseProductHash: canaryParadiseHash,
			ParadisePostbackURL: canaryParadisePost,
			ParadiseEnabled:     true,
		},
	}}
	h := newERPConfigAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	h.GetSystemConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK, obteve %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// Prova de ausência de canários no JSON de retorno
	canaries := []string{
		canarySuitPayID,
		canarySuitPaySecret,
		canarySuitPayCallback,
		canaryParadiseKey,
		canaryParadiseHash,
		canaryParadisePost,
	}
	for _, canary := range canaries {
		if strings.Contains(body, canary) {
			t.Fatalf("GET /admin/config expôs segredo canário %q no body: %s", canary, body)
		}
	}

	// Prova de ausência de campos de valor/chave no DTO de leitura
	bannedKeys := []string{
		`"suitpayClientId"`,
		`"suitpayClientSecret"`,
		`"suitpayCallbackUrl"`,
		`"paradiseSecretKey"`,
		`"paradiseProductHash"`,
		`"paradisePostbackUrl"`,
	}
	for _, bannedKey := range bannedKeys {
		if strings.Contains(body, bannedKey) {
			t.Fatalf("GET /admin/config não pode serializar o campo de valor %s: %s", bannedKey, body)
		}
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Gateway struct {
				Name                          string  `json:"name"`
				DepositFeePercent             float64 `json:"depositFeePercent"`
				WithdrawFeeFixed              float64 `json:"withdrawFeeFixed"`
				PixExpirationMinutes          int     `json:"pixExpirationMinutes"`
				SuitPaySandbox                bool    `json:"suitpaySandbox"`
				SuitPayEnabled                bool    `json:"suitpayEnabled"`
				ParadiseEnabled               bool    `json:"paradiseEnabled"`
				SuitPayClientIDConfigured     bool    `json:"suitpayClientIdConfigured"`
				SuitPayClientSecretConfigured bool    `json:"suitpayClientSecretConfigured"`
				SuitPayCallbackURLConfigured  bool    `json:"suitpayCallbackUrlConfigured"`
				ParadiseSecretKeyConfigured   bool    `json:"paradiseSecretKeyConfigured"`
				ParadiseProductHashConfigured bool    `json:"paradiseProductHashConfigured"`
				ParadisePostbackURLConfigured bool    `json:"paradisePostbackUrlConfigured"`
			} `json:"gateway"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatal("resposta deve confirmar sucesso")
	}
	gw := resp.Data.Gateway
	if !gw.SuitPayClientIDConfigured {
		t.Fatal("suitpayClientIdConfigured deve ser true quando ID configurado")
	}
	if !gw.SuitPayClientSecretConfigured {
		t.Fatal("suitpayClientSecretConfigured deve ser true quando segredo configurado")
	}
	if !gw.SuitPayCallbackURLConfigured {
		t.Fatal("suitpayCallbackUrlConfigured deve ser true quando URL callback configurada")
	}
	if !gw.ParadiseSecretKeyConfigured {
		t.Fatal("paradiseSecretKeyConfigured deve ser true quando chave configurada")
	}
	if !gw.ParadiseProductHashConfigured {
		t.Fatal("paradiseProductHashConfigured deve ser true quando hash configurado")
	}
	if !gw.ParadisePostbackURLConfigured {
		t.Fatal("paradisePostbackUrlConfigured deve ser true quando URL postback configurada")
	}
}

func TestAdminHandler_GetSystemConfig_ReportsSecretsNotConfigured(t *testing.T) {
	repo := &mockERPConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{Name: "Gateway PIX Padrão"},
	}}
	h := newERPConfigAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/config", nil)
	rec := httptest.NewRecorder()
	h.GetSystemConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK, obteve %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			Gateway struct {
				SuitPayClientIDConfigured     bool `json:"suitpayClientIdConfigured"`
				SuitPayClientSecretConfigured bool `json:"suitpayClientSecretConfigured"`
				SuitPayCallbackURLConfigured  bool `json:"suitpayCallbackUrlConfigured"`
				ParadiseSecretKeyConfigured   bool `json:"paradiseSecretKeyConfigured"`
				ParadiseProductHashConfigured bool `json:"paradiseProductHashConfigured"`
				ParadisePostbackURLConfigured bool `json:"paradisePostbackUrlConfigured"`
			} `json:"gateway"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	gw := resp.Data.Gateway
	if gw.SuitPayClientIDConfigured || gw.SuitPayClientSecretConfigured || gw.SuitPayCallbackURLConfigured {
		t.Fatal("flags de SuitPay devem ser false quando não configuradas")
	}
	if gw.ParadiseSecretKeyConfigured || gw.ParadiseProductHashConfigured || gw.ParadisePostbackURLConfigured {
		t.Fatal("flags de Paradise devem ser false quando não configuradas")
	}
}

func TestAdminHandler_UpdateGatewayConfig_EmptySecretForwardedForPreservation(t *testing.T) {
	repo := &mockERPConfigRepo{cfg: &domain.SystemConfig{}}
	h := newERPConfigAdminHandler(repo)

	payload := map[string]interface{}{
		"name":                   "Gateway PIX Padrão",
		"depositFeePercent":      1.5,
		"withdrawFeeFixed":       1.0,
		"pixExpirationMinutes":   15,
		"collectionGraceMinutes": 10,
		"suitpayClientId":        "",
		"suitpayClientSecret":    "",
		"suitpaySandbox":         true,
		"suitpayCallbackUrl":     "",
		"suitpayEnabled":         true,
		"paradiseSecretKey":      "",
		"paradiseProductHash":    "",
		"paradisePostbackUrl":    "",
		"paradiseEnabled":        false,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/config/gateway", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.UpdateGatewayConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK, obteve %d: %s", rec.Code, rec.Body.String())
	}
	if repo.updateCalls != 1 {
		t.Fatalf("esperado 1 chamada de UpdateGatewayConfig, obteve %d", repo.updateCalls)
	}
	if repo.lastGateway.SuitPayClientSecret != "" {
		t.Fatal("PUT com segredo vazio deve repassar string vazia ao repositório (semântica preserva)")
	}
	if repo.lastGateway.SuitPayClientID != "" {
		t.Fatal("PUT com client id vazio deve repassar string vazia ao repositório (semântica preserva)")
	}
	if repo.lastGateway.SuitPayCallbackURL != "" {
		t.Fatal("PUT com callback URL vazio deve repassar string vazia ao repositório (semântica preserva)")
	}
	if !repo.lastGateway.SuitPayEnabled || !repo.lastGateway.SuitPaySandbox {
		t.Fatal("flags do gateway devem ser atualizadas normalmente no PUT")
	}
}

func TestAdminHandler_UpdateGatewayConfig_CanaryRotationAndAbsenceInResponse(t *testing.T) {
	repo := &mockERPConfigRepo{cfg: &domain.SystemConfig{}}
	h := newERPConfigAdminHandler(repo)

	const (
		canaryNewID       = "canary-rotated-suitpay-id-444"
		canaryNewSecret   = "canary-rotated-suitpay-secret-555"
		canaryNewCallback = "https://canary.domain/callback-new?key=token_canary_666"
	)

	payload := map[string]interface{}{
		"name":                   "Gateway PIX Padrão",
		"depositFeePercent":      2.0,
		"withdrawFeeFixed":       1.5,
		"pixExpirationMinutes":   20,
		"collectionGraceMinutes": 10,
		"suitpayClientId":        canaryNewID,
		"suitpayClientSecret":    canaryNewSecret,
		"suitpaySandbox":         false,
		"suitpayCallbackUrl":     canaryNewCallback,
		"suitpayEnabled":         true,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/config/gateway", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.UpdateGatewayConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK, obteve %d: %s", rec.Code, rec.Body.String())
	}

	respBody := rec.Body.String()
	for _, canary := range []string{canaryNewID, canaryNewSecret, canaryNewCallback} {
		if strings.Contains(respBody, canary) {
			t.Fatalf("PUT /admin/config/gateway vazou canário na resposta: %s (canário: %s)", respBody, canary)
		}
	}
	if repo.lastGateway.SuitPayClientID != canaryNewID {
		t.Fatalf("repositório deveria receber novo ID rotacionado %q, obteve %q", canaryNewID, repo.lastGateway.SuitPayClientID)
	}
	if repo.lastGateway.SuitPayClientSecret != canaryNewSecret {
		t.Fatalf("repositório deveria receber novo secret rotacionado %q, obteve %q", canaryNewSecret, repo.lastGateway.SuitPayClientSecret)
	}
	if repo.lastGateway.SuitPayCallbackURL != canaryNewCallback {
		t.Fatalf("repositório deveria receber nova URL callback %q, obteve %q", canaryNewCallback, repo.lastGateway.SuitPayCallbackURL)
	}
}

func TestGatewayConfigJSON_OmitsAllSensitiveFields(t *testing.T) {
	gw := domain.GatewayConfig{
		SuitPayClientID:     "ci-canary",
		SuitPayClientSecret: "cs-canary",
		SuitPayCallbackURL:  "https://canary.app/cb?secret=xyz",
		ParadiseSecretKey:   "pk-canary",
		ParadiseProductHash: "hash-canary",
		ParadisePostbackURL: "https://canary.app/pb?token=abc",
	}
	raw, err := json.Marshal(gw)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, canary := range []string{"ci-canary", "cs-canary", "xyz", "pk-canary", "hash-canary", "abc"} {
		if strings.Contains(body, canary) {
			t.Fatalf("serialização do domínio vazou segredo: %s (canário: %s)", body, canary)
		}
	}
	for _, bannedKey := range []string{
		"suitpayClientId",
		"suitpayClientSecret",
		"suitpayCallbackUrl",
		"paradiseSecretKey",
		"paradiseProductHash",
		"paradisePostbackUrl",
	} {
		if strings.Contains(body, bannedKey) {
			t.Fatalf("serialização do domínio não pode emitir campo sensível %s: %s", bannedKey, body)
		}
	}
}

type mockAdminResetUserRepo struct {
	user      *domain.User
	getErr    error
	updateErr error
}

func (m *mockAdminResetUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.user, nil
}
func (m *mockAdminResetUserRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.user, nil
}
func (m *mockAdminResetUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockAdminResetUserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockAdminResetUserRepo) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockAdminResetUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockAdminResetUserRepo) Update(ctx context.Context, user *domain.User) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.user = user
	return nil
}
func (m *mockAdminResetUserRepo) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	return nil, nil
}
func (m *mockAdminResetUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockAdminResetUserRepo) Count(ctx context.Context) (int64, error)  { return 0, nil }
func (m *mockAdminResetUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

func TestAdminHandler_ResetUserSpin_Success(t *testing.T) {
	now := time.Now()
	user := &domain.User{
		ID:               "usr-reset-1",
		Username:         "operador_teste",
		AlreadySpunToday: true,
		LastSpinDate:     "2026-10-19",
		LastSpinAt:       &now,
	}
	userRepo := &mockAdminResetUserRepo{user: user}
	h := handler.NewAdminHandler(nil, userRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	r := chi.NewRouter()
	r.Post("/admin/users/{userId}/reset-spin", h.ResetUserSpin)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/usr-reset-1/reset-spin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK, obteve %d: %s", rec.Code, rec.Body.String())
	}
	if user.AlreadySpunToday {
		t.Fatal("esperava AlreadySpunToday = false após reset")
	}
	if user.LastSpinDate != "" {
		t.Fatalf("esperava LastSpinDate limpo, obteve %q", user.LastSpinDate)
	}
	if user.LastSpinAt != nil {
		t.Fatal("esperava LastSpinAt limpo (nil)")
	}
}

func TestAdminHandler_ResetUserSpin_UpdateErrorReturns500(t *testing.T) {
	now := time.Now()
	user := &domain.User{
		ID:               "usr-reset-fail",
		Username:         "operador_falha",
		AlreadySpunToday: true,
		LastSpinDate:     "2026-10-19",
		LastSpinAt:       &now,
	}
	userRepo := &mockAdminResetUserRepo{
		user:      user,
		updateErr: errors.New("db write failure"),
	}
	h := handler.NewAdminHandler(nil, userRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	r := chi.NewRouter()
	r.Post("/admin/users/{userId}/reset-spin", h.ResetUserSpin)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/usr-reset-fail/reset-spin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado 500 Internal Server Error quando Update falha, obteve %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminHandler_ResetUserSpin_NotFoundReturns404(t *testing.T) {
	userRepo := &mockAdminResetUserRepo{user: nil}
	h := handler.NewAdminHandler(nil, userRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	r := chi.NewRouter()
	r.Post("/admin/users/{userId}/reset-spin", h.ResetUserSpin)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/usr-unknown/reset-spin", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404 Not Found para usuário inexistente, obteve %d: %s", rec.Code, rec.Body.String())
	}
}
