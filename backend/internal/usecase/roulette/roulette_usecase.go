package roulette

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/id"
)

type SpinResponse struct {
	SectorIdx   int                   `json:"sectorIdx"`
	WinningSlot *domain.RouletteSlice `json:"win"`
	WonPartName string                `json:"wonPartName,omitempty"`
	PrizeAmount float64               `json:"prizeAmount"`
	User        *domain.User          `json:"user"`
	Message     string                `json:"message"`
}

type AssembleResponse struct {
	Message    string          `json:"message"`
	NewMachine *domain.Machine `json:"newMachine"`
	User       *domain.User    `json:"user"`
}

type RouletteUseCase struct {
	userRepo     domain.UserRepository
	rouletteRepo domain.RouletteRepository
	machineRepo  domain.MachineRepository
	txRepo       domain.TransactionRepository
	logRepo      domain.LogRepository
}

func NewRouletteUseCase(
	userRepo domain.UserRepository,
	rouletteRepo domain.RouletteRepository,
	machineRepo domain.MachineRepository,
	txRepo domain.TransactionRepository,
	logRepo domain.LogRepository,
) *RouletteUseCase {
	return &RouletteUseCase{
		userRepo:     userRepo,
		rouletteRepo: rouletteRepo,
		machineRepo:  machineRepo,
		txRepo:       txRepo,
		logRepo:      logRepo,
	}
}

// GetConfig retorna todas as fatias da roleta
func (uc *RouletteUseCase) GetConfig(ctx context.Context) ([]*domain.RouletteSlice, error) {
	return uc.rouletteRepo.GetSlices(ctx)
}

// Spin executa o sorteio ponderado com precisão e complexidade O(1)
func (uc *RouletteUseCase) Spin(ctx context.Context, userID string) (*SpinResponse, error) {
	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	todayRecife := time.Now().In(time.FixedZone("America/Recife", -3*3600)).Format("2006-01-02")
	if user.HasSpunToday(todayRecife) {
		return nil, domain.ErrSpinAlreadyUsedToday
	}

	slices, err := uc.rouletteRepo.GetSlices(ctx)
	if err != nil || len(slices) == 0 {
		return nil, domain.ErrInvalidRouletteSlice
	}

	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return nil, fmt.Errorf("erro no gerador de números aleatórios: %w", err)
	}
	roll := float64(n.Int64()) / 100.0 // 0.00 a 99.99

	var cumulative float64
	var winningSlice *domain.RouletteSlice
	var sectorIdx int

	for i, s := range slices {
		cumulative += s.Probability
		if roll <= cumulative {
			winningSlice = s
			sectorIdx = i
			break
		}
	}

	if winningSlice == nil {
		winningSlice = slices[0]
		sectorIdx = 0
	}

	var wonPartName string
	labelLower := strings.ToLower(winningSlice.Label)

	if strings.Contains(labelLower, "motor") {
		user.MotorC15Count++
		wonPartName = "Motor C15 Heavy Duty"
	} else if strings.Contains(labelLower, "esteira") {
		user.EsteiraReforcadaCount++
		wonPartName = "Esteira Reforçada CAT"
	} else if strings.Contains(labelLower, "filtro") {
		user.FiltroOleoCount++
		wonPartName = "Filtro de Óleo Alta Vazão"
	} else if strings.Contains(labelLower, "turbo") {
		user.TurboGarrettCount++
		wonPartName = "Turbo Garrett T66"
	} else if strings.Contains(labelLower, "caixa") {
		pRand, _ := rand.Int(rand.Reader, big.NewInt(4))
		switch pRand.Int64() {
		case 0:
			user.MotorC15Count++
			wonPartName = "Motor C15 (Caixa Misteriosa)"
		case 1:
			user.EsteiraReforcadaCount++
			wonPartName = "Esteira Reforçada (Caixa Misteriosa)"
		case 2:
			user.FiltroOleoCount++
			wonPartName = "Filtro de Óleo (Caixa Misteriosa)"
		default:
			user.TurboGarrettCount++
			wonPartName = "Turbo Garrett (Caixa Misteriosa)"
		}
	}

	if winningSlice.Amount > 0 {
		user.Balance += winningSlice.Amount
	}

	user.AlreadySpunToday = true
	if financial, ok := uc.txRepo.(interface {
		ApplySpin(context.Context, string, string, string, float64) (*domain.User, error)
	}); ok {
		committed, err := financial.ApplySpin(ctx, userID, winningSlice.Label, wonPartName, winningSlice.Amount)
		if err != nil {
			return nil, err
		}
		if uc.logRepo != nil && (winningSlice.Amount > 0 || wonPartName != "") {
			_ = uc.logRepo.LogUsage(ctx, "roulette_win", winningSlice.Amount)
		}
		return &SpinResponse{SectorIdx: sectorIdx, WinningSlot: winningSlice, WonPartName: wonPartName, PrizeAmount: winningSlice.Amount, User: committed, Message: fmt.Sprintf("Parabéns! Prêmio conquistado: %s", winningSlice.Label)}, nil
	}

	var txID string
	if winningSlice.Amount > 0 || wonPartName != "" {
		var err error
		txID, err = id.New("tx")
		if err != nil {
			return nil, err
		}
	}

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("erro ao registrar prêmio da roleta: %w", err)
	}

	now := time.Now()
	if winningSlice.Amount > 0 || wonPartName != "" {
		tx := &domain.Transaction{
			ID:        txID,
			UserID:    userID,
			Tipo:      fmt.Sprintf("Giro Roleta: %s", winningSlice.Label),
			Valor:     winningSlice.Amount,
			Status:    domain.TxStatusSucesso,
			CreatedAt: now,
			PaidAt:    &now,
		}
		_ = uc.txRepo.Create(ctx, tx)
		_ = uc.logRepo.LogUsage(ctx, "roulette_win", winningSlice.Amount)
	}

	return &SpinResponse{
		SectorIdx:   sectorIdx,
		WinningSlot: winningSlice,
		WonPartName: wonPartName,
		PrizeAmount: winningSlice.Amount,
		User:        user,
		Message:     fmt.Sprintf("Parabéns! Prêmio conquistado: %s", winningSlice.Label),
	}, nil
}

// AssembleParts monta 1x Escavadeira CAT 320 completa consumindo 4 peças Caterpillar
func (uc *RouletteUseCase) AssembleParts(ctx context.Context, userID string) (*AssembleResponse, error) {
	if financial, ok := uc.txRepo.(interface {
		AssemblePartsAtomic(context.Context, string) (*domain.User, *domain.Machine, error)
	}); ok {
		user, machine, err := financial.AssemblePartsAtomic(ctx, userID)
		if err != nil {
			return nil, err
		}
		if uc.logRepo != nil {
			_ = uc.logRepo.LogSecurity(ctx, &userID, "MACHINE_ASSEMBLED_FROM_PARTS", user.IPAddress, user.DeviceFingerprint, "")
		}
		return &AssembleResponse{Message: "🎉 Parabéns! Você montou 1x Escavadeira CAT 320 na Oficina de Peças!", NewMachine: machine, User: user}, nil
	}
	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	if !user.ConsumePartsForAssembly() {
		return nil, domain.ErrIncompletePartsAssembly
	}

	machID, err := id.New("mach-parts")
	if err != nil {
		return nil, err
	}

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	now := time.Now()
	newMachine := &domain.Machine{
		ID:              machID,
		UserID:          userID,
		Name:            "Escavadeira CAT 320 (Oficina de Peças) 🧩",
		Cost:            2500.0,
		YieldAmount:     110.0,
		LastCollectedAt: now,
		State:           domain.MachineStateAguardando,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.machineRepo.Create(ctx, newMachine); err != nil {
		return nil, fmt.Errorf("erro ao criar máquina na garagem: %w", err)
	}

	_ = uc.logRepo.LogSecurity(ctx, &userID, "MACHINE_ASSEMBLED_FROM_PARTS", user.IPAddress, user.DeviceFingerprint, "")

	return &AssembleResponse{
		Message:    "🎉 Parabéns! Você montou 1x Escavadeira CAT 320 na Oficina de Peças!",
		NewMachine: newMachine,
		User:       user,
	}, nil
}

// ResetDailySpins zera a trava de giro diário para todos os usuários
func (uc *RouletteUseCase) ResetDailySpins(ctx context.Context) error {
	return uc.userRepo.ResetDailySpins(ctx)
}

// ResetUserSpin reseta a flag AlreadySpunToday e o last_spin_date de um operador específico
func (uc *RouletteUseCase) ResetUserSpin(ctx context.Context, userID string) error {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return domain.ErrUserNotFound
	}
	user.AlreadySpunToday = false
	user.LastSpinDate = ""
	user.LastSpinAt = nil
	return uc.userRepo.Update(ctx, user)
}
