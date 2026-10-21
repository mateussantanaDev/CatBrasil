package usecase_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
	"catbrasil/backend/internal/usecase/garage"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/internal/usecase/shop"
	"catbrasil/backend/pkg/crypto"
	"catbrasil/backend/pkg/jwt"
)

// Mocks para os testes de dupla checagem financeira
type mockUserRepoDoubleCheck struct {
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepoDoubleCheck {
	return &mockUserRepoDoubleCheck{users: make(map[string]*domain.User)}
}

func (m *mockUserRepoDoubleCheck) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

func (m *mockUserRepoDoubleCheck) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.GetByID(ctx, id)
}

func (m *mockUserRepoDoubleCheck) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	for _, u := range m.users {
		if strings.EqualFold(u.Username, username) {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *mockUserRepoDoubleCheck) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Phone == phone {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepoDoubleCheck) GetByFingerprintOrIP(ctx context.Context, fp, ip string) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockUserRepoDoubleCheck) Create(ctx context.Context, u *domain.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockUserRepoDoubleCheck) Update(ctx context.Context, u *domain.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockUserRepoDoubleCheck) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	u.Balance += deltaB
	u.YieldBalanceAvailable += deltaY
	return u, nil
}

func (m *mockUserRepoDoubleCheck) List(ctx context.Context, q, s string, l, o int) ([]*domain.User, int, error) {
	var list []*domain.User
	for _, u := range m.users {
		list = append(list, u)
	}
	return list, len(list), nil
}

func (m *mockUserRepoDoubleCheck) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *mockUserRepoDoubleCheck) ResetDailySpins(ctx context.Context) error {
	for _, u := range m.users {
		u.AlreadySpunToday = false
	}
	return nil
}

type mockMachineRepoDoubleCheck struct {
	machines map[string]*domain.Machine
}

func newMockMachineRepo() *mockMachineRepoDoubleCheck {
	return &mockMachineRepoDoubleCheck{machines: make(map[string]*domain.Machine)}
}

func (m *mockMachineRepoDoubleCheck) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	if mach, ok := m.machines[id]; ok {
		return mach, nil
	}
	return nil, domain.ErrMachineNotFound
}

func (m *mockMachineRepoDoubleCheck) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return m.GetByID(ctx, id)
}

func (m *mockMachineRepoDoubleCheck) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	var list []*domain.Machine
	for _, mach := range m.machines {
		if mach.UserID == userID {
			list = append(list, mach)
		}
	}
	return list, nil
}

func (m *mockMachineRepoDoubleCheck) Create(ctx context.Context, mach *domain.Machine) error {
	m.machines[mach.ID] = mach
	return nil
}

func (m *mockMachineRepoDoubleCheck) Update(ctx context.Context, mach *domain.Machine) error {
	m.machines[mach.ID] = mach
	return nil
}

func (m *mockMachineRepoDoubleCheck) CollectYield(ctx context.Context, machID, userID string, bonus float64) (*domain.Machine, float64, error) {
	return nil, 0, nil
}

func (m *mockMachineRepoDoubleCheck) ResetDailyMachines(ctx context.Context) error {
	return nil
}

func (m *mockMachineRepoDoubleCheck) Delete(ctx context.Context, id string) error {
	delete(m.machines, id)
	return nil
}

type mockTxRepoDoubleCheck struct {
	txs map[string]*domain.Transaction
}

func newMockTxRepo() *mockTxRepoDoubleCheck {
	return &mockTxRepoDoubleCheck{txs: make(map[string]*domain.Transaction)}
}

func (m *mockTxRepoDoubleCheck) Create(ctx context.Context, tx *domain.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}

func (m *mockTxRepoDoubleCheck) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	if tx, ok := m.txs[id]; ok {
		return tx, nil
	}
	return nil, nil
}

func (m *mockTxRepoDoubleCheck) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return m.GetByID(ctx, id)
}

func (m *mockTxRepoDoubleCheck) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	var list []*domain.Transaction
	for _, tx := range m.txs {
		if tx.UserID == userID {
			list = append(list, tx)
		}
	}
	return list, nil
}

func (m *mockTxRepoDoubleCheck) GetAll(ctx context.Context, status string, l, o int) ([]*domain.Transaction, int, error) {
	var list []*domain.Transaction
	for _, tx := range m.txs {
		list = append(list, tx)
	}
	return list, len(list), nil
}

func (m *mockTxRepoDoubleCheck) Update(ctx context.Context, tx *domain.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}

func (m *mockTxRepoDoubleCheck) ApproveWithdrawal(ctx context.Context, id string) (*domain.Transaction, error) {
	tx := m.txs[id]
	if tx != nil {
		tx.Status = domain.TxStatusSucesso
	}
	return tx, nil
}

func (m *mockTxRepoDoubleCheck) RejectWithdrawal(ctx context.Context, id, reason string, lock bool) (*domain.Transaction, error) {
	tx := m.txs[id]
	if tx != nil {
		tx.Status = domain.TxStatusRejeitado
		tx.RejectionReason = &reason
	}
	return tx, nil
}

type mockCatalogRepoDoubleCheck struct {
	items map[string]*domain.FleetCatalogItem
}

func newMockCatalogRepo() *mockCatalogRepoDoubleCheck {
	return &mockCatalogRepoDoubleCheck{items: make(map[string]*domain.FleetCatalogItem)}
}

func (m *mockCatalogRepoDoubleCheck) GetAll(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	var list []*domain.FleetCatalogItem
	for _, i := range m.items {
		list = append(list, i)
	}
	return list, nil
}

func (m *mockCatalogRepoDoubleCheck) GetActive(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return m.GetAll(ctx)
}

func (m *mockCatalogRepoDoubleCheck) GetByID(ctx context.Context, id string) (*domain.FleetCatalogItem, error) {
	if i, ok := m.items[id]; ok {
		return i, nil
	}
	return nil, domain.ErrMachineCatalogNotFound
}

func (m *mockCatalogRepoDoubleCheck) Upsert(ctx context.Context, item *domain.FleetCatalogItem) error {
	m.items[item.ID] = item
	return nil
}

func (m *mockCatalogRepoDoubleCheck) SaveCatalog(ctx context.Context, items []*domain.FleetCatalogItem) error {
	for _, i := range items {
		m.items[i.ID] = i
	}
	return nil
}

func (m *mockCatalogRepoDoubleCheck) Delete(ctx context.Context, id string) error {
	delete(m.items, id)
	return nil
}

type mockReferralRepoDoubleCheck struct{}

func (m *mockReferralRepoDoubleCheck) Create(ctx context.Context, r *domain.Referral) error {
	return nil
}
func (m *mockReferralRepoDoubleCheck) Update(ctx context.Context, r *domain.Referral) error {
	return nil
}
func (m *mockReferralRepoDoubleCheck) GetByID(ctx context.Context, id string) (*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepoDoubleCheck) GetAll(ctx context.Context) ([]*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepoDoubleCheck) GetByReferrerID(ctx context.Context, id string) ([]*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepoDoubleCheck) GetByReferredName(ctx context.Context, n string) (*domain.Referral, error) {
	return nil, nil
}
func (m *mockReferralRepoDoubleCheck) CountActiveByReferrerID(ctx context.Context, id string) (int, error) {
	return 0, nil
}

type mockRankingRepoDoubleCheck struct{}

func (m *mockRankingRepoDoubleCheck) AddOrUpdateWeeklyTonnage(ctx context.Context, uid, uName string, t float64) error {
	return nil
}
func (m *mockRankingRepoDoubleCheck) GetWeeklyLeaderboard(ctx context.Context, l int) ([]*domain.LeaderboardEntry, error) {
	return nil, nil
}
func (m *mockRankingRepoDoubleCheck) GetUserRank(ctx context.Context, uid string) (*domain.LeaderboardEntry, error) {
	return nil, nil
}
func (m *mockRankingRepoDoubleCheck) ResetWeeklyLeaderboard(ctx context.Context) error { return nil }

type mockLogRepoDoubleCheck struct{}

func (m *mockLogRepoDoubleCheck) LogUsage(ctx context.Context, t string, a float64) error { return nil }
func (m *mockLogRepoDoubleCheck) LogSecurity(ctx context.Context, uid *string, e, ip, fp, d string) error {
	return nil
}
func (m *mockLogRepoDoubleCheck) GetUsageLogs(ctx context.Context, l, o int) ([]*domain.UsageLog, error) {
	return nil, nil
}
func (m *mockLogRepoDoubleCheck) GetSecurityLogs(ctx context.Context, uid *string, l, o int) ([]*domain.SecurityLog, error) {
	return nil, nil
}

type mockConfigRepoDoubleCheck struct {
	cfg *domain.SystemConfig
}

func (m *mockConfigRepoDoubleCheck) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	return m.cfg, nil
}
func (m *mockConfigRepoDoubleCheck) UpdateConfig(ctx context.Context, c *domain.SystemConfig) error {
	m.cfg = c
	return nil
}
func (m *mockConfigRepoDoubleCheck) ToggleGlobalWithdrawLock(ctx context.Context, l bool) error {
	m.cfg.GlobalWithdrawalsLocked = l
	return nil
}
func (m *mockConfigRepoDoubleCheck) UpdateAutoApprove(ctx context.Context, e bool, u float64) error {
	m.cfg.AutoApproveEnabled = e
	m.cfg.AutoApproveWithdrawalsUnder = u
	return nil
}
func (m *mockConfigRepoDoubleCheck) UpdateGatewayConfig(ctx context.Context, gw domain.GatewayConfig, g int) error {
	return nil
}
func (m *mockConfigRepoDoubleCheck) UpdateReferralConfig(ctx context.Context, c float64, rc int, b float64, r, a bool) error {
	return nil
}
func (m *mockConfigRepoDoubleCheck) ResetSeason(ctx context.Context, durationDays int) error {
	return nil
}
func (m *mockConfigRepoDoubleCheck) UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error {
	return nil
}

type mockDRERepoDoubleCheck struct{}

func (m *mockDRERepoDoubleCheck) GetMetrics(ctx context.Context) (*domain.DREMetrics, error) {
	return &domain.DREMetrics{
		TotalDeposits:     50000.0,
		TotalWithdrawals:  10000.0,
		NetDeposits:       49250.0,
		NetBankProfit:     39250.0,
		ProfitMargin:      78.5,
		TotalUserBalances: 15000.0,
	}, nil
}

// 1. DUPLA CHECAGEM: MOTOR DE MINERAÇÃO, TOLERÂNCIA DE 10 MIN & PROGRESSÃO DE CARREIRA
func TestDoubleCheck_MiningEngine_GraceWindowAndCareerBonus(t *testing.T) {
	ctx := context.Background()

	userRepo := newMockUserRepo()
	machineRepo := newMockMachineRepo()
	txRepo := newMockTxRepo()
	refRepo := &mockReferralRepoDoubleCheck{}
	rankRepo := &mockRankingRepoDoubleCheck{}
	logRepo := &mockLogRepoDoubleCheck{}

	garageUC := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, refRepo, rankRepo, logRepo, nil, nil)

	// Operador Nível 8 (+35% Bônus de Rendimento)
	user := &domain.User{
		ID:                    "USR-OP-8",
		Username:              "operador_master",
		Nivel:                 8,
		Balance:               1000.00,
		YieldBalanceAvailable: 200.00,
		ColetasCompleted:      2,
	}
	_ = userRepo.Create(ctx, user)

	baseYield := 110.00 // Escavadeira CAT 320

	t.Run("Sub-teste 1.1: Coleta antes de 24h (23h59m59s) DEVE falhar com ErrYieldNotReady", func(t *testing.T) {
		mach := &domain.Machine{
			ID:              "mach-early",
			UserID:          user.ID,
			Name:            "Escavadeira CAT 320",
			Cost:            2500.00,
			YieldAmount:     baseYield,
			LastCollectedAt: time.Now().Add(-23*time.Hour - 59*time.Minute - 59*time.Second),
		}
		_ = machineRepo.Create(ctx, mach)

		_, err := garageUC.CollectYield(ctx, user.ID, mach.ID)
		if err != domain.ErrYieldNotReady {
			t.Errorf("esperado ErrYieldNotReady, obteve: %v", err)
		}
	})

	t.Run("Sub-teste 1.2: Coleta exatamente em 24h00m00s DEVE suceder com Bônus de Nível 8 (+35%)", func(t *testing.T) {
		mach := &domain.Machine{
			ID:              "mach-exact-24h",
			UserID:          user.ID,
			Name:            "Escavadeira CAT 320",
			Cost:            2500.00,
			YieldAmount:     baseYield,
			LastCollectedAt: time.Now().Add(-24 * time.Hour),
		}
		_ = machineRepo.Create(ctx, mach)

		resp, err := garageUC.CollectYield(ctx, user.ID, mach.ID)
		if err != nil {
			t.Fatalf("coleta falhou inesperadamente: %v", err)
		}

		// 110 * 1.35 = 148.50
		expectedYield := 148.50
		if resp.CollectedAmount != expectedYield {
			t.Errorf("rendimento incorreto: esperado %.2f, obteve %.2f", expectedYield, resp.CollectedAmount)
		}
		if resp.NewBalance != 1148.50 {
			t.Errorf("novo saldo incorreto: esperado 1148.50, obteve %.2f", resp.NewBalance)
		}
		if resp.User.YieldBalanceAvailable != 348.50 {
			t.Errorf("novo saldo de reinvest incorreto: esperado 348.50, obteve %.2f", resp.User.YieldBalanceAvailable)
		}
	})

	t.Run("Sub-teste 1.3: Coleta dentro da tolerância de 10 min (24h + 5m) DEVE suceder", func(t *testing.T) {
		mach := &domain.Machine{
			ID:              "mach-grace-5m",
			UserID:          user.ID,
			Name:            "Escavadeira CAT 320",
			Cost:            2500.00,
			YieldAmount:     baseYield,
			LastCollectedAt: time.Now().Add(-24*time.Hour - 5*time.Minute),
		}
		_ = machineRepo.Create(ctx, mach)

		resp, err := garageUC.CollectYield(ctx, user.ID, mach.ID)
		if err != nil {
			t.Fatalf("coleta na janela de tolerância falhou: %v", err)
		}
		if resp.CollectedAmount != 148.50 {
			t.Errorf("rendimento incorreto na tolerância: esperado 148.50, obteve %.2f", resp.CollectedAmount)
		}
	})

	t.Run("Sub-teste 1.4: Coleta após estourar a tolerância de 10 min (24h + 10m01s) DEVE falhar com ErrYieldExpired", func(t *testing.T) {
		mach := &domain.Machine{
			ID:              "mach-expired",
			UserID:          user.ID,
			Name:            "Escavadeira CAT 320",
			Cost:            2500.00,
			YieldAmount:     baseYield,
			LastCollectedAt: time.Now().Add(-24*time.Hour - 10*time.Minute - 1*time.Second),
		}
		_ = machineRepo.Create(ctx, mach)

		_, err := garageUC.CollectYield(ctx, user.ID, mach.ID)
		if err != domain.ErrYieldExpired {
			t.Errorf("esperado ErrYieldExpired, obteve: %v", err)
		}
	})
}

// 2. DUPLA CHECAGEM: LINHA REINVEST TURBINADO & ISOLAMENTO DE SALDO
func TestDoubleCheck_Shop_ReinvestLineBalanceIsolation(t *testing.T) {
	ctx := context.Background()

	userRepo := newMockUserRepo()
	catalogRepo := newMockCatalogRepo()
	machineRepo := newMockMachineRepo()
	txRepo := newMockTxRepo()

	shopUC := shop.NewShopUseCase(userRepo, catalogRepo, machineRepo, txRepo)

	// Modelo da Linha Reinvest Turbinado
	reinvestMachine := &domain.FleetCatalogItem{
		ID:                  "cat_320_reinvest",
		Name:                "Escavadeira CAT 320 (Reinvest)",
		Category:            "REINVEST",
		Price:               150.00,
		DailyYield:          102.85,
		ReturnPercentPerDay: 68.56,
		DurationDays:        7,
		ReinvestOnly:        true,
		Active:              true,
	}
	_ = catalogRepo.Upsert(ctx, reinvestMachine)

	t.Run("Sub-teste 2.1: Operador com R$ 10.000 de saldo comum mas R$ 0 de rendimentos NÃO PODE alugar Reinvest", func(t *testing.T) {
		user := &domain.User{
			ID:                    "USR-NO-YIELD",
			Username:              "deposito_puro",
			Balance:               10000.00,
			YieldBalanceAvailable: 0.00, // Zero saldo gerado de coletas
		}
		_ = userRepo.Create(ctx, user)

		_, err := shopUC.RentMachine(ctx, user.ID, "cat_320_reinvest")
		if err != domain.ErrInsufficientYieldBalance {
			t.Errorf("esperado ErrInsufficientYieldBalance, obteve: %v", err)
		}
	})

	t.Run("Sub-teste 2.2: Operador com saldo de rendimento suficiente aluga com sucesso e ambos os saldos são deduzidos", func(t *testing.T) {
		user := &domain.User{
			ID:                    "USR-WITH-YIELD",
			Username:              "minerador_coletador",
			Balance:               1000.00,
			YieldBalanceAvailable: 200.00, // R$ 200 > R$ 150
		}
		_ = userRepo.Create(ctx, user)

		resp, err := shopUC.RentMachine(ctx, user.ID, "cat_320_reinvest")
		if err != nil {
			t.Fatalf("aluguel reinvest falhou: %v", err)
		}

		if resp.NewBalance != 850.00 {
			t.Errorf("saldo geral incorreto: esperado 850.00, obteve %.2f", resp.NewBalance)
		}
		if resp.User.YieldBalanceAvailable != 50.00 {
			t.Errorf("saldo reinvest incorreto: esperado 50.00, obteve %.2f", resp.User.YieldBalanceAvailable)
		}
	})
}

// 3. DUPLA CHECAGEM: SAQUES, ROLLOVER & AUTENTICAÇÃO 2FA
func TestDoubleCheck_Payment_RolloverAndSecurityRules(t *testing.T) {
	ctx := context.Background()

	userRepo := newMockUserRepo()
	txRepo := newMockTxRepo()
	refRepo := &mockReferralRepoDoubleCheck{}
	configRepo := &mockConfigRepoDoubleCheck{
		cfg: &domain.SystemConfig{GlobalWithdrawalsLocked: false},
	}
	logRepo := &mockLogRepoDoubleCheck{}

	secret2FA, _ := crypto.GenerateTOTPSecret()

	paymentUC := payment.NewPaymentUseCase(userRepo, txRepo, refRepo, configRepo, logRepo, "cat_secret", payment.Options{Mode: payment.ModeSimulated, Environment: "development"})
	pinHash, _ := crypto.HashPassword("123456")

	user := &domain.User{
		ID:                 "USR-WITHDRAW-TEST",
		Username:           "test_withdraw",
		Balance:            1000.00,
		SecurityPassword:   pinHash,
		TwoFactorSecret:    secret2FA,
		TwoFactorEnabled:   true,
		RequiredRollover:   500.00,
		CurrentRollover:    200.00, // Rollover insuficiente!
		FreeWithdrawActive: false,
	}
	_ = userRepo.Create(ctx, user)

	t.Run("Sub-teste 3.1: Saque com Rollover pendente DEVE ser bloqueado", func(t *testing.T) {
		req := payment.PayoutRequest{
			UserID:           user.ID,
			Amount:           300.00,
			PixKey:           "11999999999",
			SecurityPassword: "123456",
		}
		_, err := paymentUC.RequestPayout(ctx, req)
		if err == nil || !strings.Contains(err.Error(), "giro mínimo") {
			t.Errorf("esperado bloqueio por rollover pendente, obteve: %v", err)
		}
	})

	t.Run("Sub-teste 3.2: Saque com senha financeira incorreta DEVE falhar", func(t *testing.T) {
		user.CurrentRollover = 600.00 // Cumpre o rollover
		_ = userRepo.Update(ctx, user)

		req := payment.PayoutRequest{
			UserID:           user.ID,
			Amount:           300.00,
			PixKey:           "11999999999",
			SecurityPassword: "WRONG_PASSWORD",
		}
		_, err := paymentUC.RequestPayout(ctx, req)
		if err != domain.ErrInvalidSecurityPassword {
			t.Errorf("esperado ErrInvalidSecurityPassword, obteve: %v", err)
		}
	})

	t.Run("Sub-teste 3.3: Saque > R$ 500,00 com 2FA ativado exige token TOTP válido", func(t *testing.T) {
		req := payment.PayoutRequest{
			UserID:           user.ID,
			Amount:           600.00,
			PixKey:           "11999999999",
			SecurityPassword: "123456",
			TwoFactorCode:    "000000", // Código inválido
		}
		_, err := paymentUC.RequestPayout(ctx, req)
		if err != domain.ErrInvalid2FACode {
			t.Errorf("esperado ErrInvalid2FACode para saque > 500 sem 2FA, obteve: %v", err)
		}

		// Agora com código 2FA correto gerado imediatamente antes da chamada
		totpValidCode, err := crypto.GenerateTOTPCode(user.TwoFactorSecret, time.Now())
		if err != nil {
			t.Fatalf("falha ao gerar código TOTP: %v", err)
		}
		req.TwoFactorCode = totpValidCode
		resp, err := paymentUC.RequestPayout(ctx, req)
		if err != nil {
			t.Fatalf("saque com 2FA válido falhou: %v", err)
		}

		// Taxa padrão 5%: 600 * 0.05 = 30.00 -> Líquido 570.00
		if resp.Fee != 30.00 || resp.NetAmount != 570.00 {
			t.Errorf("cálculo de taxa de saque incorreto: Taxa %.2f, Líquido %.2f", resp.Fee, resp.NetAmount)
		}
		if resp.NewBalance != 400.00 {
			t.Errorf("saldo restante incorreto: esperado 400.00, obteve %.2f", resp.NewBalance)
		}
	})
}

// 4. DUPLA CHECAGEM: DRE CONTÁBIL DA DIRETORIA EM TEMPO REAL
func TestDoubleCheck_Admin_DREAccountingInvariants(t *testing.T) {
	ctx := context.Background()

	adminUC := admin.NewAdminUseCase(nil, nil, nil, &mockDRERepoDoubleCheck{}, nil, nil, nil, nil, nil)
	dre, err := adminUC.GetDRE(ctx)
	if err != nil {
		t.Fatalf("falha ao obter DRE: %v", err)
	}

	if dre.NetBankProfit != 39250.0 {
		t.Errorf("Lucro líquido incorreto: esperado 39250.0, obteve %.2f", dre.NetBankProfit)
	}

	if dre.ProfitMargin != 78.5 {
		t.Errorf("Margem percentual incorreta: esperado 78.5%%, obteve %.2f%%", dre.ProfitMargin)
	}
}

// 5. DUPLA CHECAGEM: CRIPTOGRAFIA, TIMING SAFE, HMAC-SHA256 & JWT
func TestDoubleCheck_Cryptography_SecurityHardening(t *testing.T) {
	// 5.1 Bcrypt Custo 12
	pass := "SenhaSegura2026!#"
	hash, err := crypto.HashPassword(pass)
	if err != nil {
		t.Fatalf("HashPassword falhou: %v", err)
	}
	if !crypto.CheckPassword(pass, hash) {
		t.Errorf("CheckPassword falhou para senha correta")
	}
	if crypto.CheckPassword("SenhaIncorreta", hash) {
		t.Errorf("CheckPassword aprovou senha incorreta!")
	}

	// 5.2 HMAC-SHA256 para Webhooks
	payload := "PIX-12345:paid:150.00"
	secret := "secret_webhook_key_7788"
	sig := crypto.GenerateHMACSHA256(payload, secret)

	if !crypto.VerifyHMACSHA256(payload, sig, secret) {
		t.Errorf("VerifyHMACSHA256 falhou para assinatura legítima")
	}
	if crypto.VerifyHMACSHA256(payload, sig, "wrong_secret") {
		t.Errorf("VerifyHMACSHA256 aprovou com secret incorreto!")
	}
	if crypto.VerifyHMACSHA256(payload+"_tampered", sig, secret) {
		t.Errorf("VerifyHMACSHA256 aprovou payload adulterado!")
	}

	// 5.3 JWT Duplo Papel
	tm := jwt.NewTokenManager("super_jwt_secret_123456789_enterprise", 2)
	adminUser := &domain.User{
		ID:       "gst-001",
		Username: "admin",
		Role:     domain.RoleDiretoria,
	}
	token, err := tm.GenerateToken(adminUser)
	if err != nil {
		t.Fatalf("GenerateToken falhou: %v", err)
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken falhou: %v", err)
	}
	if !claims.IsAdmin || claims.UserID != "gst-001" || claims.Role != domain.RoleDiretoria {
		t.Errorf("claims incorretos no token JWT: %+v", claims)
	}
}
