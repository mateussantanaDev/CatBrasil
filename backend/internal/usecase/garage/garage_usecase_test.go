package garage_test

import (
	"context"
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/garage"
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
	return m.user, nil
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

type mockMachineRepo struct {
	machine  *domain.Machine
	machines []*domain.Machine
}

func (m *mockMachineRepo) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return m.machine, nil
}
func (m *mockMachineRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return m.machine, nil
}
func (m *mockMachineRepo) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	if len(m.machines) > 0 {
		return m.machines, nil
	}
	if m.machine != nil {
		return []*domain.Machine{m.machine}, nil
	}
	return nil, nil
}
func (m *mockMachineRepo) Create(ctx context.Context, machine *domain.Machine) error { return nil }
func (m *mockMachineRepo) Update(ctx context.Context, machine *domain.Machine) error {
	m.machine = machine
	return nil
}
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error { return nil }

type mockTxRepo struct {
	created []*domain.Transaction
}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	return m.created, nil
}
func (m *mockTxRepo) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return nil, 0, nil
}
func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	m.created = append(m.created, tx)
	return nil
}
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	return nil, nil
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

type mockRankingRepo struct {
	tonnage float64
}

func (m *mockRankingRepo) AddOrUpdateWeeklyTonnage(ctx context.Context, userID, username string, tonnage float64) error {
	m.tonnage += tonnage
	return nil
}
func (m *mockRankingRepo) GetWeeklyLeaderboard(ctx context.Context, limit int) ([]*domain.LeaderboardEntry, error) {
	return nil, nil
}
func (m *mockRankingRepo) GetUserRank(ctx context.Context, userID string) (*domain.LeaderboardEntry, error) {
	return nil, nil
}
func (m *mockRankingRepo) ResetWeeklyLeaderboard(ctx context.Context) error { return nil }

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

func TestGarageUseCase_CollectYield_SuccessWithLevelBonus(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:               "USR-1",
		Username:         "mateus",
		Balance:          500.0,
		Nivel:            3, // Bônus de 10% (Nível 3: (3-1)*5%)
		ColetasCompleted: 2,
	}

	machine := &domain.Machine{
		ID:              "MACH-1",
		UserID:          "USR-1",
		Name:            "Escavadeira CAT 320",
		Cost:            2500.0,
		YieldAmount:     100.0,
		LastCollectedAt: time.Now().Add(-24*time.Hour - 5*time.Minute),
	}

	userRepo := &mockUserRepo{user: user}
	machineRepo := &mockMachineRepo{machine: machine}
	txRepo := &mockTxRepo{}
	referralRepo := &mockReferralRepo{}
	rankingRepo := &mockRankingRepo{}
	logRepo := &mockLogRepo{}

	uc := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, referralRepo, rankingRepo, logRepo, nil, nil)

	res, err := uc.CollectYield(ctx, "USR-1", "MACH-1")
	if err != nil {
		t.Fatalf("erro inesperado na coleta: %v", err)
	}

	expectedYield := 110.0 // R$ 100 base + 10% bônus de nível 3
	if res.CollectedAmount != expectedYield {
		t.Errorf("esperado rendimento R$ %.2f, obteve: R$ %.2f", expectedYield, res.CollectedAmount)
	}

	expectedBalance := 610.0
	if user.Balance != expectedBalance {
		t.Errorf("esperado novo saldo R$ %.2f, obteve: R$ %.2f", expectedBalance, user.Balance)
	}

	if user.ColetasCompleted != 3 {
		t.Errorf("esperado 3 coletas concluídas, obteve: %d", user.ColetasCompleted)
	}

	if rankingRepo.tonnage != expectedYield*10.0 {
		t.Errorf("esperado %.2f toneladas na corrida semanal, obteve: %.2f", expectedYield*10.0, rankingRepo.tonnage)
	}
}

func TestGarageUseCase_CollectYield_ExpiredGraceWindow(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:       "USR-1",
		Username: "mateus",
		Balance:  500.0,
	}

	machine := &domain.Machine{
		ID:              "MACH-1",
		UserID:          "USR-1",
		Name:            "Escavadeira CAT 320",
		Cost:            2500.0,
		YieldAmount:     100.0,
		LastCollectedAt: time.Now().Add(-24*time.Hour - 15*time.Minute),
	}

	userRepo := &mockUserRepo{user: user}
	machineRepo := &mockMachineRepo{machine: machine}
	txRepo := &mockTxRepo{}
	referralRepo := &mockReferralRepo{}
	rankingRepo := &mockRankingRepo{}
	logRepo := &mockLogRepo{}

	uc := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, referralRepo, rankingRepo, logRepo, nil, nil)

	_, err := uc.CollectYield(ctx, "USR-1", "MACH-1")
	if err != domain.ErrYieldExpired {
		t.Fatalf("esperado ErrYieldExpired, obteve: %v", err)
	}

	if user.Balance != 500.0 {
		t.Errorf("saldo não deveria ter sido alterado na expiração")
	}
}

func TestGarageUseCase_LevelUp(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:               "USR-1",
		Username:         "mateus",
		Nivel:            1,
		ColetasCompleted: 5,
		TratorRented:     true,
	}

	referrals := []*domain.Referral{
		{ID: "ref-1", Status: domain.ReferralStatusAtivo},
		{ID: "ref-2", Status: domain.ReferralStatusAtivo},
		{ID: "ref-3", Status: domain.ReferralStatusAtivo},
	}

	userRepo := &mockUserRepo{user: user}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{}
	referralRepo := &mockReferralRepo{referrals: referrals}
	rankingRepo := &mockRankingRepo{}
	logRepo := &mockLogRepo{}

	uc := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, referralRepo, rankingRepo, logRepo, nil, nil)

	res, err := uc.LevelUp(ctx, "USR-1")
	if err != nil {
		t.Fatalf("falha na promoção de nível: %v", err)
	}

	if res.NewLevel != 2 {
		t.Errorf("esperado nível 2, obteve: %d", res.NewLevel)
	}

	if user.ColetasCompleted != 0 || user.TratorRented != false {
		t.Errorf("esperado reset dos requisitos para o próximo nível")
	}
}

type mockLevelRepo struct {
	levels map[int]*domain.LevelConfig
}

func (m *mockLevelRepo) GetAll(ctx context.Context) ([]*domain.LevelConfig, error) {
	var list []*domain.LevelConfig
	for i := 1; i <= 12; i++ {
		if l, ok := m.levels[i]; ok {
			list = append(list, l)
		}
	}
	return list, nil
}

func (m *mockLevelRepo) GetByLevel(ctx context.Context, level int) (*domain.LevelConfig, error) {
	if l, ok := m.levels[level]; ok {
		return l, nil
	}
	return nil, nil
}

func (m *mockLevelRepo) Update(ctx context.Context, config *domain.LevelConfig) error {
	m.levels[config.Level] = config
	return nil
}

func TestGarageUseCase_StrictBattlePass(t *testing.T) {
	ctx := context.Background()

	lvlRepo := &mockLevelRepo{
		levels: map[int]*domain.LevelConfig{
			1: {Level: 1, Title: "Nível 1", CommissionPercent: 5.0, BonusYieldPercent: 0},
			2: {Level: 2, Title: "Nível 2", RequiredReferrals: 8, RequiredMachines: 5, RequiredColetas: 5, CommissionPercent: 5.0, BonusYieldPercent: 5.0},
			5: {Level: 5, Title: "Nível 5", RequiredReferrals: 8, RequiredMachines: 5, HasInsuranceVoucher: true, InsuranceDaysRecovered: 2},
			6: {Level: 6, Title: "Nível 6", RequiredReferrals: 12, RequiredMachines: 8, HasExtraRouletteSpin: true, CommissionPercent: 8.0},
			12: {Level: 12, Title: "Nível 12", RequiredReferrals: 15, RequiredMachines: 12, MaxMachinesAllowed: 25, CommissionPercent: 10.0, SpecialRewardText: "Smartphone"},
		},
	}

	t.Run("Bloqueia avanço se faltar 1 único convite ou máquina", func(t *testing.T) {
		user := &domain.User{
			ID:               "USR-BATTLE-1",
			Username:         "operador_passe",
			Nivel:            1,
			ColetasCompleted: 5,
		}
		userRepo := &mockUserRepo{user: user}
		machineRepo := &mockMachineRepo{
			machines: []*domain.Machine{
				{ID: "m1", UserID: "USR-BATTLE-1"},
				{ID: "m2", UserID: "USR-BATTLE-1"},
				{ID: "m3", UserID: "USR-BATTLE-1"},
				{ID: "m4", UserID: "USR-BATTLE-1"},
				// apenas 4 máquinas, precisa de 5
			},
		}
		referralRepo := &mockReferralRepo{
			referrals: []*domain.Referral{
				{ID: "r1", Status: domain.ReferralStatusAtivo},
				{ID: "r2", Status: domain.ReferralStatusAtivo},
				{ID: "r3", Status: domain.ReferralStatusAtivo},
				{ID: "r4", Status: domain.ReferralStatusAtivo},
				{ID: "r5", Status: domain.ReferralStatusAtivo},
				{ID: "r6", Status: domain.ReferralStatusAtivo},
				{ID: "r7", Status: domain.ReferralStatusAtivo},
				{ID: "r8", Status: domain.ReferralStatusAtivo}, // 8 convidados ok
			},
		}
		txRepo := &mockTxRepo{}
		rankingRepo := &mockRankingRepo{}
		logRepo := &mockLogRepo{}

		uc := garage.NewGarageUseCase(userRepo, machineRepo, txRepo, referralRepo, rankingRepo, logRepo, lvlRepo, nil)

		// Tenta promover
		_, err := uc.LevelUp(ctx, "USR-BATTLE-1")
		if err == nil {
			t.Fatal("esperava erro de requisito pendente de máquinas ativas, mas avançou")
		}

		// Adiciona a 5ª máquina para cumprir 100% das missões
		machineRepo.machines = append(machineRepo.machines, &domain.Machine{ID: "m5", UserID: "USR-BATTLE-1"})

		res, err := uc.LevelUp(ctx, "USR-BATTLE-1")
		if err != nil {
			t.Fatalf("deveria promover com 100%% das missões cumpridas: %v", err)
		}
		if res.NewLevel != 2 {
			t.Errorf("esperado nível 2, obteve %d", res.NewLevel)
		}
	})

	t.Run("Promove para Nível 5 e concede Vale Seguro de 2 dias", func(t *testing.T) {
		user := &domain.User{
			ID:                     "USR-BATTLE-5",
			Nivel:                  4,
			InsuranceVouchersCount: 0,
		}
		userRepo := &mockUserRepo{user: user}
		machineRepo := &mockMachineRepo{
			machines: []*domain.Machine{
				{ID: "m1"}, {ID: "m2"}, {ID: "m3"}, {ID: "m4"}, {ID: "m5"},
			},
		}
		referralRepo := &mockReferralRepo{
			referrals: []*domain.Referral{
				{ID: "r1", Status: domain.ReferralStatusAtivo},
				{ID: "r2", Status: domain.ReferralStatusAtivo},
				{ID: "r3", Status: domain.ReferralStatusAtivo},
				{ID: "r4", Status: domain.ReferralStatusAtivo},
				{ID: "r5", Status: domain.ReferralStatusAtivo},
				{ID: "r6", Status: domain.ReferralStatusAtivo},
				{ID: "r7", Status: domain.ReferralStatusAtivo},
				{ID: "r8", Status: domain.ReferralStatusAtivo},
			},
		}
		uc := garage.NewGarageUseCase(userRepo, machineRepo, &mockTxRepo{}, referralRepo, &mockRankingRepo{}, &mockLogRepo{}, lvlRepo, nil)

		res, err := uc.LevelUp(ctx, "USR-BATTLE-5")
		if err != nil {
			t.Fatalf("falha ao promover para nível 5: %v", err)
		}
		if res.NewLevel != 5 {
			t.Errorf("esperado nível 5, obteve %d", res.NewLevel)
		}
		if user.InsuranceVouchersCount != 1 {
			t.Errorf("esperado 1 vale seguro concedido, obteve %d", user.InsuranceVouchersCount)
		}
	})
}

func TestGarageUseCase_Level12_MaintenanceAndSuspension(t *testing.T) {
	ctx := context.Background()

	lvlRepo := &mockLevelRepo{
		levels: map[int]*domain.LevelConfig{
			11: {
				Level:             11,
				Title:             "Diretor Geral",
				TierName:          "TIER 3",
				CommissionPercent: 9.0,
				BonusYieldPercent: 6.0,
			},
			12: {
				Level:             12,
				Title:             "Comandante Supremo",
				TierName:          "TIER 3",
				RequiredReferrals: 2,
				RequiredMachines:  2,
				RequiredColetas:   5,
				CommissionPercent: 10.0,
				BonusYieldPercent: 7.5,
				SpecialRewardText: "Smartphone + Placa Hall da Fama",
			},
		},
	}

	t.Run("Promove para Nível 12 e inicializa datas de atividade", func(t *testing.T) {
		user := &domain.User{
			ID:               "USR-LVL12",
			Nivel:            11,
			ColetasCompleted: 10,
		}
		userRepo := &mockUserRepo{user: user}
		machineRepo := &mockMachineRepo{
			machines: []*domain.Machine{{ID: "m1"}, {ID: "m2"}},
		}
		referralRepo := &mockReferralRepo{
			referrals: []*domain.Referral{
				{ID: "r1", Status: domain.ReferralStatusAtivo},
				{ID: "r2", Status: domain.ReferralStatusAtivo},
			},
		}
		uc := garage.NewGarageUseCase(userRepo, machineRepo, &mockTxRepo{}, referralRepo, &mockRankingRepo{}, &mockLogRepo{}, lvlRepo, nil)

		res, err := uc.LevelUp(ctx, "USR-LVL12")
		if err != nil {
			t.Fatalf("falha ao promover para nível 12: %v", err)
		}
		if res.NewLevel != 12 {
			t.Errorf("esperado nível 12, obteve %d", res.NewLevel)
		}
		if !user.HasMaxLevelPlaque {
			t.Error("esperado HasMaxLevelPlaque = true")
		}
		if user.Level12ReachedAt == nil {
			t.Error("esperado Level12ReachedAt inicializado")
		}
		if user.Level12BonusSuspended {
			t.Error("esperado Level12BonusSuspended = false no momento da promoção")
		}
	})

	t.Run("Nível 12 com prazo expirado tem bônus suspensos e aviso emitido", func(t *testing.T) {
		pastDate := time.Now().Add(-35 * 24 * time.Hour) // 35 dias atrás (passou dos 30)
		user := &domain.User{
			ID:                              "USR-LVL12-EXPIRED",
			Nivel:                           12,
			CustomReferralCommissionPercent: 10.0,
			LastReferralCreatedAt:           &pastDate,
			Level12ReachedAt:                &pastDate,
			Level12BonusSuspended:           false,
		}
		userRepo := &mockUserRepo{user: user}
		uc := garage.NewGarageUseCase(userRepo, &mockMachineRepo{}, &mockTxRepo{}, &mockReferralRepo{}, &mockRankingRepo{}, &mockLogRepo{}, lvlRepo, nil)

		status, err := uc.GetBattlePassStatus(ctx, "USR-LVL12-EXPIRED")
		if err != nil {
			t.Fatalf("erro ao obter status de carreira: %v", err)
		}

		if !status.IsLevel12 {
			t.Error("esperado IsLevel12 = true")
		}
		if !status.Level12BonusSuspended {
			t.Error("esperado Level12BonusSuspended = true após 35 dias sem convidados")
		}
		if !status.Level12WarningActive {
			t.Error("esperado Level12WarningActive = true")
		}
		if status.CommissionPercent != 5.0 {
			t.Errorf("comissão deveria ser rebaixada para 5.0%% base quando suspenso, obteve %.1f%%", status.CommissionPercent)
		}
		if status.BonusYieldPercent != 0.0 {
			t.Errorf("bônus de rendimento deveria ser 0.0%% quando suspenso, obteve %.1f%%", status.BonusYieldPercent)
		}
	})

	t.Run("Nível 12 ativo dentro do prazo mantém 10% de comissão e bônus de produção", func(t *testing.T) {
		recentDate := time.Now().Add(-5 * 24 * time.Hour) // 5 dias atrás (restam 25)
		user := &domain.User{
			ID:                              "USR-LVL12-ACTIVE",
			Nivel:                           12,
			CustomReferralCommissionPercent: 10.0,
			LastReferralCreatedAt:           &recentDate,
			Level12ReachedAt:                &recentDate,
			Level12BonusSuspended:           false,
		}
		userRepo := &mockUserRepo{user: user}
		uc := garage.NewGarageUseCase(userRepo, &mockMachineRepo{}, &mockTxRepo{}, &mockReferralRepo{}, &mockRankingRepo{}, &mockLogRepo{}, lvlRepo, nil)

		status, err := uc.GetBattlePassStatus(ctx, "USR-LVL12-ACTIVE")
		if err != nil {
			t.Fatalf("erro ao obter status: %v", err)
		}

		if status.Level12BonusSuspended {
			t.Error("não deveria estar suspenso dentro do prazo de 30 dias")
		}
		if status.CommissionPercent != 10.0 {
			t.Errorf("esperado 10.0%% de comissão, obteve %.1f%%", status.CommissionPercent)
		}
		if status.BonusYieldPercent != 7.5 {
			t.Errorf("esperado 7.5%% de bônus de produção, obteve %.1f%%", status.BonusYieldPercent)
		}
	})
}

