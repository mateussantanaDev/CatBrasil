package shop

import (
	"context"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/id"
)

type RentResponse struct {
	Message    string          `json:"message"`
	NewMachine *domain.Machine `json:"newMachine"`
	NewBalance float64         `json:"balance"`
	XPGained   int             `json:"xpGained"`
	User       *domain.User    `json:"user"`
}

type ShopUseCase struct {
	userRepo    domain.UserRepository
	catalogRepo domain.CatalogRepository
	machineRepo domain.MachineRepository
	txRepo      domain.TransactionRepository
}

func NewShopUseCase(
	userRepo domain.UserRepository,
	catalogRepo domain.CatalogRepository,
	machineRepo domain.MachineRepository,
	txRepo domain.TransactionRepository,
) *ShopUseCase {
	return &ShopUseCase{
		userRepo:    userRepo,
		catalogRepo: catalogRepo,
		machineRepo: machineRepo,
		txRepo:      txRepo,
	}
}

// GetCatalog lista todas as frotas ativas com cálculo de rendimento
func (uc *ShopUseCase) GetCatalog(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return uc.catalogRepo.GetActive(ctx)
}

// RentMachine processa o aluguel de frotas com validação de saldo, rollover e linha Reinvest
func (uc *ShopUseCase) RentMachine(ctx context.Context, userID, catalogID string, requestKey ...string) (*RentResponse, error) {
	if financial, ok := uc.txRepo.(interface {
		RentMachineAtomic(context.Context, string, string, string) (*domain.Machine, *domain.User, int, error)
	}); ok {
		key := ""
		if len(requestKey) > 0 {
			key = requestKey[0]
		}
		machine, user, xp, err := financial.RentMachineAtomic(ctx, userID, catalogID, key)
		if err != nil {
			return nil, err
		}
		return &RentResponse{Message: fmt.Sprintf("%s contratado com sucesso!", machine.Name), NewMachine: machine, NewBalance: user.Balance, XPGained: xp, User: user}, nil
	}
	item, err := uc.catalogRepo.GetByID(ctx, catalogID)
	if err != nil || item == nil {
		return nil, domain.ErrMachineCatalogNotFound
	}

	if !item.Active {
		return nil, domain.ErrCatalogItemInactive
	}

	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	// Regra 1: Validação de Linha Reinvest Turbinado (exige saldo acumulado de coletas na plataforma)
	if item.ReinvestOnly {
		if user.YieldBalanceAvailable < item.Price {
			return nil, domain.ErrInsufficientYieldBalance
		}
	}

	// Regra 2: Saldo em carteira suficiente
	if user.Balance < item.Price {
		return nil, domain.ErrInsufficientBalance
	}

	// Regra 3: Limite máximo de compras por operador
	existingMachines, err := uc.machineRepo.GetByUserID(ctx, userID)
	if err == nil {
		count := 0
		for _, m := range existingMachines {
			if m.Name == item.Name {
				count++
			}
		}
		if item.MaxPurchasesPerUser > 0 && count >= item.MaxPurchasesPerUser {
			return nil, domain.ErrMaxPurchasesExceeded
		}
	}

	// Débito e atualizações financeiras
	newBalance := user.Balance - item.Price
	newCurrentRollover := user.CurrentRollover + item.Price
	newYieldAvail := user.YieldBalanceAvailable
	if item.ReinvestOnly {
		newYieldAvail = user.YieldBalanceAvailable - item.Price
		if newYieldAvail < 0 {
			newYieldAvail = 0
		}
	}

	xpGained := int(item.Price * 0.02)
	tratorAtivo := user.TratorRented
	if catalogID == "trator_d8t" || item.Category == "PESADA" {
		tratorAtivo = true
	}

	machID, err := id.New("mach")
	if err != nil {
		return nil, err
	}
	txID, err := id.New("tx")
	if err != nil {
		return nil, err
	}

	user.Balance = newBalance
	user.CurrentRollover = newCurrentRollover
	user.YieldBalanceAvailable = newYieldAvail
	user.XP += xpGained
	user.TratorRented = tratorAtivo

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("erro ao atualizar saldo do operador: %w", err)
	}

	now := time.Now()
	newMachine := &domain.Machine{
		ID:              machID,
		UserID:          userID,
		Name:            item.Name,
		Cost:            item.Price,
		YieldAmount:     item.DailyYield,
		LastCollectedAt: now,
		State:           domain.MachineStateAguardando,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.machineRepo.Create(ctx, newMachine); err != nil {
		return nil, fmt.Errorf("erro ao criar máquina na garagem: %w", err)
	}

	// Registra transação de débito
	tx := &domain.Transaction{
		ID:        txID,
		UserID:    userID,
		Tipo:      fmt.Sprintf("Aluguel %s", item.Name),
		Valor:     -item.Price,
		Status:    domain.TxStatusSucesso,
		CreatedAt: now,
		PaidAt:    &now,
	}
	_ = uc.txRepo.Create(ctx, tx)

	return &RentResponse{
		Message:    fmt.Sprintf("%s contratado com sucesso!", item.Name),
		NewMachine: newMachine,
		NewBalance: newBalance,
		XPGained:   xpGained,
		User:       user,
	}, nil
}
