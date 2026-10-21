package admin_test

import (
	"context"
	"testing"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/admin"
)

type mockUserRepo struct {
	user *domain.User
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return m.user, nil
}
func (m *mockUserRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.User, error) {
	return m.user, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.user, nil
}
func (m *mockUserRepo) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error {
	m.user = user
	return nil
}
func (m *mockUserRepo) AdjustBalance(ctx context.Context, id string, deltaB, deltaY float64) (*domain.User, error) {
	m.user.Balance += deltaB
	m.user.YieldBalanceAvailable += deltaY
	return m.user, nil
}
func (m *mockUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return []*domain.User{m.user}, 1, nil
}
func (m *mockUserRepo) Count(ctx context.Context) (int64, error) { return 1, nil }
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

type mockTxRepo struct {
	tx *domain.Transaction
}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return m.tx, nil
}
func (m *mockTxRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return m.tx, nil
}
func (m *mockTxRepo) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	return []*domain.Transaction{m.tx}, nil
}
func (m *mockTxRepo) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return []*domain.Transaction{m.tx}, 1, nil
}
func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	m.tx.Status = domain.TxStatusSucesso
	return m.tx, nil
}
func (m *mockTxRepo) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	m.tx.Status = domain.TxStatusRejeitado
	return m.tx, nil
}

type mockDRERepo struct{}

func (m *mockDRERepo) GetMetrics(ctx context.Context) (*domain.DREMetrics, error) {
	return &domain.DREMetrics{
		TotalDeposits:    100000.0,
		TotalWithdrawals: 30000.0,
		NetBankProfit:    20000.0,
		ProfitMargin:     20.0,
	}, nil
}

type mockCatalogRepo struct{}

func (m *mockCatalogRepo) GetAll(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return nil, nil
}
func (m *mockCatalogRepo) GetActive(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return nil, nil
}
func (m *mockCatalogRepo) GetByID(ctx context.Context, id string) (*domain.FleetCatalogItem, error) {
	return nil, nil
}
func (m *mockCatalogRepo) Upsert(ctx context.Context, item *domain.FleetCatalogItem) error {
	return nil
}
func (m *mockCatalogRepo) SaveCatalog(ctx context.Context, items []*domain.FleetCatalogItem) error {
	return nil
}
func (m *mockCatalogRepo) Delete(ctx context.Context, id string) error { return nil }

type mockRouletteRepo struct {
	slices []*domain.RouletteSlice
}

func (m *mockRouletteRepo) GetSlices(ctx context.Context) ([]*domain.RouletteSlice, error) {
	return m.slices, nil
}
func (m *mockRouletteRepo) SaveSlices(ctx context.Context, slices []*domain.RouletteSlice) error {
	m.slices = slices
	return nil
}
func (m *mockRouletteRepo) GetSliceByID(ctx context.Context, id int) (*domain.RouletteSlice, error) {
	return nil, nil
}

type mockAnnounceRepo struct{}

func (m *mockAnnounceRepo) GetAll(ctx context.Context) ([]*domain.Announcement, error) {
	return nil, nil
}
func (m *mockAnnounceRepo) GetActive(ctx context.Context) ([]*domain.Announcement, error) {
	return nil, nil
}
func (m *mockAnnounceRepo) GetByID(ctx context.Context, id string) (*domain.Announcement, error) {
	return nil, nil
}
func (m *mockAnnounceRepo) Create(ctx context.Context, ann *domain.Announcement) error { return nil }
func (m *mockAnnounceRepo) Update(ctx context.Context, ann *domain.Announcement) error { return nil }
func (m *mockAnnounceRepo) ToggleActive(ctx context.Context, id string) (*domain.Announcement, error) {
	return nil, nil
}
func (m *mockAnnounceRepo) Delete(ctx context.Context, id string) error     { return nil }
func (m *mockAnnounceRepo) IncrementViews(ctx context.Context, id string) error { return nil }

type mockConfigRepo struct{}

func (m *mockConfigRepo) GetConfig(ctx context.Context) (*domain.SystemConfig, error) {
	return &domain.SystemConfig{GlobalWithdrawalsLocked: false}, nil
}
func (m *mockConfigRepo) UpdateConfig(ctx context.Context, config *domain.SystemConfig) error {
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

func (m *mockLogRepo) LogUsage(ctx context.Context, logType string, amount float64) error {
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

type mockMachineRepo struct{}

func (m *mockMachineRepo) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) Create(ctx context.Context, machine *domain.Machine) error { return nil }
func (m *mockMachineRepo) Update(ctx context.Context, machine *domain.Machine) error { return nil }
func (m *mockMachineRepo) CollectYield(ctx context.Context, machineID, userID string, bonusPercent float64) (*domain.Machine, float64, error) {
	return nil, 0, nil
}
func (m *mockMachineRepo) ResetDailyMachines(ctx context.Context) error { return nil }
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error           { return nil }

func TestAdminUseCase_DREAndModeration(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{ID: "USR-1", Username: "operador", Balance: 500.0}
	tx := &domain.Transaction{ID: "WDR-1", UserID: "USR-1", Valor: -100.0, Status: domain.TxStatusPendente}

	userRepo := &mockUserRepo{user: user}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{tx: tx}
	dreRepo := &mockDRERepo{}
	catalogRepo := &mockCatalogRepo{}
	rouletteRepo := &mockRouletteRepo{}
	announceRepo := &mockAnnounceRepo{}
	configRepo := &mockConfigRepo{}
	logRepo := &mockLogRepo{}

	uc := admin.NewAdminUseCase(userRepo, machineRepo, txRepo, dreRepo, catalogRepo, rouletteRepo, announceRepo, configRepo, logRepo)

	// DRE
	dre, err := uc.GetDRE(ctx)
	if err != nil {
		t.Fatalf("falha ao obter DRE: %v", err)
	}
	if dre.NetBankProfit != 20000.0 {
		t.Errorf("lucro líquido incorreto: %.2f", dre.NetBankProfit)
	}

	// Aprovação de Saque
	approvedTx, err := uc.ApproveWithdrawal(ctx, "WDR-1")
	if err != nil {
		t.Fatalf("falha na aprovação do saque: %v", err)
	}
	if approvedTx.Status != domain.TxStatusSucesso {
		t.Errorf("esperado status sucesso no saque aprovado")
	}

	// Validação de Roleta (soma 90% deve falhar!)
	badSlices := []*domain.RouletteSlice{
		{ID: 1, Probability: 50.0},
		{ID: 2, Probability: 40.0},
	}
	errBadRoulette := uc.UpdateRouletteConfig(ctx, badSlices)
	if errBadRoulette != domain.ErrProbabilitiesMustEqual100 {
		t.Errorf("esperado ErrProbabilitiesMustEqual100, obteve: %v", errBadRoulette)
	}

	// Validação de Roleta (soma 100% deve passar)
	goodSlices := []*domain.RouletteSlice{
		{ID: 1, Probability: 50.0},
		{ID: 2, Probability: 50.0},
	}
	errGoodRoulette := uc.UpdateRouletteConfig(ctx, goodSlices)
	if errGoodRoulette != nil {
		t.Fatalf("erro ao salvar roleta válida: %v", errGoodRoulette)
	}
}
