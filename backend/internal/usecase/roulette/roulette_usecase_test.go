package roulette_test

import (
	"context"
	"testing"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/roulette"
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
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error {
	m.user.AlreadySpunToday = false
	return nil
}

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

type mockMachineRepo struct {
	created []*domain.Machine
}

func (m *mockMachineRepo) GetByID(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) GetByUserID(ctx context.Context, userID string) ([]*domain.Machine, error) {
	return m.created, nil
}
func (m *mockMachineRepo) Create(ctx context.Context, machine *domain.Machine) error {
	m.created = append(m.created, machine)
	return nil
}
func (m *mockMachineRepo) Update(ctx context.Context, machine *domain.Machine) error { return nil }
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error               { return nil }

type mockTxRepo struct{}

func (m *mockTxRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByIDForUpdate(ctx context.Context, id string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetByUserID(ctx context.Context, userID, filter string) ([]*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) GetAll(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return nil, 0, nil
}
func (m *mockTxRepo) Create(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.Transaction) error { return nil }
func (m *mockTxRepo) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	return nil, nil
}
func (m *mockTxRepo) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	return nil, nil
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

func TestRouletteUseCase_Spin(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:               "USR-1",
		Username:         "mateus",
		Balance:          100.0,
		AlreadySpunToday: false,
	}
	slices := []*domain.RouletteSlice{
		{ID: 1, Label: "🧩 Motor C15", Amount: 0, Probability: 25.0},
		{ID: 2, Label: "🧩 Esteira CAT", Amount: 0, Probability: 25.0},
		{ID: 3, Label: "🧩 Filtro Óleo", Amount: 0, Probability: 25.0},
		{ID: 4, Label: "🧩 Turbo Garrett", Amount: 0, Probability: 25.0},
	}

	userRepo := &mockUserRepo{user: user}
	rouletteRepo := &mockRouletteRepo{slices: slices}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{}
	logRepo := &mockLogRepo{}

	uc := roulette.NewRouletteUseCase(userRepo, rouletteRepo, machineRepo, txRepo, logRepo)

	res, err := uc.Spin(ctx, "USR-1")
	if err != nil {
		t.Fatalf("falha no giro da roleta: %v", err)
	}

	if !user.AlreadySpunToday {
		t.Errorf("esperado alreadySpunToday = true")
	}

	if res.WonPartName == "" && res.PrizeAmount == 0 {
		t.Errorf("esperado ganho de peça ou valor monetário")
	}

	// Segundo giro no mesmo dia deve falhar
	_, err2 := uc.Spin(ctx, "USR-1")
	if err2 != domain.ErrSpinAlreadyUsedToday {
		t.Errorf("esperado ErrSpinAlreadyUsedToday, obteve: %v", err2)
	}
}

func TestRouletteUseCase_AssembleParts(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:                    "USR-1",
		Username:              "mateus",
		MotorC15Count:         1,
		EsteiraReforcadaCount: 1,
		FiltroOleoCount:       1,
		TurboGarrettCount:     1,
	}

	userRepo := &mockUserRepo{user: user}
	rouletteRepo := &mockRouletteRepo{}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{}
	logRepo := &mockLogRepo{}

	uc := roulette.NewRouletteUseCase(userRepo, rouletteRepo, machineRepo, txRepo, logRepo)

	res, err := uc.AssembleParts(ctx, "USR-1")
	if err != nil {
		t.Fatalf("falha ao montar máquina na oficina: %v", err)
	}

	if len(machineRepo.created) != 1 {
		t.Fatalf("esperado 1 máquina criada, obteve: %d", len(machineRepo.created))
	}

	if user.MotorC15Count != 0 || user.EsteiraReforcadaCount != 0 || user.FiltroOleoCount != 0 || user.TurboGarrettCount != 0 {
		t.Errorf("esperado consumo de 1 unidade de cada peça")
	}

	if res.NewMachine.Cost != 2500.0 || res.NewMachine.YieldAmount != 110.0 {
		t.Errorf("propriedades inválidas da máquina montada")
	}
}

func TestRouletteUseCase_ResetUserSpin(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:               "USR-RESET",
		Username:         "operador_reset",
		AlreadySpunToday: true,
		LastSpinDate:     "2026-10-19",
	}

	userRepo := &mockUserRepo{user: user}
	uc := roulette.NewRouletteUseCase(userRepo, nil, nil, nil, nil)

	err := uc.ResetUserSpin(ctx, "USR-RESET")
	if err != nil {
		t.Fatalf("falha ao resetar giro do usuário: %v", err)
	}

	if user.AlreadySpunToday {
		t.Fatal("esperava AlreadySpunToday = false")
	}
	if user.LastSpinDate != "" {
		t.Fatalf("esperava LastSpinDate limpo, obteve: %s", user.LastSpinDate)
	}
	if user.LastSpinAt != nil {
		t.Fatal("esperava LastSpinAt limpo (nil)")
	}
}
