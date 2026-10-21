package shop_test

import (
	"context"
	"testing"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/shop"
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
	return nil, nil
}
func (m *mockUserRepo) List(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return nil, 0, nil
}
func (m *mockUserRepo) Count(ctx context.Context) (int64, error) { return 1, nil }
func (m *mockUserRepo) ResetDailySpins(ctx context.Context) error { return nil }

type mockCatalogRepo struct {
	items map[string]*domain.FleetCatalogItem
}

func (m *mockCatalogRepo) GetAll(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	var list []*domain.FleetCatalogItem
	for _, it := range m.items {
		list = append(list, it)
	}
	return list, nil
}
func (m *mockCatalogRepo) GetActive(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	var list []*domain.FleetCatalogItem
	for _, it := range m.items {
		if it.Active {
			list = append(list, it)
		}
	}
	return list, nil
}
func (m *mockCatalogRepo) GetByID(ctx context.Context, id string) (*domain.FleetCatalogItem, error) {
	return m.items[id], nil
}
func (m *mockCatalogRepo) Upsert(ctx context.Context, item *domain.FleetCatalogItem) error {
	m.items[item.ID] = item
	return nil
}
func (m *mockCatalogRepo) SaveCatalog(ctx context.Context, items []*domain.FleetCatalogItem) error {
	return nil
}
func (m *mockCatalogRepo) Delete(ctx context.Context, id string) error {
	delete(m.items, id)
	return nil
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

func TestShopUseCase_RentMachine_Success(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:                    "USR-1",
		Username:              "mateus",
		Balance:               5000.0,
		YieldBalanceAvailable: 2000.0,
	}
	item := &domain.FleetCatalogItem{
		ID:                  "cat_320",
		Name:                "Escavadeira CAT 320",
		Price:               2500.0,
		DailyYield:          110.0,
		Active:              true,
		MaxPurchasesPerUser: 3,
	}

	userRepo := &mockUserRepo{user: user}
	catalogRepo := &mockCatalogRepo{items: map[string]*domain.FleetCatalogItem{"cat_320": item}}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{}

	uc := shop.NewShopUseCase(userRepo, catalogRepo, machineRepo, txRepo)

	res, err := uc.RentMachine(ctx, "USR-1", "cat_320")
	if err != nil {
		t.Fatalf("falha ao alugar máquina: %v", err)
	}

	if res.NewBalance != 2500.0 {
		t.Errorf("esperado novo saldo R$ 2500.00, obteve: R$ %.2f", res.NewBalance)
	}

	if user.CurrentRollover != 2500.0 {
		t.Errorf("esperado acréscimo de R$ 2500.00 no rollover")
	}

	if len(machineRepo.created) != 1 {
		t.Errorf("esperado 1 máquina criada na garagem")
	}
}

func TestShopUseCase_RentMachine_ReinvestOnlyInsufficient(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{
		ID:                    "USR-1",
		Username:              "mateus",
		Balance:               1000.0,
		YieldBalanceAvailable: 100.0, // Apenas R$ 100 de coletas
	}
	item := &domain.FleetCatalogItem{
		ID:           "reinvest_turbo",
		Name:         "Linha Reinvest Turbo",
		Price:        500.0,
		ReinvestOnly: true,
		Active:       true,
	}

	userRepo := &mockUserRepo{user: user}
	catalogRepo := &mockCatalogRepo{items: map[string]*domain.FleetCatalogItem{"reinvest_turbo": item}}
	machineRepo := &mockMachineRepo{}
	txRepo := &mockTxRepo{}

	uc := shop.NewShopUseCase(userRepo, catalogRepo, machineRepo, txRepo)

	_, err := uc.RentMachine(ctx, "USR-1", "reinvest_turbo")
	if err != domain.ErrInsufficientYieldBalance {
		t.Errorf("esperado ErrInsufficientYieldBalance, obteve: %v", err)
	}
}
