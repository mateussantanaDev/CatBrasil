package payment_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/pkg/crypto"
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}
func (m *mockUserRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.users[id], nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}
func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	m.users[user.ID] = user
	return nil
}
func (m *mockUserRepo) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	u := m.users[id]
	u.Balance += deltaB
	u.YieldBalanceAvailable += deltaY
	return u, nil
}
func (m *mockUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepo) Count(ctx context.Context) (int64, error)  { return 1, nil }
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

type mockTxRepo struct {
	txs map[string]*domain.Transaction
}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return m.txs[id], nil
}
func (m *mockTxRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return m.txs[id], nil
}
func (m *mockTxRepo) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return nil, 0, nil
}
func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}
func (m *mockTxRepo) AttachDepositQRCode(ctx context.Context, id, externalID, pixCode, qrURL string) (*domain.Transaction, error) {
	tx := m.txs[id]
	tx.ExternalID, tx.PixCode, tx.QRCodeURL = &externalID, &pixCode, &qrURL
	return tx, nil
}

type mockIdentityRepo struct{ person domain.IdentityPerson }

func (m *mockIdentityRepo) CreateUserWithIdentity(context.Context, *domain.User, domain.IdentityPerson) error {
	return nil
}
func (m *mockIdentityRepo) Submit(context.Context, string, domain.IdentityPerson) error { return nil }
func (m *mockIdentityRepo) Status(context.Context, string) (string, error)              { return "verified", nil }
func (m *mockIdentityRepo) VerifiedPerson(context.Context, string) (domain.IdentityPerson, error) {
	return m.person, nil
}
func (m *mockIdentityRepo) Decide(context.Context, string, string, string, string) error { return nil }
func (m *mockIdentityRepo) Pending(context.Context) ([]string, error)                    { return nil, nil }
func (m *mockIdentityRepo) Inspect(context.Context, string, string) (domain.IdentityPerson, error) {
	return m.person, nil
}

func TestSuitPayCashInUsesReviewedPayer(t *testing.T) {
	var sent struct {
		RequestNumber string `json:"requestNumber"`
		Client        struct {
			Name     string `json:"name"`
			Document string `json:"document"`
			Email    string `json:"email"`
		} `json:"client"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/gateway/request-qrcode" {
			t.Errorf("rota incorreta: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Errorf("payload inválido: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"provider-identity-1","paymentCode":"STAGING-ONLY-code"}`))
	}))
	defer server.Close()
	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste Sintetica", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	txs := newMockTrackerTxRepo(map[string]*domain.Transaction{})
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}
	uc := payment.NewPaymentUseCase(users, txs, nil, cfg, nil, "webhook-secret", payment.Options{Mode: payment.ModeSuitPay, Environment: "staging", PixSandboxURL: server.URL + "/api/v1/gateway", IdentityRepo: &mockIdentityRepo{person: person}})
	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 25})
	if err != nil {
		t.Fatal(err)
	}
	if sent.Client.Name != person.LegalName || sent.Client.Document != person.CPF || sent.Client.Email != person.Email || sent.RequestNumber != res.TxID {
		t.Fatalf("pagador incorreto: %+v", sent)
	}
	if txs.txs[res.TxID] == nil || txs.txs[res.TxID].ExternalID == nil || *txs.txs[res.TxID].ExternalID != "provider-identity-1" {
		t.Fatal("cobrança não associada")
	}
	if res.ExpirationMinutes != 0 || res.QRCodeURL != "" {
		t.Fatal("provedor não garante prazo em minutos nem imagem QR")
	}
}
func (m *mockTxRepo) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	return nil, nil
}

func TestWebhookRejectsUnsignedAndIneligibleCredits(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{ID: "u-1", Username: "operador"}
	users := &mockUserRepo{users: map[string]*domain.User{"u-1": user}}
	tx := &domain.Transaction{ID: "PIX-1", UserID: user.ID, Tipo: "Depósito PIX", Valor: 100, Status: domain.TxStatusPendente}
	txs := &mockTxRepo{txs: map[string]*domain.Transaction{tx.ID: tx}}
	uc := payment.NewPaymentUseCase(users, txs, nil, nil, nil, "webhook-secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})
	for _, candidate := range []struct {
		payload payment.WebhookPayload
		sig     string
	}{
		{payment.WebhookPayload{TxID: tx.ID, Status: "paid", PaidAmount: 100}, ""},
		{payment.WebhookPayload{TxID: tx.ID, Status: "paid", PaidAmount: 100}, "invalid"},
		{payment.WebhookPayload{TxID: tx.ID, Status: "unpaid", PaidAmount: 100}, crypto.GenerateHMACSHA256("PIX-1:unpaid:100.00", "webhook-secret")},
		{payment.WebhookPayload{TxID: tx.ID, Status: "paid", PaidAmount: 200}, crypto.GenerateHMACSHA256("PIX-1:paid:200.00", "webhook-secret")},
	} {
		if result := uc.ProcessWebhook(ctx, candidate.payload, candidate.sig); result.Success {
			t.Fatalf("crédito indevido aceito: %+v", candidate.payload)
		}
		if user.Balance != 0 || tx.Status != domain.TxStatusPendente {
			t.Fatal("webhook rejeitado alterou o estado financeiro")
		}
	}
	tx.Tipo = "Saque PIX"
	payload := payment.WebhookPayload{TxID: tx.ID, Status: "paid", PaidAmount: 100}
	if uc.ProcessWebhook(ctx, payload, crypto.GenerateHMACSHA256("PIX-1:paid:100.00", "webhook-secret")).Success {
		t.Fatal("transação de saque não pode receber crédito de depósito")
	}
}

func TestProductionBlocksSimulatedAndIncompleteRealMoneyFlows(t *testing.T) {
	user := &domain.User{ID: "u-1", Username: "operador", Phone: "11999999999"}
	users := &mockUserRepo{users: map[string]*domain.User{user.ID: user}}
	txs := &mockTxRepo{txs: map[string]*domain.Transaction{}}
	simulated := payment.NewPaymentUseCase(users, txs, nil, nil, nil, "secret", payment.Options{Mode: payment.ModeSimulated, Environment: "production"})
	if res, err := simulated.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: user.ID, Amount: 50}); res != nil || !errors.Is(err, domain.ErrDepositUnavailable) {
		t.Fatalf("produção deve rejeitar QR simulado: response=%+v, err=%v", res, err)
	}
	if len(txs.txs) != 0 {
		t.Fatal("depósito simulado não pode ser persistido em produção")
	}
	if simulated.ProcessSimulatedWebhook(context.Background(), payment.WebhookPayload{}).Success {
		t.Fatal("simulação não pode creditar em production")
	}
	real := payment.NewPaymentUseCase(users, txs, nil, &mockConfigRepo{cfg: &domain.SystemConfig{}}, nil, "secret", payment.Options{Mode: payment.ModeSuitPay, Environment: "production"})
	if _, err := real.RequestPayout(context.Background(), payment.PayoutRequest{UserID: user.ID, Amount: 100}); err == nil {
		t.Fatal("saque real incompleto deve falhar fechado")
	}
}

type mockReferralRepo struct {
	referrals []*domain.Referral
}

func (m *mockReferralRepo) GetByReferrerID(ctx context.Context, referrerID string) ([]*domain.Referral, error) {
	return m.referrals, nil
}
func (m *mockReferralRepo) GetAll(ctx context.Context) ([]*domain.Referral, error) {
	return m.referrals, nil
}
func (m *mockReferralRepo) GetByID(ctx context.Context, id string) (*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepo) GetByReferredName(ctx context.Context, name string) (*domain.Referral, error) {
	for _, r := range m.referrals {
		if r.Nome == name {
			return r, nil
		}
	}
	return nil, nil
}
func (m *mockReferralRepo) Create(ctx context.Context, ref *domain.Referral) error { return nil }
func (m *mockReferralRepo) Update(ctx context.Context, ref *domain.Referral) error { return nil }
func (m *mockReferralRepo) CountActiveByReferrerID(ctx context.Context, referrerID string) (int, error) {
	count := 0
	for _, r := range m.referrals {
		if r.Status == domain.ReferralStatusAtivo {
			count++
		}
	}
	return count, nil
}

type mockConfigRepo struct {
	cfg *domain.SystemConfig
	err error
}

func (m *mockConfigRepo) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cfg, nil
}
func (m *mockConfigRepo) UpdateConfig(ctx context.Context, cfg *domain.SystemConfig) error {
	m.cfg = cfg
	return nil
}
func (m *mockConfigRepo) ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error { return nil }
func (m *mockConfigRepo) UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error {
	return nil
}
func (m *mockConfigRepo) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, graceMinutes int) error {
	return nil
}
func (m *mockConfigRepo) UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error {
	return nil
}
func (m *mockConfigRepo) ResetSeason(ctx context.Context, durationDays int) error {
	return nil
}
func (m *mockConfigRepo) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	return nil
}

type mockLogRepo struct{}

func (m *mockLogRepo) LogUsage(ctx context.Context, eventType string, amount float64) error {
	return nil
}
func (m *mockLogRepo) LogSecurity(ctx context.Context, userID *string, event, ip, fingerprint, details string) error {
	return nil
}
func (m *mockLogRepo) GetUsageLogs(ctx context.Context, limit, offset int) ([]*domain.UsageLog, error) {
	return nil, nil
}
func (m *mockLogRepo) GetSecurityLogs(ctx context.Context, userID *string, limit, offset int) ([]*domain.SecurityLog, error) {
	return nil, nil
}

func TestPaymentUseCase_CreatePixDeposit(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{ID: "USR-1", Username: "mateus"}
	userRepo := &mockUserRepo{users: map[string]*domain.User{"USR-1": user}}
	txRepo := &mockTxRepo{txs: make(map[string]*domain.Transaction)}
	referralRepo := &mockReferralRepo{}
	configRepo := &mockConfigRepo{cfg: &domain.SystemConfig{}}
	logRepo := &mockLogRepo{}

	uc := payment.NewPaymentUseCase(userRepo, txRepo, referralRepo, configRepo, logRepo, "secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})

	res, err := uc.CreatePixDeposit(ctx, payment.PixDepositRequest{
		UserID: "USR-1",
		Amount: 500.0,
	})
	if err != nil {
		t.Fatalf("erro ao gerar PIX: %v", err)
	}

	if res.Amount != 500.0 || res.PixCode == "" || res.QRCodeURL != "" {
		t.Errorf("dados de PIX inválidos: %+v", res)
	}

	if len(txRepo.txs) != 1 {
		t.Errorf("esperado 1 transação registrada como pendente")
	}
}

func TestPaymentUseCase_ProcessWebhook_ConfirmedWithCommission(t *testing.T) {
	ctx := context.Background()
	referrer := &domain.User{ID: "REF-1", Username: "patrocinador", Balance: 100.0}
	guest := &domain.User{ID: "GUEST-1", Username: "convidado_novo", Balance: 0.0}

	usersMap := map[string]*domain.User{
		"REF-1":   referrer,
		"GUEST-1": guest,
	}

	tx := &domain.Transaction{
		ID:     "PIX-100",
		UserID: "GUEST-1",
		Tipo:   "Depósito PIX",
		Valor:  1000.0,
		Status: domain.TxStatusPendente,
	}
	txsMap := map[string]*domain.Transaction{"PIX-100": tx}

	referrals := []*domain.Referral{
		{ID: "ref-1", ReferrerID: "REF-1", Nome: "convidado_novo", Status: domain.ReferralStatusInativo},
	}

	userRepo := &mockUserRepo{users: usersMap}
	txRepo := &mockTxRepo{txs: txsMap}
	referralRepo := &mockReferralRepo{referrals: referrals}
	configRepo := &mockConfigRepo{cfg: &domain.SystemConfig{}}
	logRepo := &mockLogRepo{}

	uc := payment.NewPaymentUseCase(userRepo, txRepo, referralRepo, configRepo, logRepo, "secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})

	result := uc.ProcessWebhook(ctx, payment.WebhookPayload{
		TxID:       "PIX-100",
		Status:     "paid",
		PaidAmount: 1000.0,
	}, crypto.GenerateHMACSHA256("PIX-100:paid:1000.00", "secret"))

	if !result.Success {
		t.Fatalf("webhook deveria ter sido processado com sucesso: %s", result.Error)
	}
	if result.Outcome != "applied" || result.PaidAmount != 1000 {
		t.Fatalf("primeira confirmação deve carregar resultado aplicado e valor no payload WS: %+v", result)
	}
	replay := uc.ProcessWebhook(ctx, payment.WebhookPayload{
		TxID: "PIX-100", Status: "paid", PaidAmount: 1000,
	}, crypto.GenerateHMACSHA256("PIX-100:paid:1000.00", "secret"))
	if !replay.Success || replay.Outcome != "duplicate" {
		t.Fatalf("replay não pode parecer novo crédito no WebSocket: %+v", replay)
	}

	if guest.Balance != 1000.0 || guest.RequiredRollover != 1000.0 {
		t.Errorf("saldo ou rollover do convidado inválidos: Balance=%.2f, Rollover=%.2f", guest.Balance, guest.RequiredRollover)
	}

	expectedReferrerBalance := 150.0
	if referrer.Balance != expectedReferrerBalance {
		t.Errorf("esperado saldo do patrocinador R$ %.2f, obteve: R$ %.2f", expectedReferrerBalance, referrer.Balance)
	}
}

func TestPaymentUseCase_RequestPayout_RolloverAndSecurity(t *testing.T) {
	ctx := context.Background()
	pinHash, _ := crypto.HashPassword("123456")
	user := &domain.User{
		ID:               "USR-1",
		Username:         "mateus",
		Balance:          1000.0,
		RequiredRollover: 500.0,
		CurrentRollover:  200.0,
		SecurityPassword: pinHash,
	}

	userRepo := &mockUserRepo{users: map[string]*domain.User{"USR-1": user}}
	txRepo := &mockTxRepo{txs: make(map[string]*domain.Transaction)}
	referralRepo := &mockReferralRepo{}
	configRepo := &mockConfigRepo{cfg: &domain.SystemConfig{}}
	logRepo := &mockLogRepo{}

	uc := payment.NewPaymentUseCase(userRepo, txRepo, referralRepo, configRepo, logRepo, "secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})

	_, err := uc.RequestPayout(ctx, payment.PayoutRequest{
		UserID:           "USR-1",
		Amount:           100.0,
		PixKey:           "cpf-123",
		SecurityPassword: "123456",
	})
	if err == nil {
		t.Fatalf("esperado erro de rollover não atingido")
	}

	user.CurrentRollover = 600.0

	_, err2 := uc.RequestPayout(ctx, payment.PayoutRequest{
		UserID:           "USR-1",
		Amount:           100.0,
		PixKey:           "cpf-123",
		SecurityPassword: "999999",
	})
	if err2 != domain.ErrInvalidSecurityPassword {
		t.Fatalf("esperado ErrInvalidSecurityPassword, obteve: %v", err2)
	}

	res, err3 := uc.RequestPayout(ctx, payment.PayoutRequest{
		UserID:           "USR-1",
		Amount:           200.0,
		PixKey:           "cpf-123",
		SecurityPassword: "123456",
	})
	if err3 != nil {
		t.Fatalf("erro inesperado no saque: %v", err3)
	}

	if res.NetAmount != 190.0 || res.Fee != 10.0 {
		t.Errorf("cálculo de taxa de saque inválido: Net=%.2f, Fee=%.2f", res.NetAmount, res.Fee)
	}

	if user.Balance != 800.0 {
		t.Errorf("esperado saldo final R$ 800.00, obteve: R$ %.2f", user.Balance)
	}
}

func TestTOTP_GoogleAuthenticator(t *testing.T) {
	secret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("falha ao gerar segredo TOTP: %v", err)
	}

	code, err := crypto.GenerateTOTPCode(secret, time.Now())
	if err != nil {
		t.Fatalf("falha ao gerar código TOTP: %v", err)
	}

	if len(code) != 6 {
		t.Errorf("esperado código de 6 dígitos, obteve: %s", code)
	}

	if !crypto.VerifyTOTPCode(secret, code) {
		t.Errorf("validação de código TOTP falhou")
	}
}

type mockTrackerTxRepo struct {
	*mockTxRepo
	startCalls  int
	startFunc   func(ctx context.Context, txID string) error
	issueCalls  int
	issueFunc   func(ctx context.Context, txID, reason, externalID string) error
	intentCalls int
	intentFunc  func(ctx context.Context, requestNumber, externalID string) (*domain.Transaction, error)
}

func (m *mockTrackerTxRepo) MarkDepositSubmissionStarted(ctx context.Context, txID string) error {
	m.startCalls++
	if m.startFunc != nil {
		return m.startFunc(ctx, txID)
	}
	return nil
}

func (m *mockTrackerTxRepo) MarkDepositSubmissionIssue(ctx context.Context, txID, reason, externalID string) error {
	m.issueCalls++
	if m.issueFunc != nil {
		return m.issueFunc(ctx, txID, reason, externalID)
	}
	if tx, ok := m.txs[txID]; ok {
		if tx.Status != domain.TxStatusSucesso {
			tx.Status = domain.TxStatusProcessando
			tx.RejectionReason = &reason
			if externalID != "" {
				tx.ExternalID = &externalID
			}
		}
	}
	return nil
}

func (m *mockTrackerTxRepo) MarkDepositIntentSent(ctx context.Context, requestNumber, externalID string) (*domain.Transaction, error) {
	m.intentCalls++
	if m.intentFunc != nil {
		return m.intentFunc(ctx, requestNumber, externalID)
	}
	if tx, ok := m.txs[requestNumber]; ok {
		tx.ExternalID = &externalID
		return tx, nil
	}
	return nil, domain.ErrTransactionNotFound
}

func newMockTrackerTxRepo(txs map[string]*domain.Transaction) *mockTrackerTxRepo {
	if txs == nil {
		txs = make(map[string]*domain.Transaction)
	}
	return &mockTrackerTxRepo{
		mockTxRepo: &mockTxRepo{txs: txs},
	}
}

type mockApplierTxRepo struct {
	*mockTrackerTxRepo
	applyFunc func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error)
}

func (m *mockApplierTxRepo) ApplyDepositEvent(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
	if m.applyFunc != nil {
		return m.applyFunc(ctx, requestNumber, externalID, status, amount)
	}
	return nil, "", nil
}

func newMockApplierTxRepo(txs map[string]*domain.Transaction) *mockApplierTxRepo {
	return &mockApplierTxRepo{
		mockTrackerTxRepo: newMockTrackerTxRepo(txs),
	}
}

func makeSignedSuitPayWebhook(idTx any, typeTx, statusTx string, value float64, reqNumber, secret string) []byte {
	valStr := fmt.Sprintf("%.2f", value)
	idStr := fmt.Sprint(idTx)
	values := idStr + typeTx + statusTx + valStr + reqNumber
	digest := sha256.Sum256([]byte(secret + values))
	hash := hex.EncodeToString(digest[:])
	return []byte(fmt.Sprintf(`{"idTransaction":%s,"typeTransaction":"%s","statusTransaction":"%s","value":%s,"requestNumber":"%s","hash":"%s"}`,
		idStr, typeTx, statusTx, valStr, reqNumber, hash))
}

func makeSignedSuitPayWebhookWithStringID(idStr, typeTx, statusTx string, value float64, reqNumber, secret string) []byte {
	valStr := fmt.Sprintf("%.2f", value)
	values := idStr + typeTx + statusTx + valStr + reqNumber
	digest := sha256.Sum256([]byte(secret + values))
	hash := hex.EncodeToString(digest[:])
	return []byte(fmt.Sprintf(`{"idTransaction":"%s","typeTransaction":"%s","statusTransaction":"%s","value":%s,"requestNumber":"%s","hash":"%s"}`,
		idStr, typeTx, statusTx, valStr, reqNumber, hash))
}

func TestSuitPayCashIn_TimeoutPersistsAmbiguousState(t *testing.T) {
	// Fake SuitPay gateway server that simulates timeout / connection error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	rawTxs := map[string]*domain.Transaction{}
	baseTxRepo := newMockTrackerTxRepo(rawTxs)
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, baseTxRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro por falha/timeout no gateway")
	}
	if !errors.Is(err, domain.ErrDepositUnavailable) {
		t.Fatalf("falha SuitPay deve sinalizar indisponibilidade: %v", err)
	}
	if res != nil {
		t.Fatal("cliente não deve receber PIX em caso de timeout")
	}

	if len(rawTxs) != 1 {
		t.Fatalf("esperada 1 transação registrada no banco, obteve %d", len(rawTxs))
	}
	var recordedTx *domain.Transaction
	for _, tx := range rawTxs {
		recordedTx = tx
		break
	}
	if recordedTx.Status != domain.TxStatusProcessando {
		t.Fatalf("esperado status 'processando' para reconciliação, obteve %s", recordedTx.Status)
	}
	if recordedTx.GatewayProvider != string(payment.ModeSuitPay) || recordedTx.PixCode != nil {
		t.Fatalf("falha real não pode gerar QR sintético: %+v", recordedTx)
	}
	if recordedTx.RejectionReason == nil || !strings.Contains(*recordedTx.RejectionReason, "gateway_submission_issue") {
		t.Fatalf("motivo de issue de submissão não registrado: %v", recordedTx.RejectionReason)
	}
}

func TestSuitPayCashIn_StagingRequiresSandboxBeforeGatewayCall(t *testing.T) {
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer"}}}
	txs := map[string]*domain.Transaction{}
	uc := payment.NewPaymentUseCase(users, newMockTrackerTxRepo(txs), nil,
		&mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{
			SuitPayEnabled: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs",
		}}}, nil, "secret", payment.Options{
			Mode: payment.ModeSuitPay, Environment: "staging", PixSandboxURL: server.URL,
			IdentityRepo: &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste"}},
		})
	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if res != nil || !errors.Is(err, domain.ErrDepositUnavailable) {
		t.Fatalf("staging sem sandbox deve falhar fechado: response=%+v, err=%v", res, err)
	}
	if serverCalls != 0 || len(txs) != 0 {
		t.Fatalf("staging sem sandbox não pode chamar gateway nem criar depósito: calls=%d, txs=%d", serverCalls, len(txs))
	}
}

func TestSuitPayCashIn_DevelopmentCannotReachLiveGateway(t *testing.T) {
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer"}}}
	txs := map[string]*domain.Transaction{}
	uc := payment.NewPaymentUseCase(users, newMockTrackerTxRepo(txs), nil,
		&mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{
			SuitPayEnabled: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs",
		}}}, nil, "secret", payment.Options{
			Mode: payment.ModeSuitPay, Environment: "development", LiveEnabled: true,
			IdentityRepo: &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste"}},
		})
	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if res != nil || !errors.Is(err, domain.ErrDepositUnavailable) || len(txs) != 0 {
		t.Fatalf("desenvolvimento sem sandbox não pode criar cobrança real: response=%+v, err=%v, txs=%d", res, err, len(txs))
	}
}

func TestSuitPayCashIn_IncompleteResponse_PreservesExternalID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// idTransaction provided, but paymentCode is empty (missing QR)
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"provider-inc-999","paymentCode":""}`))
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	rawTxs := map[string]*domain.Transaction{}
	baseTxRepo := newMockTrackerTxRepo(rawTxs)
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, baseTxRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro de resposta incompleta da SuitPay")
	}
	if !errors.Is(err, domain.ErrDepositUnavailable) {
		t.Fatalf("resposta SuitPay incompleta deve sinalizar indisponibilidade: %v", err)
	}
	if res != nil {
		t.Fatal("cliente não deve receber PIX quando código está ausente")
	}

	var recordedTx *domain.Transaction
	for _, tx := range rawTxs {
		recordedTx = tx
		break
	}
	if recordedTx == nil {
		t.Fatal("transação não registrada")
	}
	if recordedTx.Status != domain.TxStatusProcessando {
		t.Fatalf("esperado status 'processando' para reconciliação, obteve %s", recordedTx.Status)
	}
	if recordedTx.ExternalID == nil || *recordedTx.ExternalID != "provider-inc-999" {
		t.Fatalf("external_id deve ser preservado para reconciliação: %v", recordedTx.ExternalID)
	}
	if recordedTx.RejectionReason == nil || !strings.Contains(*recordedTx.RejectionReason, "gateway_incomplete_response") {
		t.Fatalf("motivo incompleto deve ser registrado: %v", recordedTx.RejectionReason)
	}
}

func TestSuitPayCashIn_IncompleteResponse_MissingTransactionID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// paymentCode provided, but idTransaction is missing
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"","paymentCode":"some-pix-code"}`))
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	rawTxs := map[string]*domain.Transaction{}
	baseTxRepo := newMockTrackerTxRepo(rawTxs)
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, baseTxRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro quando idTransaction está ausente")
	}
	if res != nil {
		t.Fatal("cliente não deve receber PIX inválido")
	}

	var recordedTx *domain.Transaction
	for _, tx := range rawTxs {
		recordedTx = tx
		break
	}
	if recordedTx.Status != domain.TxStatusProcessando {
		t.Fatalf("esperado status 'processando', obteve %s", recordedTx.Status)
	}
}

func TestSuitPayCashIn_TrackerStartFailure_GatewayNeverCalled(t *testing.T) {
	httpCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"should-never-reach","paymentCode":"pix"}`))
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	rawTxs := map[string]*domain.Transaction{}
	trackerRepo := newMockTrackerTxRepo(rawTxs)
	trackerRepo.startFunc = func(ctx context.Context, txID string) error {
		return errors.New("db disk full on tracker start")
	}
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, trackerRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro quando MarkDepositSubmissionStarted falha")
	}
	if res != nil {
		t.Fatal("nenhum resultado deve ser retornado se início de submissão falhar")
	}
	if httpCalls != 0 {
		t.Fatalf("o gateway NUNCA deve ser chamado se o tracker falhar antes do envio! Chamadas: %d", httpCalls)
	}
}

func TestSuitPayCashIn_TrackerAbsent_GatewayNeverCalled(t *testing.T) {
	httpCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"should-never-reach","paymentCode":"pix"}`))
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	plainRepo := &mockTxRepo{txs: map[string]*domain.Transaction{}} // does not implement DepositSubmissionTracker
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, plainRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	res, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro quando DepositTracker estiver ausente")
	}
	if res != nil {
		t.Fatal("nenhum resultado deve ser retornado se tracker estiver ausente")
	}
	if httpCalls != 0 {
		t.Fatalf("o gateway NUNCA deve ser chamado se o tracker estiver ausente! Chamadas: %d", httpCalls)
	}
}

func TestSuitPayCashIn_ContextCanceledAfterSend_RecordsIssueWithFreshTimeout(t *testing.T) {
	// Gateway server delays enough for caller context to cancel
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"late-tx","paymentCode":"code"}`))
	}))
	defer server.Close()

	person := domain.IdentityPerson{CPF: "52998224725", LegalName: "Pessoa Teste", Email: "teste@example.invalid"}
	users := &mockUserRepo{users: map[string]*domain.User{"payer": {ID: "payer", Username: "alias", Phone: "+5511999999999"}}}
	rawTxs := map[string]*domain.Transaction{}
	trackerRepo := newMockTrackerTxRepo(rawTxs)

	var wasActiveDuringCall bool
	var hadDeadline bool
	var capturedDeadline time.Time
	trackerRepo.issueFunc = func(ctx context.Context, txID, reason, externalID string) error {
		wasActiveDuringCall = (ctx.Err() == nil)
		capturedDeadline, hadDeadline = ctx.Deadline()
		if tx, ok := rawTxs[txID]; ok {
			tx.Status = domain.TxStatusProcessando
			tx.RejectionReason = &reason
		}
		return nil
	}

	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{Gateway: domain.GatewayConfig{SuitPayEnabled: true, SuitPaySandbox: true, SuitPayClientID: "ci", SuitPayClientSecret: "cs"}}}

	uc := payment.NewPaymentUseCase(users, trackerRepo, nil, cfg, nil, "webhook-secret", payment.Options{
		Mode:          payment.ModeSuitPay,
		Environment:   "staging",
		PixSandboxURL: server.URL + "/api/v1/gateway",
		IdentityRepo:  &mockIdentityRepo{person: person},
	})

	// Caller ctx with very tight timeout so it cancels during HTTP call
	callerCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	res, err := uc.CreatePixDeposit(callerCtx, payment.PixDepositRequest{UserID: "payer", Amount: 50})
	if err == nil {
		t.Fatal("esperado erro por cancelamento de contexto")
	}
	if res != nil {
		t.Fatal("cliente não deve receber PIX em caso de cancelamento")
	}

	if trackerRepo.issueCalls != 1 {
		t.Fatalf("esperada 1 chamada ao MarkDepositSubmissionIssue, obteve %d", trackerRepo.issueCalls)
	}

	// Verify that the context passed to MarkDepositSubmissionIssue was alive (fresh timeout), NOT dead callerCtx
	if !wasActiveDuringCall {
		t.Fatal("contexto de ocorrência deveria estar ativo durante a chamada ao repositório")
	}
	if !hadDeadline {
		t.Fatal("contexto de ocorrência deve ter timeout configurado")
	}
	if time.Until(capturedDeadline) > 5*time.Second {
		t.Fatalf("timeout do contexto de ocorrência não deve exceder 5 segundos: %v", time.Until(capturedDeadline))
	}
}

func TestSuitPayWebhook_DepositEventApplier_Outcomes(t *testing.T) {
	clientSecret := "test-secret"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	t.Run("PAID_OUT applied", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			if status != "PAID_OUT" || amount != 150.0 || requestNumber != "PIX-101" {
				return nil, "", fmt.Errorf("parâmetros incorretos no apply: %s %f %s", status, amount, requestNumber)
			}
			return &domain.Transaction{ID: requestNumber, UserID: "usr-1", Status: domain.TxStatusSucesso}, "applied", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(42, "PIX", "PAID_OUT", 150.00, "PIX-101", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("webhook deveria ter sido bem sucedido: %s", result.Error)
		}
		if result.Outcome != "applied" || result.Kind != "deposit" || result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("resultado inesperado: %+v", result)
		}
	})

	t.Run("CHARGEBACK applied", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			if status != "CHARGEBACK" || amount != 200.0 {
				return nil, "", fmt.Errorf("chargeback com parâmetros inválidos: %s %f", status, amount)
			}
			return &domain.Transaction{ID: requestNumber, UserID: "usr-2", Status: domain.TxStatusCancelado}, "applied", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(43, "PIX", "CHARGEBACK", 200.00, "PIX-102", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("chargeback válido deve ser aceito: %s", result.Error)
		}
		if result.Outcome != "applied" || result.Kind != "chargeback" || result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("resultado inesperado para chargeback: %+v", result)
		}
	})

	t.Run("Duplicate callback idempotent", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-3", Status: domain.TxStatusSucesso}, "duplicate", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(44, "PIX", "PAID_OUT", 50.00, "PIX-103", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("duplicata idempotente deve ter sucesso: %s", result.Error)
		}
		if result.Outcome != "duplicate" || result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("resultado inesperado para duplicata: %+v", result)
		}
	})

	t.Run("Unrecognized authenticated event queued", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			if status != "UNDER_ANALYSIS" {
				return nil, "", fmt.Errorf("status inesperado: %s", status)
			}
			return &domain.Transaction{ID: requestNumber, UserID: "usr-4", Status: domain.TxStatusProcessando}, "queued", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(45, "PIX", "UNDER_ANALYSIS", 75.00, "PIX-104", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("evento autenticado enfileirado deve ter sucesso: %s", result.Error)
		}
		if result.Outcome != "queued" || result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("resultado inesperado para evento enfileirado: %+v", result)
		}
	})

	t.Run("PAID with type PIX does not credit", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		var appliedStatus string
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			appliedStatus = status
			return &domain.Transaction{ID: requestNumber, UserID: "usr-paid", Status: domain.TxStatusPendente}, "queued", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(49, "PIX", "PAID", 100.00, "PIX-108", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("webhook autenticado deve ser aceito e enfileirado: %s", result.Error)
		}
		if appliedStatus != "PAID" {
			t.Fatalf("status passado ao applier não pode ser normalizado para PAID_OUT! Obteve: %s", appliedStatus)
		}
		if result.Outcome != "queued" || result.Kind != "deposit" {
			t.Fatalf("esperado outcome queued sem concessão de crédito, obteve: %+v", result)
		}
	})

	t.Run("Unexpected type with PAID_OUT does not credit", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		var appliedStatus string
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			appliedStatus = status
			return &domain.Transaction{ID: requestNumber, UserID: "usr-unk", Status: domain.TxStatusPendente}, "queued", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(50, "BOLETO", "PAID_OUT", 100.00, "PIX-109", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("webhook autenticado de tipo inesperado deve ser aceito e enfileirado: %s", result.Error)
		}
		if appliedStatus == "PAID_OUT" {
			t.Fatalf("PAID_OUT NUNCA deve ser passado ao applier para tipo não-PIX! Obteve: %s", appliedStatus)
		}
		if !strings.HasPrefix(appliedStatus, "UNSUPPORTED_TYPE:") {
			t.Fatalf("esperado status sintético UNSUPPORTED_TYPE:BOLETO, obteve: %s", appliedStatus)
		}
		if result.Outcome != "queued" {
			t.Fatalf("esperado outcome queued, obteve: %+v", result)
		}
	})

	t.Run("Missing requestNumber with externalID arrives at applier", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		var passedReq, passedExt string
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			passedReq = requestNumber
			passedExt = externalID
			return &domain.Transaction{ID: "PIX-FOUND", ExternalID: &externalID, UserID: "usr-ext", Status: domain.TxStatusSucesso}, "applied", nil
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(51, "PIX", "PAID_OUT", 100.00, "", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("callback com externalID deve ser aceito pelo applier: %s", result.Error)
		}
		if passedReq != "" || passedExt != "51" {
			t.Fatalf("parâmetros incorretos passados ao applier: req=%q ext=%q", passedReq, passedExt)
		}
		if result.Outcome != "applied" || result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("esperado 200 OK applied, obteve: %+v", result)
		}
	})

	t.Run("Both requestNumber and externalID missing rejects with 400", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		digest := sha256.Sum256([]byte(clientSecret + "PIXPAID_OUT10.00"))
		raw := []byte(fmt.Sprintf(`{"idTransaction":"","typeTransaction":"PIX","statusTransaction":"PAID_OUT","value":10.00,"requestNumber":"","hash":"%s"}`, hex.EncodeToString(digest[:])))

		result := uc.ProcessSuitPayWebhook(context.Background(), raw)
		if result.Success {
			t.Fatal("deve rejeitar quando ambos identificadores forem ausentes")
		}
		if result.HTTPStatusCode != http.StatusBadRequest || result.ErrorCode != "MISSING_IDENTIFIER" {
			t.Fatalf("esperado erro 400 MISSING_IDENTIFIER, obteve: %+v", result)
		}
	})

	t.Run("Persistence failure returns HTTP 500 and never confirms", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return nil, "", errors.New("db connection failure")
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(46, "PIX", "PAID_OUT", 100.00, "PIX-105", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if result.Success {
			t.Fatal("webhook NUNCA deve ser confirmado se a persistência falhou")
		}
		if result.ErrorCode != "PERSISTENCE_FAILED" || result.HTTPStatusCode != http.StatusInternalServerError {
			t.Fatalf("esperado erro 500 PERSISTENCE_FAILED, obteve: %+v", result)
		}
	})

	t.Run("Invalid signature returns HTTP 401", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		// tampered payload
		raw := makeSignedSuitPayWebhook(47, "PIX", "PAID_OUT", 100.00, "PIX-106", "wrong-secret")
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if result.Success {
			t.Fatal("webhook com assinatura inválida deve falhar")
		}
		if result.ErrorCode != "INVALID_SIGNATURE" || result.HTTPStatusCode != http.StatusUnauthorized {
			t.Fatalf("esperado erro 401 INVALID_SIGNATURE, obteve: %+v", result)
		}
	})

	t.Run("Fail closed in all environments if DepositApplier is absent", func(t *testing.T) {
		// Plain mockTxRepo does not implement DepositEventApplier
		plainRepo := &mockTxRepo{txs: map[string]*domain.Transaction{}}

		for _, env := range []string{"development", "staging", "production"} {
			ucEnv := payment.NewPaymentUseCase(nil, plainRepo, nil, cfg, nil, "", payment.Options{
				Mode:        payment.ModeSuitPay,
				Environment: env,
			})

			raw := makeSignedSuitPayWebhook(48, "PIX", "PAID_OUT", 100.00, "PIX-107", clientSecret)
			res := ucEnv.ProcessSuitPayWebhook(context.Background(), raw)

			if res.Success {
				t.Fatalf("ambiente %s deve falhar fechado se DepositApplier estiver ausente", env)
			}
			if res.ErrorCode != "DEPOSIT_APPLIER_UNAVAILABLE" || res.HTTPStatusCode != http.StatusInternalServerError {
				t.Fatalf("esperado DEPOSIT_APPLIER_UNAVAILABLE em %s: %+v", env, res)
			}
		}
	})
}

func TestSuitPayCashIn_EarlyCallbackPreserved(t *testing.T) {
	// Scenario: early callback confirms transaction before AttachDepositQRCode or submission issue handling completes
	rawTxs := map[string]*domain.Transaction{
		"PIX-EARLY": {
			ID:     "PIX-EARLY",
			Status: domain.TxStatusSucesso, // already confirmed by early callback
			Valor:  100,
			Tipo:   "Depósito PIX",
		},
	}
	applierRepo := newMockApplierTxRepo(rawTxs)

	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: "secret",
		},
	}}

	uc := payment.NewPaymentUseCase(nil, applierRepo, nil, cfg, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})

	// Simulate subsequent issue marking on the same transaction
	// Helper should not overwrite TxStatusSucesso
	tx := rawTxs["PIX-EARLY"]
	if tx.Status != domain.TxStatusSucesso {
		t.Fatal("transação deveria estar como sucesso")
	}

	// Try marking submission issue on already successful transaction
	err := applierRepo.MarkDepositSubmissionIssue(context.Background(), "PIX-EARLY", "timeout_after_send", "")
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != domain.TxStatusSucesso {
		t.Fatalf("callback antecipado foi revertido: status atual %s", tx.Status)
	}

	_ = uc
}

func TestSuitPayCashIn_LocalExpirationDoesNotCancel(t *testing.T) {
	// Local expiration is an investigation flag for reconciliation, never an automatic cancellation.
	tx := &domain.Transaction{
		ID:        "PIX-OLD",
		Status:    domain.TxStatusPendente,
		CreatedAt: time.Now().Add(-2 * time.Hour), // 2 hours ago
		Valor:     100,
		Tipo:      "Depósito PIX",
	}
	repo := &mockTxRepo{txs: map[string]*domain.Transaction{"PIX-OLD": tx}}
	uc := payment.NewPaymentUseCase(nil, repo, nil, nil, nil, "", payment.Options{
		Mode:        payment.ModeSimulated,
		Environment: "development",
	})

	list, err := uc.GetUserTransactions(context.Background(), tx.UserID, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = list

	// Transaction must remain pending and NOT auto-cancelled
	if tx.Status == domain.TxStatusCancelado {
		t.Fatal("expiração local não pode cancelar automaticamente a transação")
	}
}

func TestSuitPayWebhook_ConfigErrorsAndKillSwitch(t *testing.T) {
	clientSecret := "test-secret-123"

	t.Run("DB error on GetConfig returns 503 Service Unavailable for retry", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		errCfgRepo := &mockConfigRepo{err: errors.New("db pool connection exhausted")}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, errCfgRepo, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(60, "PIX", "PAID_OUT", 100.00, "PIX-CFG-ERR", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if result.Success {
			t.Fatal("webhook deve falhar se GetConfig falhar")
		}
		if result.HTTPStatusCode != http.StatusServiceUnavailable {
			t.Fatalf("esperado status 503 Service Unavailable para retry do gateway, obteve: %d", result.HTTPStatusCode)
		}
		if result.ErrorCode != "CONFIG_UNAVAILABLE" {
			t.Fatalf("esperado errorCode CONFIG_UNAVAILABLE, obteve: %s", result.ErrorCode)
		}
	})

	t.Run("Missing client secret returns 500 Internal Server Error", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		noSecretCfg := &mockConfigRepo{cfg: &domain.SystemConfig{
			Gateway: domain.GatewayConfig{
				SuitPayEnabled:      true,
				SuitPayClientSecret: "", // empty secret
			},
		}}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, noSecretCfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(61, "PIX", "PAID_OUT", 100.00, "PIX-NO-SECRET", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if result.Success {
			t.Fatal("webhook deve falhar se segredo estiver ausente")
		}
		if result.HTTPStatusCode != http.StatusInternalServerError {
			t.Fatalf("esperado status 500 Internal Server Error quando secret ausente, obteve: %d", result.HTTPStatusCode)
		}
		if result.ErrorCode != "CONFIG_UNAVAILABLE" {
			t.Fatalf("esperado errorCode CONFIG_UNAVAILABLE, obteve: %s", result.ErrorCode)
		}
	})

	t.Run("Kill switch off (SuitPayEnabled: false) accepts and verifies valid callback", func(t *testing.T) {
		txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
		var appliedReq, appliedStatus string
		txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			appliedReq = requestNumber
			appliedStatus = status
			return &domain.Transaction{ID: requestNumber, UserID: "usr-ks", Status: domain.TxStatusSucesso}, "applied", nil
		}

		offCfg := &mockConfigRepo{cfg: &domain.SystemConfig{
			Gateway: domain.GatewayConfig{
				SuitPayEnabled:      false, // kill switch OFF for new deposits
				SuitPayClientSecret: clientSecret,
			},
		}}

		users := &mockUserRepo{users: map[string]*domain.User{"usr-ks": {ID: "usr-ks"}}}
		uc := payment.NewPaymentUseCase(users, txRepo, nil, offCfg, nil, "", payment.Options{
			Mode:         payment.ModeSuitPay,
			Environment:  "staging",
			IdentityRepo: &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725", LegalName: "Teste", Email: "teste@example.com"}},
		})

		// 1. New deposit creation must be BLOCKED
		_, err := uc.CreatePixDeposit(context.Background(), payment.PixDepositRequest{UserID: "usr-ks", Amount: 100})
		if err == nil || !strings.Contains(err.Error(), "não configurado para cobranças") {
			t.Fatalf("esperado bloqueio de novas cobranças quando SuitPayEnabled=false, obteve: %v", err)
		}

		// 2. Legitimate callback for already existing deposit intent must be ACCEPTED
		raw := makeSignedSuitPayWebhook(62, "PIX", "PAID_OUT", 100.00, "PIX-EXISTING", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success {
			t.Fatalf("callback de intenção existente deve ser aceito mesmo com kill switch off: %s", result.Error)
		}
		if result.HTTPStatusCode != http.StatusOK || result.Outcome != "applied" {
			t.Fatalf("esperado 200 OK applied, obteve: %+v", result)
		}
		if appliedReq != "PIX-EXISTING" || appliedStatus != "PAID_OUT" {
			t.Fatalf("parâmetros incorretos passados ao applier: req=%s status=%s", appliedReq, appliedStatus)
		}
	})
}

func TestSuitPayWebhook_LongUnsupportedType_CappedAt32Chars(t *testing.T) {
	clientSecret := "secret-len-test"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
	var receivedStatus string
	txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
		receivedStatus = status
		return &domain.Transaction{ID: requestNumber, UserID: "usr-len", Status: domain.TxStatusPendente}, "queued", nil
	}

	uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})

	longType := "VERY_LONG_PAYMENT_TYPE_TRANSACTION_THAT_EXCEEDS_THE_DATABASE_COLUMN_LIMIT_OF_32_CHARACTERS"
	raw := makeSignedSuitPayWebhook(63, longType, "PAID_OUT", 50.00, "PIX-LONG-TYPE", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("webhook autenticado com tipo longo deve ser aceito e enfileirado: %s", result.Error)
	}
	if len(receivedStatus) > 32 {
		t.Fatalf("status enviado ao applier excedeu 32 caracteres (violação de schema): len=%d (%q)", len(receivedStatus), receivedStatus)
	}
	if !strings.HasPrefix(receivedStatus, "UNSUPPORTED_TYPE:") {
		t.Fatalf("status deve manter prefixo UNSUPPORTED_TYPE:, obteve: %q", receivedStatus)
	}
}

func TestSuitPayWebhook_PersistenceFailure_HidesInternalDatabaseError(t *testing.T) {
	clientSecret := "secret-sec-test"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
	txRepo.applyFunc = func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
		return nil, "", errors.New("pq: deadlocked relation transactions_pkey (SQLSTATE 40P01) table locked")
	}

	uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})

	raw := makeSignedSuitPayWebhook(64, "PIX", "PAID_OUT", 100.00, "PIX-DB-LEAK", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if result.Success {
		t.Fatal("webhook deve falhar se persistência falhar")
	}
	if result.HTTPStatusCode != http.StatusInternalServerError {
		t.Fatalf("esperado status 500, obteve: %d", result.HTTPStatusCode)
	}
	if result.ErrorCode != "PERSISTENCE_FAILED" {
		t.Fatalf("esperado errorCode PERSISTENCE_FAILED, obteve: %s", result.ErrorCode)
	}
	// Verify generic user/gateway message: MUST NOT expose internal DB / SQL details
	if strings.Contains(result.Error, "deadlocked") || strings.Contains(result.Error, "SQLSTATE") || strings.Contains(result.Error, "transactions_pkey") {
		t.Fatalf("ERRO VAZOU DETALHES INTERNOS DO BANCO: %s", result.Error)
	}
	if result.Error != "falha temporária ao persistir evento de pagamento" {
		t.Fatalf("mensagem inesperada: %s", result.Error)
	}
}

func newSuitPayWebhookUsecase(t *testing.T, clientSecret string, applyFunc func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error)) *payment.PaymentUseCase {
	t.Helper()
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}
	txRepo := newMockApplierTxRepo(map[string]*domain.Transaction{})
	txRepo.applyFunc = applyFunc
	return payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
		Mode:        payment.ModeSuitPay,
		Environment: "staging",
	})
}

func TestSuitPayWebhook_EarlyReversalPAIDOut_ClassifiedAsChargeback(t *testing.T) {
	// The repository applied PAID_OUT and immediately reversed an earlier
	// CHARGEBACK: outcome "applied" but transaction ends "cancelado".
	// This must be classified as chargeback/reversed, never as successful deposit.
	const clientSecret = "secret-early-reversal"
	uc := newSuitPayWebhookUsecase(t, clientSecret, func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
		if status != "PAID_OUT" {
			t.Fatalf("status inesperado enviado ao applier: %s", status)
		}
		return &domain.Transaction{ID: requestNumber, UserID: "usr-rev", Status: domain.TxStatusCancelado}, "applied", nil
	})

	raw := makeSignedSuitPayWebhook(70, "PIX", "PAID_OUT", 90.00, "PIX-EARLY-REV", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("transição durável deve ter sucesso: %s", result.Error)
	}
	if result.Outcome != "applied" {
		t.Fatalf("esperado outcome applied do repositório, obteve: %s", result.Outcome)
	}
	if result.Kind != "chargeback" {
		t.Fatalf("PAID_OUT revertido deve ser classificado como chargeback, obteve kind: %s", result.Kind)
	}
	if result.Status != string(domain.TxStatusCancelado) {
		t.Fatalf("status cancelado deve ser preservado no resultado, obteve: %s", result.Status)
	}
	if result.HTTPStatusCode != http.StatusOK {
		t.Fatalf("resultado durável deve manter HTTP 200, obteve: %d", result.HTTPStatusCode)
	}
	if result.UserID != "usr-rev" || result.TxID != "PIX-EARLY-REV" {
		t.Fatalf("resultado com identificadores incorretos: %+v", result)
	}
	if strings.Contains(strings.ToLower(result.Message), "confirmado com sucesso") {
		t.Fatalf("mensagem não pode tratar estorno como depósito confirmado: %s", result.Message)
	}
}

func TestSuitPayWebhook_DuplicateNeverSignalsNewCredit(t *testing.T) {
	const clientSecret = "secret-duplicate"

	t.Run("duplicate of successful deposit keeps deposit kind but duplicate outcome", func(t *testing.T) {
		uc := newSuitPayWebhookUsecase(t, clientSecret, func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-dup", Status: domain.TxStatusSucesso}, "duplicate", nil
		})

		raw := makeSignedSuitPayWebhook(71, "PIX", "PAID_OUT", 60.00, "PIX-DUP-1", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success || result.Outcome != "duplicate" {
			t.Fatalf("duplicata deve ser durável com outcome duplicate: %+v", result)
		}
		if result.HTTPStatusCode != http.StatusOK {
			t.Fatalf("duplicata autenticada deve manter HTTP 200, obteve: %d", result.HTTPStatusCode)
		}
		if result.Kind != "deposit" {
			t.Fatalf("duplicata de depósito liquidado deve manter kind deposit, obteve: %s", result.Kind)
		}
	})

	t.Run("duplicate of reversed chargeback is classified as chargeback", func(t *testing.T) {
		uc := newSuitPayWebhookUsecase(t, clientSecret, func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{ID: requestNumber, UserID: "usr-dup-cb", Status: domain.TxStatusCancelado}, "duplicate", nil
		})

		raw := makeSignedSuitPayWebhook(72, "PIX", "CHARGEBACK", 60.00, "PIX-DUP-2", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success || result.Outcome != "duplicate" {
			t.Fatalf("duplicata de chargeback deve ter outcome duplicate: %+v", result)
		}
		if result.Kind != "chargeback" || result.Status != string(domain.TxStatusCancelado) {
			t.Fatalf("duplicata de chargeback deve manter classificação de chargeback: %+v", result)
		}
	})
}

func TestSuitPayWebhook_LargeNumericID_PassesExactExternalID(t *testing.T) {
	// 2^53+1 cannot survive a float64 round-trip; the applier must receive the
	// exact decimal digits of the gateway identifier.
	const clientSecret = "secret-large-id"
	const largeID int64 = 9007199254740993

	var gotExternal string
	uc := newSuitPayWebhookUsecase(t, clientSecret, func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
		gotExternal = externalID
		return &domain.Transaction{ID: requestNumber, UserID: "usr-big", Status: domain.TxStatusSucesso}, "applied", nil
	})

	raw := makeSignedSuitPayWebhook(largeID, "PIX", "PAID_OUT", 42.00, "PIX-BIG-ID", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("callback com id grande deve ser aceito: %s", result.Error)
	}
	if gotExternal != "9007199254740993" {
		t.Fatalf("external ID numérico grande perdeu precisão: esperado 9007199254740993, obteve %q", gotExternal)
	}
}

func TestSuitPayWebhook_StringID_PassesExactExternalID(t *testing.T) {
	const clientSecret = "secret-string-id"

	var gotExternal string
	uc := newSuitPayWebhookUsecase(t, clientSecret, func(ctx context.Context, requestNumber, externalID, status string, amount float64) (*domain.Transaction, string, error) {
		gotExternal = externalID
		return &domain.Transaction{ID: requestNumber, UserID: "usr-str", Status: domain.TxStatusCancelado}, "applied", nil
	})

	raw := makeSignedSuitPayWebhookWithStringID("prov-str-9007199254740993", "PIX", "CHARGEBACK", 42.00, "PIX-STR-ID", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("callback com id string deve ser aceito: %s", result.Error)
	}
	if gotExternal != "prov-str-9007199254740993" {
		t.Fatalf("external ID string alterado: obteve %q", gotExternal)
	}
	if result.Kind != "chargeback" {
		t.Fatalf("chargeback aplicado deve ter kind chargeback, obteve: %s", result.Kind)
	}
}

// ----------------------------------------------------------------------------
// P0.3 Cash-Out Tests
// ----------------------------------------------------------------------------

type mockIdentityRepoErr struct {
	err error
}

func (m *mockIdentityRepoErr) CreateUserWithIdentity(context.Context, *domain.User, domain.IdentityPerson) error {
	return m.err
}
func (m *mockIdentityRepoErr) Submit(context.Context, string, domain.IdentityPerson) error {
	return m.err
}
func (m *mockIdentityRepoErr) Status(context.Context, string) (string, error) {
	return "", m.err
}
func (m *mockIdentityRepoErr) VerifiedPerson(context.Context, string) (domain.IdentityPerson, error) {
	return domain.IdentityPerson{}, m.err
}
func (m *mockIdentityRepoErr) Decide(context.Context, string, string, string, string) error {
	return m.err
}
func (m *mockIdentityRepoErr) Pending(context.Context) ([]string, error) {
	return nil, m.err
}
func (m *mockIdentityRepoErr) Inspect(context.Context, string, string) (domain.IdentityPerson, error) {
	return domain.IdentityPerson{}, m.err
}

type mockFinancialRepo struct {
	reservePayoutFunc         func(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error)
	claimPayoutFunc           func(ctx context.Context, id string) (*domain.Transaction, error)
	recordPayoutSubFunc       func(ctx context.Context, id, ext string) (*domain.Transaction, error)
	confirmPayoutFunc         func(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error)
	cancelPayoutFunc          func(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error)
	applyPayoutCallbackFunc   func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, error)
	applyPayoutOutcomeFunc    func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error)
	getPayoutByExternalIDFunc func(ctx context.Context, ext string) (*domain.Transaction, error)
	confirmDepositFunc        func(ctx context.Context, id string, amount float64, ext, method string) (*domain.Transaction, float64, error)
}

func (m *mockFinancialRepo) ConfirmDeposit(ctx context.Context, id string, amount float64, ext, method string) (*domain.Transaction, float64, error) {
	if m.confirmDepositFunc != nil {
		return m.confirmDepositFunc(ctx, id, amount, ext, method)
	}
	return nil, 0, nil
}

func (m *mockFinancialRepo) ReservePayout(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
	if m.reservePayoutFunc != nil {
		return m.reservePayoutFunc(ctx, tx, validate)
	}
	net := 95.0
	fee := 5.0
	tx.NetAmount = &net
	tx.FeeAmount = &fee
	tx.Status = domain.TxStatusPendente
	return tx, 900.0, false, nil
}

func (m *mockFinancialRepo) ClaimPayout(ctx context.Context, id string) (*domain.Transaction, error) {
	if m.claimPayoutFunc != nil {
		return m.claimPayoutFunc(ctx, id)
	}
	net := 95.0
	fee := 5.0
	key := "test@pix.com"
	return &domain.Transaction{
		ID:        id,
		UserID:    "usr-payout",
		Tipo:      "Saque PIX",
		Valor:     -100.0,
		NetAmount: &net,
		FeeAmount: &fee,
		PixKey:    &key,
		Status:    domain.TxStatusProcessando,
	}, nil
}

func (m *mockFinancialRepo) RecordPayoutSubmission(ctx context.Context, id, ext string) (*domain.Transaction, error) {
	if m.recordPayoutSubFunc != nil {
		return m.recordPayoutSubFunc(ctx, id, ext)
	}
	net := 95.0
	return &domain.Transaction{ID: id, Status: domain.TxStatusProcessando, NetAmount: &net, ExternalID: &ext}, nil
}

func (m *mockFinancialRepo) ConfirmPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	if m.confirmPayoutFunc != nil {
		return m.confirmPayoutFunc(ctx, id, ext, net)
	}
	return &domain.Transaction{ID: id, Status: domain.TxStatusSucesso}, nil
}

func (m *mockFinancialRepo) CancelPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	if m.cancelPayoutFunc != nil {
		return m.cancelPayoutFunc(ctx, id, ext, net)
	}
	return &domain.Transaction{ID: id, Status: domain.TxStatusCancelado}, nil
}

func (m *mockFinancialRepo) ApplyPayoutCallback(ctx context.Context, ext, status string, val float64) (*domain.Transaction, error) {
	if m.applyPayoutCallbackFunc != nil {
		return m.applyPayoutCallbackFunc(ctx, ext, status, val)
	}
	return nil, nil
}

func (m *mockFinancialRepo) ApplyPayoutCallbackWithOutcome(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
	if m.applyPayoutOutcomeFunc != nil {
		return m.applyPayoutOutcomeFunc(ctx, ext, status, val)
	}
	if m.applyPayoutCallbackFunc != nil {
		tx, err := m.applyPayoutCallbackFunc(ctx, ext, status, val)
		if err != nil {
			return nil, "", err
		}
		if tx == nil {
			return nil, "queued", nil
		}
		return tx, "applied", nil
	}
	return nil, "queued", nil
}

// legacyFinancialRepoOnly implements FinancialRepository WITHOUT the
// outcome-aware callback, reproducing repositories that predate P0.3.
type legacyFinancialRepoOnly struct{}

func (legacyFinancialRepoOnly) ConfirmDeposit(ctx context.Context, id string, amount float64, ext, method string) (*domain.Transaction, float64, error) {
	return nil, 0, nil
}
func (legacyFinancialRepoOnly) ReservePayout(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
	return tx, 0, false, nil
}
func (legacyFinancialRepoOnly) ClaimPayout(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) RecordPayoutSubmission(ctx context.Context, id, ext string) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) ConfirmPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) CancelPayout(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) ApplyPayoutCallback(ctx context.Context, ext, status string, val float64) (*domain.Transaction, error) {
	return nil, nil
}
func (legacyFinancialRepoOnly) GetPayoutByExternalID(ctx context.Context, ext string) (*domain.Transaction, error) {
	return nil, nil
}

func (m *mockFinancialRepo) GetPayoutByExternalID(ctx context.Context, ext string) (*domain.Transaction, error) {
	if m.getPayoutByExternalIDFunc != nil {
		return m.getPayoutByExternalIDFunc(ctx, ext)
	}
	return nil, nil
}

func TestSuitPayPayout_RequiresVerifiedIdentityBeforeReservation(t *testing.T) {
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	hashedSec, _ := crypto.HashPassword("123456")
	users := &mockUserRepo{users: map[string]*domain.User{
		"usr-1": {
			ID:               "usr-1",
			Balance:          1000.0,
			SecurityPassword: hashedSec,
			CurrentRollover:  1000.0,
			RequiredRollover: 100.0,
		},
	}}

	t.Run("fails closed when IdentityRepo is nil", func(t *testing.T) {
		reserveCalled := false
		finRepo := &mockFinancialRepo{
			reservePayoutFunc: func(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
				reserveCalled = true
				return tx, 900.0, false, nil
			},
		}

		uc := payment.NewPaymentUseCase(users, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        nil,
		})

		_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
			UserID:           "usr-1",
			Amount:           100.0,
			PixKey:           "test@pix.com",
			IdempotencyKey:   "idem-1",
			SecurityPassword: "123456",
		})
		if err == nil || !strings.Contains(err.Error(), "serviço de verificação de identidade indisponível") {
			t.Fatalf("esperado erro de identidade indisponível, obteve: %v", err)
		}
		if reserveCalled {
			t.Fatal("ReservePayout NUNCA deve ser chamado quando identidade estiver indisponível")
		}
	})

	t.Run("fails closed when IdentityRepo.VerifiedPerson returns error", func(t *testing.T) {
		reserveCalled := false
		finRepo := &mockFinancialRepo{
			reservePayoutFunc: func(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
				reserveCalled = true
				return tx, 900.0, false, nil
			},
		}

		uc := payment.NewPaymentUseCase(users, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        &mockIdentityRepoErr{err: errors.New("person not found or pending verification")},
		})

		_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
			UserID:           "usr-1",
			Amount:           100.0,
			PixKey:           "test@pix.com",
			IdempotencyKey:   "idem-2",
			SecurityPassword: "123456",
		})
		if err == nil || !strings.Contains(err.Error(), "identidade não verificada para saque") {
			t.Fatalf("esperado erro de identidade não verificada, obteve: %v", err)
		}
		if reserveCalled {
			t.Fatal("ReservePayout NUNCA deve ser chamado quando pessoa não estiver verificada")
		}
	})

	t.Run("fails closed when person has invalid CPF check digits", func(t *testing.T) {
		reserveCalled := false
		finRepo := &mockFinancialRepo{
			reservePayoutFunc: func(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
				reserveCalled = true
				return tx, 900.0, false, nil
			},
		}

		// 11111111111 has all equal digits, rejected by NormalizeCPF
		uc := payment.NewPaymentUseCase(users, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: "11111111111"}},
		})

		_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
			UserID:           "usr-1",
			Amount:           100.0,
			PixKey:           "test@pix.com",
			IdempotencyKey:   "idem-3",
			SecurityPassword: "123456",
		})
		if err == nil || !strings.Contains(err.Error(), "documento de identidade inválido para saque") {
			t.Fatalf("esperado erro de documento de identidade inválido, obteve: %v", err)
		}
		if reserveCalled {
			t.Fatal("ReservePayout NUNCA deve ser chamado com CPF inválido")
		}
	})

	t.Run("never leaks CPF into error messages or security logs", func(t *testing.T) {
		rawCPF := "52998224725"
		finRepo := &mockFinancialRepo{
			reservePayoutFunc: func(ctx context.Context, tx *domain.Transaction, validate func(*domain.User) error) (*domain.Transaction, float64, bool, error) {
				if validate != nil {
					u := users.users["usr-1"]
					if err := validate(u); err != nil {
						return nil, 0, false, err
					}
				}
				return tx, 900.0, false, nil
			},
		}
		uc := payment.NewPaymentUseCase(users, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: rawCPF}},
		})

		// Test with invalid security password to produce an error
		_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
			UserID:           "usr-1",
			Amount:           100.0,
			PixKey:           "test@pix.com",
			IdempotencyKey:   "idem-4",
			SecurityPassword: "wrong-password",
		})
		if err == nil {
			t.Fatal("esperado erro de senha inválida")
		}
		if strings.Contains(err.Error(), rawCPF) {
			t.Fatalf("CPF NUNCA PODE VAZAR EM ERROS: %v", err)
		}
	})
}

func TestSuitPayPayout_ManualApprovalVerifiesIdentityBeforeClaim(t *testing.T) {
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	pendingTx := &domain.Transaction{
		ID:              "WDR-PENDING-1",
		UserID:          "usr-man-1",
		Tipo:            "Saque PIX",
		Valor:           -100.0,
		GatewayProvider: string(payment.ModeSuitPay),
		Status:          domain.TxStatusPendente,
	}

	t.Run("fails closed and does not claim payout if IdentityRepo is nil", func(t *testing.T) {
		txRepo := &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-PENDING-1": pendingTx}}
		claimed := false
		finRepo := &mockFinancialRepo{
			claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
				claimed = true
				return pendingTx, nil
			},
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        nil,
		})

		_, err := uc.ApprovePayout(context.Background(), "WDR-PENDING-1")
		if err == nil || !strings.Contains(err.Error(), "serviço de verificação de identidade indisponível") {
			t.Fatalf("esperado erro de identidade indisponível, obteve: %v", err)
		}
		if claimed {
			t.Fatal("ClaimPayout NUNCA deve ser chamado quando IdentityRepo for nil")
		}
	})

	t.Run("fails closed and does not claim payout if person is unverified", func(t *testing.T) {
		txRepo := &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-PENDING-1": pendingTx}}
		claimed := false
		finRepo := &mockFinancialRepo{
			claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
				claimed = true
				return pendingTx, nil
			},
		}

		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        &mockIdentityRepoErr{err: errors.New("kyc rejected")},
		})

		_, err := uc.ApprovePayout(context.Background(), "WDR-PENDING-1")
		if err == nil || !strings.Contains(err.Error(), "identidade não verificada para envio de saque") {
			t.Fatalf("esperado erro de identidade não verificada, obteve: %v", err)
		}
		if claimed {
			t.Fatal("ClaimPayout NUNCA deve ser chamado quando KYC rejeitado")
		}
	})
}

func TestSuitPayPayout_DispatchSendsDocumentValidationAndExternalID(t *testing.T) {
	var receivedReq struct {
		Value              float64 `json:"value"`
		Key                string  `json:"key"`
		TypeKey            string  `json:"typeKey"`
		DocumentValidation string  `json:"documentValidation"`
		ExternalID         string  `json:"externalId"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pix-payment" {
			_ = json.NewDecoder(r.Body).Decode(&receivedReq)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response":"OK","idTransaction":"STG-TRANSFER-999"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	net := 95.0
	fee := 5.0
	pixKey := "destinatario@pix.com"
	pendingTx := &domain.Transaction{
		ID:              "WDR-DISPATCH-1",
		UserID:          "usr-disp-1",
		Tipo:            "Saque PIX",
		Valor:           -100.0,
		NetAmount:       &net,
		FeeAmount:       &fee,
		PixKey:          &pixKey,
		GatewayProvider: string(payment.ModeSuitPay),
		Status:          domain.TxStatusPendente,
	}

	recordedID := ""
	recordedExt := ""
	finRepo := &mockFinancialRepo{
		claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			copyTx := *pendingTx
			copyTx.Status = domain.TxStatusProcessando
			return &copyTx, nil
		},
		recordPayoutSubFunc: func(ctx context.Context, id, ext string) (*domain.Transaction, error) {
			recordedID = id
			recordedExt = ext
			copyTx := *pendingTx
			copyTx.ExternalID = &ext
			copyTx.Status = domain.TxStatusProcessando
			return &copyTx, nil
		},
	}

	rawCPF := "529.982.247-25"
	expectedNormalizedCPF := "52998224725"

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-DISPATCH-1": pendingTx}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		PixSandboxURL:       server.URL,
		FinancialRepository: finRepo,
		IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: rawCPF}},
	})

	res, err := uc.ApprovePayout(context.Background(), "WDR-DISPATCH-1")
	if err != nil {
		t.Fatalf("esperado sucesso na aprovação e dispatch: %v", err)
	}
	if res == nil {
		t.Fatal("esperada transação retornada")
	}

	if receivedReq.DocumentValidation != expectedNormalizedCPF {
		t.Fatalf("documentValidation incorreto: esperado %q, obteve %q", expectedNormalizedCPF, receivedReq.DocumentValidation)
	}
	if receivedReq.ExternalID != "WDR-DISPATCH-1" {
		t.Fatalf("externalId incorreto: esperado %q, obteve %q", "WDR-DISPATCH-1", receivedReq.ExternalID)
	}
	if receivedReq.Value != 95.0 {
		t.Fatalf("valor incorreto: esperado 95.0, obteve %v", receivedReq.Value)
	}
	if receivedReq.Key != pixKey {
		t.Fatalf("chave incorreta: esperado %q, obteve %q", pixKey, receivedReq.Key)
	}
	if recordedID != "WDR-DISPATCH-1" || recordedExt != "STG-TRANSFER-999" {
		t.Fatalf("RecordPayoutSubmission com parâmetros incorretos: id=%q ext=%q", recordedID, recordedExt)
	}
}

func TestSuitPayPayout_AmbiguousGatewayErrorDoesNotCancelOrResend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"response":"ERROR","message":"internal gateway error"}`))
	}))
	defer server.Close()

	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	net := 95.0
	fee := 5.0
	pixKey := "dest@pix.com"
	pendingTx := &domain.Transaction{
		ID:              "WDR-AMBIG-1",
		UserID:          "usr-ambig-1",
		Tipo:            "Saque PIX",
		Valor:           -100.0,
		NetAmount:       &net,
		FeeAmount:       &fee,
		PixKey:          &pixKey,
		GatewayProvider: string(payment.ModeSuitPay),
		Status:          domain.TxStatusPendente,
	}

	cancelled := false
	finRepo := &mockFinancialRepo{
		claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			copyTx := *pendingTx
			copyTx.Status = domain.TxStatusProcessando
			return &copyTx, nil
		},
		cancelPayoutFunc: func(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
			cancelled = true
			return nil, nil
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-AMBIG-1": pendingTx}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		PixSandboxURL:       server.URL,
		FinancialRepository: finRepo,
		IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725"}},
	})

	res, err := uc.ApprovePayout(context.Background(), "WDR-AMBIG-1")
	if err != nil {
		t.Fatalf("erro inesperado: erro ambíguo de gateway deve manter reserva sem estourar erro não tratado: %v", err)
	}
	if res.Status != domain.TxStatusProcessando {
		t.Fatalf("status deve permanecer processando para conciliação do operador, obteve: %s", res.Status)
	}
	if cancelled {
		t.Fatal("CancelPayout NUNCA deve ser chamado automaticamente em erro ambíguo de gateway")
	}
}

func TestSuitPayPayout_DocumentValidateHTTPResponseKeepsUncertainProcessingState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":"DOCUMENT_VALIDATE","message":"Chave PIX não pertence ao documento informado"}`))
	}))
	defer server.Close()

	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	net := 95.0
	fee := 5.0
	pixKey := "dest@pix.com"
	pendingTx := &domain.Transaction{
		ID:              "WDR-DOCVAL-1",
		UserID:          "usr-docval-1",
		Tipo:            "Saque PIX",
		Valor:           -100.0,
		NetAmount:       &net,
		FeeAmount:       &fee,
		PixKey:          &pixKey,
		GatewayProvider: string(payment.ModeSuitPay),
		Status:          domain.TxStatusPendente,
	}

	cancelled := false
	recorded := false
	finRepo := &mockFinancialRepo{
		claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			copyTx := *pendingTx
			copyTx.Status = domain.TxStatusProcessando
			return &copyTx, nil
		},
		cancelPayoutFunc: func(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
			cancelled = true
			return nil, nil
		},
		recordPayoutSubFunc: func(ctx context.Context, id, ext string) (*domain.Transaction, error) {
			recorded = true
			return nil, nil
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-DOCVAL-1": pendingTx}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		PixSandboxURL:       server.URL,
		FinancialRepository: finRepo,
		IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725"}},
	})

	res, err := uc.ApprovePayout(context.Background(), "WDR-DOCVAL-1")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res.Status != domain.TxStatusProcessando {
		t.Fatalf("DOCUMENT_VALIDATE na resposta HTTP deve manter estado processando para conciliação, obteve: %s", res.Status)
	}
	if cancelled {
		t.Fatal("CancelPayout NUNCA deve ser chamado automaticamente em resposta DOCUMENT_VALIDATE sem evidência oficial transacional")
	}
	if recorded {
		t.Fatal("RecordPayoutSubmission não deve ser chamado quando a transferência não retornou OK")
	}
}

func TestSuitPayPayout_ManualApprovalValidatesGatewayProviderMatchesMode(t *testing.T) {
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci-test",
			SuitPayClientSecret: "cs-test",
			SuitPaySandbox:      true,
		},
	}}

	claimed := false
	finRepo := &mockFinancialRepo{
		claimPayoutFunc: func(ctx context.Context, id string) (*domain.Transaction, error) {
			claimed = true
			return nil, nil
		},
	}

	t.Run("blocks when withdrawal GatewayProvider is simulated but usecase mode is SuitPay", func(t *testing.T) {
		claimed = false
		tx := &domain.Transaction{
			ID:              "WDR-MISMATCH-1",
			UserID:          "usr-1",
			Tipo:            "Saque PIX",
			Valor:           -100.0,
			GatewayProvider: "simulated",
			Status:          domain.TxStatusPendente,
		}

		uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-MISMATCH-1": tx}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
			IdentityRepo:        &mockIdentityRepo{person: domain.IdentityPerson{CPF: "52998224725"}},
		})

		_, err := uc.ApprovePayout(context.Background(), "WDR-MISMATCH-1")
		if err == nil || !strings.Contains(err.Error(), "incompatível com modo") {
			t.Fatalf("esperado erro de provedor incompatível, obteve: %v", err)
		}
		if claimed {
			t.Fatal("ClaimPayout NUNCA deve ser chamado quando GatewayProvider não corresponder ao Mode")
		}
	})

	t.Run("blocks simulated mode approval in production", func(t *testing.T) {
		claimed = false
		tx := &domain.Transaction{
			ID:              "WDR-SIM-PROD",
			UserID:          "usr-1",
			Tipo:            "Saque PIX",
			Valor:           -100.0,
			GatewayProvider: "simulated",
			Status:          domain.TxStatusPendente,
		}

		uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{"WDR-SIM-PROD": tx}}, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSimulated,
			Environment:         "production",
			FinancialRepository: finRepo,
		})

		_, err := uc.ApprovePayout(context.Background(), "WDR-SIM-PROD")
		if err == nil || !strings.Contains(err.Error(), "indisponíveis em produção") {
			t.Fatalf("esperado bloqueio de modo simulado em produção, obteve: %v", err)
		}
		if claimed {
			t.Fatal("ClaimPayout NUNCA deve ser chamado para simulação em produção")
		}
	})
}

func TestSuitPayPayout_UnknownAuthenticatedStatusIsQueuedDurablyWithoutRefund(t *testing.T) {
	clientSecret := "secret-payout-unk"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	var gotExt, gotStatus string
	var gotAmount float64
	cancelCalled := false
	finRepo := &mockFinancialRepo{
		applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			gotExt, gotStatus, gotAmount = ext, status, val
			return nil, "queued", nil
		},
		cancelPayoutFunc: func(ctx context.Context, id, ext string, net float64) (*domain.Transaction, error) {
			cancelCalled = true
			return nil, nil
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: finRepo,
	})

	// DOCUMENT_VALIDATE is an HTTP transfer-response code, not a webhook status,
	// but an authenticated callback of any status must reach the durable queue.
	raw := makeSignedSuitPayWebhook(333, "PIX_CASHOUT", "DOCUMENT_VALIDATE", 100.00, "", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("status autenticado desconhecido deve ser enfileirado, não rejeitado: %+v", result)
	}
	if result.HTTPStatusCode != http.StatusOK {
		t.Fatalf("esperado status 200 para evento enfileirado, obteve: %d", result.HTTPStatusCode)
	}
	if result.Outcome != "queued" || result.Kind != "payout" {
		t.Fatalf("esperado outcome queued com kind payout, obteve: %+v", result)
	}
	if gotExt != "333" || gotStatus != "DOCUMENT_VALIDATE" || gotAmount != 100.00 {
		t.Fatalf("repositório deve receber o evento autenticado exato, obteve ext=%q status=%q amount=%.2f", gotExt, gotStatus, gotAmount)
	}
	if cancelCalled {
		t.Fatal("CancelPayout NUNCA deve ser chamado para status autenticado desconhecido")
	}
}

func TestSuitPayPayout_LegacyNonTransactionalFailsClosedForSuitPay(t *testing.T) {
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientID:     "ci",
			SuitPayClientSecret: "cs",
		},
	}}

	hashedSec, _ := crypto.HashPassword("123456")
	users := &mockUserRepo{users: map[string]*domain.User{
		"usr-legacy": {
			ID:               "usr-legacy",
			Balance:          1000.0,
			SecurityPassword: hashedSec,
			CurrentRollover:  1000.0,
			RequiredRollover: 100.0,
		},
	}}

	// FinancialRepository is nil -> legacy non-transactional path
	uc := payment.NewPaymentUseCase(users, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "development",
		FinancialRepository: nil,
	})

	_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
		UserID:           "usr-legacy",
		Amount:           100.0,
		PixKey:           "test@pix.com",
		IdempotencyKey:   "idem-leg",
		SecurityPassword: "123456",
	})
	if err == nil || !strings.Contains(err.Error(), "caminho legado não transacional não suporta modo SuitPay") {
		t.Fatalf("esperado fail-closed no caminho legado para modo SuitPay, obteve: %v", err)
	}
}

func TestSuitPayPayout_SimulatedModeInProductionFailsClosed(t *testing.T) {
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{}}
	finRepo := &mockFinancialRepo{}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSimulated,
		Environment:         "production",
		FinancialRepository: finRepo,
	})

	_, err := uc.RequestPayout(context.Background(), payment.PayoutRequest{
		UserID:           "usr-prod",
		Amount:           100.0,
		PixKey:           "test@pix.com",
		IdempotencyKey:   "idem-prod",
		SecurityPassword: "password",
	})
	if err == nil || !strings.Contains(err.Error(), "saques simulados indisponíveis em produção") {
		t.Fatalf("esperado erro de simulação indisponível em produção, obteve: %v", err)
	}
}

func TestSuitPayPayout_CallbackOrphanNeverEmitsWithdrawApproved(t *testing.T) {
	clientSecret := "secret-payout-orphan"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	// The repository reports a durable queued outcome (orphan, no matching payout in local DB)
	finRepo := &mockFinancialRepo{
		applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return nil, "queued", nil
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: finRepo,
	})

	raw := makeSignedSuitPayWebhook(777, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("callback órfão deve retornar 200 OK para conciliação, obteve erro: %s", result.Error)
	}
	if result.Outcome != "queued" || result.Status != "queued" {
		t.Fatalf("esperado resultado queued, obteve outcome=%s status=%s", result.Outcome, result.Status)
	}
}

func TestSuitPayPayout_PayoutCallbackReconcilesWithoutRealtimeWebsocketEvent(t *testing.T) {
	clientSecret := "secret-payout-apply"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	net := 100.0
	finRepo := &mockFinancialRepo{
		applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return &domain.Transaction{
				ID:        "WDR-APPLY-1",
				UserID:    "usr-payout-1",
				Status:    domain.TxStatusSucesso,
				NetAmount: &net,
			}, "applied", nil
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: finRepo,
	})

	raw := makeSignedSuitPayWebhook(555, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if !result.Success {
		t.Fatalf("callback de saque válido deve ter sucesso: %s", result.Error)
	}
	if result.Outcome != "applied" || result.Kind != "payout" {
		t.Fatalf("esperado outcome applied com kind payout, obteve: %+v", result)
	}
}

func TestSuitPayPayout_PersistenceFailureDoesNotLeakSQL(t *testing.T) {
	clientSecret := "secret-payout-leak"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	finRepo := &mockFinancialRepo{
		applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			return nil, "", errors.New("pq: deadlocked relation transactions_pkey (SQLSTATE 40P01) internal table locked")
		},
	}

	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: finRepo,
	})

	raw := makeSignedSuitPayWebhook(444, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if result.Success {
		t.Fatal("deve falhar quando persistência falhar")
	}
	if result.HTTPStatusCode != http.StatusInternalServerError {
		t.Fatalf("esperado status 500, obteve: %d", result.HTTPStatusCode)
	}
	if result.ErrorCode != "PERSISTENCE_FAILED" {
		t.Fatalf("esperado errorCode PERSISTENCE_FAILED, obteve: %s", result.ErrorCode)
	}
	if strings.Contains(result.Error, "deadlocked") || strings.Contains(result.Error, "SQLSTATE") || strings.Contains(result.Error, "transactions_pkey") {
		t.Fatalf("ERRO VAZOU DETALHES DE SQL/BANCO: %s", result.Error)
	}
	if result.Error != "falha temporária ao persistir callback de saque" {
		t.Fatalf("mensagem inesperada: %s", result.Error)
	}
}

func TestSuitPayPayout_CallbackOutcomesAreExact(t *testing.T) {
	clientSecret := "secret-payout-outcomes"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}
	net := 95.0

	cases := []struct {
		name        string
		outcome     string
		tx          *domain.Transaction
		wantOutcome string
		wantStatus  string
	}{
		{
			name:    "applied settlement",
			outcome: "applied",
			tx: &domain.Transaction{
				ID: "WDR-OUT-1", UserID: "usr-out-1", Status: domain.TxStatusSucesso, NetAmount: &net,
			},
			wantOutcome: "applied",
			wantStatus:  string(domain.TxStatusSucesso),
		},
		{
			name:    "applied cancellation refund",
			outcome: "applied",
			tx: &domain.Transaction{
				ID: "WDR-OUT-2", UserID: "usr-out-2", Status: domain.TxStatusCancelado, NetAmount: &net,
			},
			wantOutcome: "applied",
			wantStatus:  string(domain.TxStatusCancelado),
		},
		{
			name:    "duplicate replay",
			outcome: "duplicate",
			tx: &domain.Transaction{
				ID: "WDR-OUT-3", UserID: "usr-out-3", Status: domain.TxStatusSucesso, NetAmount: &net,
			},
			wantOutcome: "duplicate",
			wantStatus:  string(domain.TxStatusSucesso),
		},
		{
			name:        "queued orphan",
			outcome:     "queued",
			tx:          nil,
			wantOutcome: "queued",
			wantStatus:  "queued",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			finRepo := &mockFinancialRepo{
				applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
					return tc.tx, tc.outcome, nil
				},
			}
			uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
				Mode:                payment.ModeSuitPay,
				Environment:         "staging",
				FinancialRepository: finRepo,
			})

			statusTx := "PAID_OUT"
			if tc.wantStatus == string(domain.TxStatusCancelado) {
				statusTx = "CANCELED"
			}
			raw := makeSignedSuitPayWebhook(888, "PIX_CASHOUT", statusTx, 95.00, "", clientSecret)
			result := uc.ProcessSuitPayWebhook(context.Background(), raw)

			if !result.Success {
				t.Fatalf("callback deve ser aceito: %+v", result)
			}
			if result.Outcome != tc.wantOutcome {
				t.Fatalf("outcome exato esperado %q, obteve %q", tc.wantOutcome, result.Outcome)
			}
			if result.Status != tc.wantStatus {
				t.Fatalf("status exato esperado %q, obteve %q", tc.wantStatus, result.Status)
			}
			if result.Kind != "payout" {
				t.Fatalf("esperado kind payout, obteve %q", result.Kind)
			}
			if tc.tx != nil && (result.TxID != tc.tx.ID || result.UserID != tc.tx.UserID) {
				t.Fatalf("transação devolvida incorreta: %+v", result)
			}
		})
	}
}

func TestSuitPayPayout_CallbackWithoutOutcomeRepositoryFailsClosed(t *testing.T) {
	clientSecret := "secret-payout-noapplier"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	// FinancialRepository without ApplyPayoutCallbackWithOutcome: the webhook
	// must fail closed instead of guessing cash-out state.
	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: legacyFinancialRepoOnly{},
	})

	raw := makeSignedSuitPayWebhook(666, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
	result := uc.ProcessSuitPayWebhook(context.Background(), raw)

	if result.Success {
		t.Fatal("modo SuitPay sem applier de outcome deve falhar fechado")
	}
	if result.HTTPStatusCode != http.StatusInternalServerError {
		t.Fatalf("esperado 500, obteve: %d", result.HTTPStatusCode)
	}
	if result.ErrorCode != "PAYOUT_APPLIER_UNAVAILABLE" {
		t.Fatalf("esperado errorCode PAYOUT_APPLIER_UNAVAILABLE, obteve: %s", result.ErrorCode)
	}
	if result.Error == "" {
		t.Fatal("erro genérico esperado")
	}
	if result.Outcome != "" || result.TxID != "" || result.UserID != "" {
		t.Fatalf("resposta de falha não pode parecer conciliação: %+v", result)
	}
}

func TestSuitPayPayout_CallbackMissingIdentifierOrAmountIsRejected400(t *testing.T) {
	clientSecret := "secret-payout-badreq"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	repoCalled := false
	finRepo := &mockFinancialRepo{
		applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
			repoCalled = true
			return nil, "queued", nil
		},
	}
	uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
		Mode:                payment.ModeSuitPay,
		Environment:         "staging",
		FinancialRepository: finRepo,
	})

	t.Run("missing external id", func(t *testing.T) {
		repoCalled = false
		values := "PIX_CASHOUT" + "PAID_OUT" + "100.00"
		digest := sha256.Sum256([]byte(clientSecret + values))
		body := fmt.Sprintf(`{"idTransaction":"","typeTransaction":"PIX_CASHOUT","statusTransaction":"PAID_OUT","value":100.00,"requestNumber":"","hash":"%s"}`, hex.EncodeToString(digest[:]))
		result := uc.ProcessSuitPayWebhook(context.Background(), []byte(body))
		if result.Success || result.HTTPStatusCode != http.StatusBadRequest || result.ErrorCode != "INCOMPLETE_PAYOUT_CALLBACK" {
			t.Fatalf("esperado 400 INCOMPLETE_PAYOUT_CALLBACK sem identificador, obteve: %+v", result)
		}
		if repoCalled {
			t.Fatal("repositório não deve ser chamado sem identificador")
		}
	})

	t.Run("invalid amount", func(t *testing.T) {
		repoCalled = false
		raw := makeSignedSuitPayWebhook(999, "PIX_CASHOUT", "PAID_OUT", 0, "", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)
		if result.Success || result.HTTPStatusCode != http.StatusBadRequest || result.ErrorCode != "INCOMPLETE_PAYOUT_CALLBACK" {
			t.Fatalf("esperado 400 INCOMPLETE_PAYOUT_CALLBACK com valor inválido, obteve: %+v", result)
		}
		if repoCalled {
			t.Fatal("repositório não deve ser chamado com valor inválido")
		}
	})
}

func TestSuitPayPayout_UnknownOutcomeFailsClosedWithoutAck(t *testing.T) {
	// Sem prova de persistência (outcome desconhecido), a SuitPay NUNCA pode
	// receber ACK (HTTP 200 / Success=true). Fail closed com 500, errorCode
	// estável e mensagem genérica sem vazar SQL nem o outcome arbitrário.
	clientSecret := "secret-payout-unknown-outcome"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	cases := []struct {
		name    string
		outcome string
		tx      *domain.Transaction
	}{
		{
			name:    "arbitrary outcome with terminal transaction",
			outcome: "half_applied",
			tx:      &domain.Transaction{ID: "WDR-UNK-1", UserID: "usr-unk", Status: domain.TxStatusSucesso},
		},
		{
			name:    "empty outcome without transaction",
			outcome: "",
			tx:      nil,
		},
		{
			name:    "unknown outcome without transaction",
			outcome: "maybe_done",
			tx:      nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			finRepo := &mockFinancialRepo{
				applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
					return tc.tx, tc.outcome, nil
				},
			}
			uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
				Mode:                payment.ModeSuitPay,
				Environment:         "staging",
				FinancialRepository: finRepo,
			})

			raw := makeSignedSuitPayWebhook(7771, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
			result := uc.ProcessSuitPayWebhook(context.Background(), raw)

			if result.Success {
				t.Fatal("outcome desconhecido NUNCA pode ser confirmado (ACK) ao gateway")
			}
			if result.HTTPStatusCode != http.StatusInternalServerError {
				t.Fatalf("esperado HTTP 500 para outcome desconhecido, obteve: %d", result.HTTPStatusCode)
			}
			if result.ErrorCode != "PAYOUT_CALLBACK_UNKNOWN_OUTCOME" {
				t.Fatalf("esperado errorCode estável PAYOUT_CALLBACK_UNKNOWN_OUTCOME, obteve: %s", result.ErrorCode)
			}
			if result.Error == "" {
				t.Fatal("mensagem genérica é obrigatória")
			}
			if tc.outcome != "" && strings.Contains(result.Error, tc.outcome) {
				t.Fatalf("mensagem não pode vazar o outcome arbitrário %q: %s", tc.outcome, result.Error)
			}
			lower := strings.ToLower(result.Error)
			if strings.Contains(lower, "sql") || strings.Contains(lower, "select") || strings.Contains(lower, "insert") || strings.Contains(lower, "error:") {
				t.Fatalf("mensagem não pode vazar detalhes internos: %s", result.Error)
			}
			if result.Outcome != "" || result.Kind != "" || result.Status != "" || result.TxID != "" || result.UserID != "" {
				t.Fatalf("resposta de falha não pode parecer conciliação (gatilho de notificação WS): %+v", result)
			}
		})
	}
}

func TestSuitPayPayout_InconsistentOutcomeTransactionPairsFailClosed(t *testing.T) {
	// "applied" exige tx != nil com status exatamente sucesso/cancelado;
	// "duplicate" exige tx != nil terminal; "queued" exige tx == nil.
	// Qualquer outra combinação é 500 genérico, sem notificação WS.
	clientSecret := "secret-payout-inconsistent"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	cases := []struct {
		name    string
		outcome string
		tx      *domain.Transaction
	}{
		{"applied without transaction", "applied", nil},
		{"applied with pending transaction", "applied", &domain.Transaction{ID: "WDR-INC-1", UserID: "usr-inc", Status: domain.TxStatusPendente}},
		{"applied with processing transaction", "applied", &domain.Transaction{ID: "WDR-INC-2", UserID: "usr-inc", Status: domain.TxStatusProcessando}},
		{"applied with rejected transaction", "applied", &domain.Transaction{ID: "WDR-INC-3", UserID: "usr-inc", Status: domain.TxStatusRejeitado}},
		{"duplicate without transaction", "duplicate", nil},
		{"duplicate with non-terminal transaction", "duplicate", &domain.Transaction{ID: "WDR-INC-4", UserID: "usr-inc", Status: domain.TxStatusProcessando}},
		{"queued with transaction", "queued", &domain.Transaction{ID: "WDR-INC-5", UserID: "usr-inc", Status: domain.TxStatusSucesso}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			finRepo := &mockFinancialRepo{
				applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
					return tc.tx, tc.outcome, nil
				},
			}
			uc := payment.NewPaymentUseCase(nil, &mockTxRepo{txs: map[string]*domain.Transaction{}}, nil, cfg, nil, "", payment.Options{
				Mode:                payment.ModeSuitPay,
				Environment:         "staging",
				FinancialRepository: finRepo,
			})

			raw := makeSignedSuitPayWebhook(7772, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
			result := uc.ProcessSuitPayWebhook(context.Background(), raw)

			if result.Success {
				t.Fatalf("par %s/%v inconsistente NUNCA pode ser confirmado ao gateway", tc.outcome, tc.tx)
			}
			if result.HTTPStatusCode != http.StatusInternalServerError {
				t.Fatalf("esperado HTTP 500, obteve: %d", result.HTTPStatusCode)
			}
			if result.ErrorCode != "PAYOUT_CALLBACK_INVALID_RESULT" {
				t.Fatalf("esperado errorCode estável PAYOUT_CALLBACK_INVALID_RESULT, obteve: %s", result.ErrorCode)
			}
			if result.Error == "" {
				t.Fatal("mensagem genérica é obrigatória")
			}
			lower := strings.ToLower(result.Error)
			if strings.Contains(lower, "sql") || strings.Contains(lower, "select") || strings.Contains(lower, "pq:") {
				t.Fatalf("mensagem não pode vazar detalhes internos: %s", result.Error)
			}
			if result.Outcome != "" || result.Kind != "" || result.Status != "" || result.TxID != "" || result.UserID != "" {
				t.Fatalf("resposta de falha não pode parecer conciliação (gatilho de notificação WS): %+v", result)
			}
		})
	}
}

// mockPayoutApplierTxRepo implements PayoutOutcomeApplier on the txRepo side to
// prove the constructor must NOT route the callback to a repository different
// from the effective FinancialRepository used for Claim/Record.
type mockPayoutApplierTxRepo struct {
	*mockTxRepo
	called bool
}

func (m *mockPayoutApplierTxRepo) ApplyPayoutCallbackWithOutcome(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
	m.called = true
	return &domain.Transaction{ID: "WDR-FROM-TXREPO", UserID: "wrong-repo", Status: domain.TxStatusSucesso}, "applied", nil
}

// mockCombinedTxAndFinancialRepo implements domain.TransactionRepository,
// FinancialRepository (via legacyFinancialRepoOnly), and PayoutOutcomeApplier
// in a single repository, modeling the production TransactionRepository layout.
type mockCombinedTxAndFinancialRepo struct {
	*mockTxRepo
	legacyFinancialRepoOnly
	called bool
}

func (m *mockCombinedTxAndFinancialRepo) ApplyPayoutCallbackWithOutcome(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
	m.called = true
	return &domain.Transaction{ID: "WDR-FROM-COMBINED", UserID: "usr-combined", Status: domain.TxStatusSucesso}, "applied", nil
}

func TestSuitPayPayout_ApplierResolvedFromEffectiveFinancialRepository(t *testing.T) {
	clientSecret := "secret-payout-applier-source"
	cfg := &mockConfigRepo{cfg: &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			SuitPayEnabled:      true,
			SuitPayClientSecret: clientSecret,
		},
	}}

	t.Run("resolves PayoutApplier from explicit FinancialRepository", func(t *testing.T) {
		txRepo := &mockPayoutApplierTxRepo{mockTxRepo: &mockTxRepo{txs: map[string]*domain.Transaction{}}}
		finCalled := false
		finRepo := &mockFinancialRepo{
			applyPayoutOutcomeFunc: func(ctx context.Context, ext, status string, val float64) (*domain.Transaction, string, error) {
				finCalled = true
				return &domain.Transaction{ID: "WDR-FIN-1", UserID: "usr-fin", Status: domain.TxStatusSucesso}, "applied", nil
			},
		}
		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: finRepo,
		})

		raw := makeSignedSuitPayWebhook(7781, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success || result.Outcome != "applied" || result.TxID != "WDR-FIN-1" {
			t.Fatalf("callback deve ser aplicado pelo FinancialRepository efetivo, obteve: %+v", result)
		}
		if !finCalled {
			t.Fatal("FinancialRepository (dono de Claim/Record) deve aplicar o callback")
		}
		if txRepo.called {
			t.Fatal("PayoutApplier NUNCA deve ser resolvido do txRepo quando o FinancialRepository implementa o contrato")
		}
	})

	t.Run("fails closed when explicit FinancialRepository lacks applier without falling back to txRepo", func(t *testing.T) {
		txRepo := &mockPayoutApplierTxRepo{mockTxRepo: &mockTxRepo{txs: map[string]*domain.Transaction{}}}
		// legacyFinancialRepoOnly implements FinancialRepository WITHOUT
		// ApplyPayoutCallbackWithOutcome. Even though txRepo implements it,
		// the constructor must NOT cross repositories to resolve the applier.
		uc := payment.NewPaymentUseCase(nil, txRepo, nil, cfg, nil, "", payment.Options{
			Mode:                payment.ModeSuitPay,
			Environment:         "staging",
			FinancialRepository: legacyFinancialRepoOnly{},
		})

		raw := makeSignedSuitPayWebhook(7782, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if result.Success {
			t.Fatal("deve falhar fechado quando FinancialRepository explícito não implementa PayoutOutcomeApplier")
		}
		if result.HTTPStatusCode != http.StatusInternalServerError || result.ErrorCode != "PAYOUT_APPLIER_UNAVAILABLE" {
			t.Fatalf("esperado 500 PAYOUT_APPLIER_UNAVAILABLE, obteve: %+v", result)
		}
		if txRepo.called {
			t.Fatal("txRepo NUNCA deve ser chamado quando FinancialRepository explícito não implementa o applier (sem fallback cruzado)")
		}
	})

	t.Run("auto-resolves PayoutApplier when FinancialRepository is inferred from txRepo implementing both", func(t *testing.T) {
		combined := &mockCombinedTxAndFinancialRepo{
			mockTxRepo: &mockTxRepo{txs: map[string]*domain.Transaction{}},
		}
		// FinancialRepository is nil: inferred from txRepo, and PayoutApplier
		// is auto-resolved from the same inferred FinancialRepository.
		uc := payment.NewPaymentUseCase(nil, combined, nil, cfg, nil, "", payment.Options{
			Mode:        payment.ModeSuitPay,
			Environment: "staging",
		})

		raw := makeSignedSuitPayWebhook(7783, "PIX_CASHOUT", "PAID_OUT", 100.00, "", clientSecret)
		result := uc.ProcessSuitPayWebhook(context.Background(), raw)

		if !result.Success || result.Outcome != "applied" || result.TxID != "WDR-FROM-COMBINED" {
			t.Fatalf("callback deve ser aplicado via repositório combinado inferido, obteve: %+v", result)
		}
		if !combined.called {
			t.Fatal("repositório combinado inferido deve aplicar o callback")
		}
	})
}
