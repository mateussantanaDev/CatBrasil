package garage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/pkg/id"
)

type CollectResponse struct {
	Message          string              `json:"message"`
	CollectedAmount  float64             `json:"collectedAmount"`
	NewBalance       float64             `json:"newBalance"`
	ColetasCompleted int                 `json:"coletasCompleted"`
	State            domain.MachineState `json:"state"`
	User             *domain.User        `json:"user"`
}

type LevelUpResponse struct {
	Message  string       `json:"message"`
	NewLevel int          `json:"newLevel"`
	User     *domain.User `json:"user"`
}

type InsuranceRecoverResponse struct {
	Message           string  `json:"message"`
	RecoveredAmount   float64 `json:"recoveredAmount"`
	NewBalance        float64 `json:"newBalance"`
	RemainingVouchers int     `json:"remainingVouchers"`
}

type GarageUseCase struct {
	userRepo     domain.UserRepository
	machineRepo  domain.MachineRepository
	txRepo       domain.TransactionRepository
	referralRepo domain.ReferralRepository
	rankingRepo  domain.RankingRepository
	logRepo      domain.LogRepository
	levelRepo    domain.LevelRepository
	configRepo   domain.ConfigRepository
}

func NewGarageUseCase(
	userRepo domain.UserRepository,
	machineRepo domain.MachineRepository,
	txRepo domain.TransactionRepository,
	referralRepo domain.ReferralRepository,
	rankingRepo domain.RankingRepository,
	logRepo domain.LogRepository,
	levelRepo domain.LevelRepository,
	configRepo domain.ConfigRepository,
) *GarageUseCase {
	return &GarageUseCase{
		userRepo:     userRepo,
		machineRepo:  machineRepo,
		txRepo:       txRepo,
		referralRepo: referralRepo,
		rankingRepo:  rankingRepo,
		logRepo:      logRepo,
		levelRepo:    levelRepo,
		configRepo:   configRepo,
	}
}

// GetUserMachines retorna todas as máquinas do operador com os estados e tempos recalculados
func (uc *GarageUseCase) GetUserMachines(ctx context.Context, userID string) ([]*domain.Machine, error) {
	machines, err := uc.machineRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for _, m := range machines {
		state, secs := m.CalculateState(now, 10)
		m.State = state
		m.SecondsRemaining = secs
	}

	return machines, nil
}

// CollectYield processa a coleta diária com validação estrita da tolerância de 10 min e bônus de nível
func (uc *GarageUseCase) CollectYield(ctx context.Context, userID, machineID string) (*CollectResponse, error) {
	if financial, ok := uc.txRepo.(interface {
		CollectYieldAtomic(context.Context, string, string) (*domain.User, float64, error)
	}); ok {
		user, amount, err := financial.CollectYieldAtomic(ctx, userID, machineID)
		if err != nil {
			return nil, err
		}
		if uc.logRepo != nil {
			_ = uc.logRepo.LogUsage(ctx, "COLLECT_SUCCESS", amount)
		}
		return &CollectResponse{Message: fmt.Sprintf("Coleta de R$ %.2f concluída com sucesso!", amount), CollectedAmount: amount, NewBalance: user.Balance, ColetasCompleted: user.ColetasCompleted, State: domain.MachineStateAguardando, User: user}, nil
	}
	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	machine, err := uc.machineRepo.GetByID(ctx, machineID)
	if err != nil || machine == nil {
		return nil, domain.ErrMachineNotFound
	}

	if machine.UserID != userID {
		return nil, domain.ErrMachineNotFound
	}

	now := time.Now()
	state, _ := machine.CalculateState(now, 10)

	if state == domain.MachineStateAguardando {
		return nil, domain.ErrYieldNotReady
	}

	if state == domain.MachineStateExpirado {
		machine.ResetCycle(now)
		_ = uc.machineRepo.Update(ctx, machine)
		if uc.logRepo != nil {
			_ = uc.logRepo.LogUsage(ctx, "EXPIRED_YIELD_RETAINED", machine.YieldAmount)
		}
		return nil, domain.ErrYieldExpired
	}

	// Bônus passivo de rendimento diário de acordo com a configuração do nível
	yieldMultiplier := 1.0
	if uc.levelRepo != nil {
		if lvlCfg, err := uc.levelRepo.GetByLevel(ctx, user.Nivel); err == nil && lvlCfg != nil {
			bonusPercent := lvlCfg.BonusYieldPercent
			// Se o operador for Nível 12 e estiver com bônus suspenso por inatividade de convites, desativa o bônus extra
			if user.Nivel >= 12 && user.Level12BonusSuspended {
				bonusPercent = 0.0
			}
			yieldMultiplier = 1.0 + (bonusPercent / 100.0)
		}
	} else {
		yieldMultiplier = 1.0 + float64(user.Nivel-1)*0.05
	}

	yieldAmount := math.Round(machine.YieldAmount*yieldMultiplier*100) / 100
	newBalance := math.Round((user.Balance+yieldAmount)*100) / 100
	novasColetas := user.ColetasCompleted + 1

	txID, err := id.New("tx")
	if err != nil {
		return nil, err
	}

	user.Balance = newBalance
	user.YieldBalanceAvailable = math.Round((user.YieldBalanceAvailable+yieldAmount)*100) / 100
	user.ColetasCompleted = novasColetas
	user.XP += 25

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	machine.ResetCycle(now)
	if err := uc.machineRepo.Update(ctx, machine); err != nil {
		return nil, err
	}

	tx := &domain.Transaction{
		ID:        txID,
		UserID:    userID,
		Tipo:      fmt.Sprintf("Rendimento [%s]", machine.Name),
		Valor:     yieldAmount,
		Status:    domain.TxStatusSucesso,
		CreatedAt: now,
		PaidAt:    &now,
	}
	_ = uc.txRepo.Create(ctx, tx)

	if uc.rankingRepo != nil {
		_ = uc.rankingRepo.AddOrUpdateWeeklyTonnage(ctx, user.ID, user.Username, yieldAmount*10)
	}

	if uc.logRepo != nil {
		_ = uc.logRepo.LogUsage(ctx, "COLLECT_SUCCESS", yieldAmount)
	}

	return &CollectResponse{
		Message:          fmt.Sprintf("Coleta de R$ %.2f concluída com sucesso!", yieldAmount),
		CollectedAmount:  yieldAmount,
		NewBalance:       newBalance,
		ColetasCompleted: novasColetas,
		State:            domain.MachineStateAguardando,
		User:             user,
	}, nil
}

// GetBattlePassStatus retorna o progresso das missões do operador e o status da temporada
func (uc *GarageUseCase) GetBattlePassStatus(ctx context.Context, userID string) (*domain.BattlePassStatus, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	var allLevels []*domain.LevelConfig
	if uc.levelRepo != nil {
		allLevels, err = uc.levelRepo.GetAll(ctx)
		if err != nil {
			return nil, err
		}
	}

	seasonStartedAt := time.Now()
	durationDays := 75
	seasonNum := 1
	if uc.configRepo != nil {
		cfg, _ := uc.configRepo.GetConfig(ctx)
		if cfg != nil {
			seasonStartedAt = cfg.SeasonStartedAt
			durationDays = cfg.SeasonDurationDays
			if durationDays <= 0 {
				durationDays = 75
			}
			seasonNum = cfg.CurrentSeasonNumber
			if seasonNum <= 0 {
				seasonNum = 1
			}
		}
	}

	seasonEndsAt := seasonStartedAt.Add(time.Duration(durationDays) * 24 * time.Hour)
	daysRemaining := int(time.Until(seasonEndsAt).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	activeRefs := 0
	if uc.referralRepo != nil {
		activeRefs, _ = uc.referralRepo.CountActiveByReferrerID(ctx, userID)
	}

	machines, _ := uc.machineRepo.GetByUserID(ctx, userID)
	activeMachines := len(machines)

	tonnage := 0.0
	if uc.rankingRepo != nil {
		userRank, _ := uc.rankingRepo.GetUserRank(ctx, userID)
		if userRank != nil {
			tonnage = userRank.WeeklyTonnage
		}
	}

	nextLevelNum := user.Nivel + 1
	var nextLevelConfig *domain.LevelConfig
	var missingReqs []string
	canLevelUp := false

	if nextLevelNum <= 12 {
		for _, l := range allLevels {
			if l.Level == nextLevelNum {
				nextLevelConfig = l
				break
			}
		}

		if nextLevelConfig != nil {
			if activeRefs < nextLevelConfig.RequiredReferrals {
				missingReqs = append(missingReqs, fmt.Sprintf("Convidar mais %d pessoas (atual: %d / %d)", nextLevelConfig.RequiredReferrals-activeRefs, activeRefs, nextLevelConfig.RequiredReferrals))
			}
			if activeMachines < nextLevelConfig.RequiredMachines {
				missingReqs = append(missingReqs, fmt.Sprintf("Possuir pelo menos %d máquinas ativas (atual: %d / %d)", nextLevelConfig.RequiredMachines, activeMachines, nextLevelConfig.RequiredMachines))
			}
			if user.ColetasCompleted < nextLevelConfig.RequiredColetas {
				missingReqs = append(missingReqs, fmt.Sprintf("Realizar mais %d coletas manuais (atual: %d / %d)", nextLevelConfig.RequiredColetas-user.ColetasCompleted, user.ColetasCompleted, nextLevelConfig.RequiredColetas))
			}
			if tonnage < nextLevelConfig.RequiredTonnage {
				missingReqs = append(missingReqs, fmt.Sprintf("Acumular mais %.0ft de mineração (atual: %.0ft / %.0ft)", nextLevelConfig.RequiredTonnage-tonnage, tonnage, nextLevelConfig.RequiredTonnage))
			}
			if len(missingReqs) == 0 {
				canLevelUp = true
			}
		}
	}

	// Lógica de Manutenção do Nível 12 (Nível Máximo)
	isLevel12 := user.Nivel >= 12
	level12DaysRemaining := 0
	level12WarningActive := false
	level12WarningMessage := ""
	level12Suspended := user.Level12BonusSuspended

	if isLevel12 {
		maintenanceDays := 30
		if uc.configRepo != nil {
			if cfg, err := uc.configRepo.GetConfig(ctx); err == nil && cfg != nil && cfg.Level12MaintenanceDays > 0 {
				maintenanceDays = cfg.Level12MaintenanceDays
			}
		}

		refDate := time.Now()
		if user.LastReferralCreatedAt != nil {
			refDate = *user.LastReferralCreatedAt
		} else if user.Level12ReachedAt != nil {
			refDate = *user.Level12ReachedAt
		}

		deadline := refDate.Add(time.Duration(maintenanceDays) * 24 * time.Hour)
		diff := time.Until(deadline)
		daysLeft := int(diff.Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
		level12DaysRemaining = daysLeft

		if time.Now().After(deadline) {
			// Prazo expirado! Suspende os bônus excedentes
			level12Suspended = true
			if !user.Level12BonusSuspended {
				user.Level12BonusSuspended = true
				_ = uc.userRepo.Update(ctx, user)
			}
			level12WarningActive = true
			level12WarningMessage = "⚠️ SEUS SUPER BÔNUS DE NÍVEL 12 ESTÃO TEMPORARIAMENTE PAUSADOS: Convide pelo menos 1 novo operador para sua rede para reativar imediatamente sua comissão de 10% e bônus de produção!"
		} else if daysLeft <= 7 {
			// Alerta prévio (últimos 7 dias de tolerância)
			level12WarningActive = true
			level12WarningMessage = fmt.Sprintf("⚠️ ATENÇÃO OPERADOR NÍVEL 12: Faltam apenas %d dia(s) de atividade garantida. Convide um novo operador para manter seus super bônus de 10%% ativos!", daysLeft)
		} else {
			level12WarningMessage = fmt.Sprintf("👑 OPERADOR SUPREMO ATIVO: Você tem %d dias restantes de bônus garantidos no Nível 12.", daysLeft)
		}
	}

	currentYieldBonus := 0.0
	currentCommission := user.CustomReferralCommissionPercent
	if currentCommission <= 0 {
		currentCommission = 5.0
	}
	for _, l := range allLevels {
		if l.Level == user.Nivel {
			currentYieldBonus = l.BonusYieldPercent
			break
		}
	}

	if isLevel12 && level12Suspended {
		currentCommission = 5.0
		currentYieldBonus = 0.0
	}

	maxMachines := user.MaxMachinesAllowed
	if maxMachines <= 0 {
		maxMachines = 15
	}

	return &domain.BattlePassStatus{
		CurrentLevel:                    user.Nivel,
		CurrentSeasonNumber:             seasonNum,
		SeasonDurationDays:              durationDays,
		SeasonDaysRemaining:             daysRemaining,
		SeasonStartedAt:                 seasonStartedAt,
		SeasonEndsAt:                    seasonEndsAt,
		CurrentReferrals:                activeRefs,
		CurrentActiveMachines:           activeMachines,
		CurrentColetas:                  user.ColetasCompleted,
		CurrentTonnage:                  tonnage,
		InsuranceVouchers:               user.InsuranceVouchersCount,
		MaxMachinesAllowed:              maxMachines,
		CommissionPercent:               currentCommission,
		BonusYieldPercent:               currentYieldBonus,
		HasMaxLevelPlaque:               user.HasMaxLevelPlaque,
		CanLevelUp:                      canLevelUp,
		MissingRequirements:             missingReqs,
		NextLevelConfig:                 nextLevelConfig,
		AllLevels:                       allLevels,
		IsLevel12:                       isLevel12,
		Level12MaintenanceDaysRemaining: level12DaysRemaining,
		Level12BonusSuspended:           level12Suspended,
		Level12WarningActive:            level12WarningActive,
		Level12WarningMessage:           level12WarningMessage,
	}, nil
}

// LevelUp processa a promoção estrita de nível de carreira do operador
func (uc *GarageUseCase) LevelUp(ctx context.Context, userID string) (*LevelUpResponse, error) {
	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	if user.Nivel >= 12 {
		return nil, errors.New("você já alcançou o Nível 12 (Nível Máximo da Temporada)")
	}

	nextLevelNum := user.Nivel + 1

	if uc.levelRepo == nil {
		// Fallback compatibilidade legado
		activeCount, _ := uc.referralRepo.CountActiveByReferrerID(ctx, userID)
		if !user.CanLevelUp(activeCount) {
			return nil, domain.ErrLevelUpRequirementsNotMet
		}
		user.Nivel++
		user.ColetasCompleted = 0
		user.TratorRented = false
		_ = uc.userRepo.Update(ctx, user)
		return &LevelUpResponse{
			Message:  fmt.Sprintf("🎉 Parabéns! Você foi promovido para o Nível %d!", user.Nivel),
			NewLevel: user.Nivel,
			User:     user,
		}, nil
	}

	nextLevelConfig, err := uc.levelRepo.GetByLevel(ctx, nextLevelNum)
	if err != nil || nextLevelConfig == nil {
		return nil, fmt.Errorf("configuração para o nível %d não encontrada", nextLevelNum)
	}

	// 1. Validação Estrita das Missões Obrigatórias
	activeRefs := 0
	if uc.referralRepo != nil {
		activeRefs, _ = uc.referralRepo.CountActiveByReferrerID(ctx, userID)
	}
	if activeRefs < nextLevelConfig.RequiredReferrals {
		return nil, fmt.Errorf("requisito de convidados não atingido: necessário %d, você possui %d", nextLevelConfig.RequiredReferrals, activeRefs)
	}

	machines, _ := uc.machineRepo.GetByUserID(ctx, userID)
	if len(machines) < nextLevelConfig.RequiredMachines {
		return nil, fmt.Errorf("requisito de máquinas ativas não atingido: necessário %d, você possui %d", nextLevelConfig.RequiredMachines, len(machines))
	}

	if user.ColetasCompleted < nextLevelConfig.RequiredColetas {
		return nil, fmt.Errorf("requisito de coletas manuais não atingido: necessário %d, você possui %d", nextLevelConfig.RequiredColetas, user.ColetasCompleted)
	}

	if uc.rankingRepo != nil {
		userRank, _ := uc.rankingRepo.GetUserRank(ctx, userID)
		currentTonnage := 0.0
		if userRank != nil {
			currentTonnage = userRank.WeeklyTonnage
		}
		if currentTonnage < nextLevelConfig.RequiredTonnage {
			return nil, fmt.Errorf("requisito de tonelagem no ranking não atingido: necessário %.0ft, você possui %.0ft", nextLevelConfig.RequiredTonnage, currentTonnage)
		}
	}

	// 2. Aplicação da Promoção e Injeção dos Prêmios Exclusivos
	user.Nivel = nextLevelNum
	user.ColetasCompleted = 0 // Reinicia contador para a meta do próximo nível
	user.XP += 200

	// Prêmio: Vale Seguro (Nível 5)
	if nextLevelConfig.HasInsuranceVoucher {
		user.InsuranceVouchersCount++
	}

	// Prêmio: Giro Instantâneo da Roleta (Níveis 6 a 10)
	if nextLevelConfig.HasExtraRouletteSpin {
		user.AlreadySpunToday = false
	}

	// Prêmio: Expansão do Limite Máximo da Garagem (Níveis 11 e 12)
	if nextLevelConfig.MaxMachinesAllowed > user.MaxMachinesAllowed {
		user.MaxMachinesAllowed = nextLevelConfig.MaxMachinesAllowed
	}

	// Prêmio: Comissão de Afiliados Aumentada (6%, 7%, 8%, 9%, 10%)
	if nextLevelConfig.CommissionPercent > user.CustomReferralCommissionPercent {
		user.CustomReferralCommissionPercent = nextLevelConfig.CommissionPercent
	}

	// Prêmio Nível 12: Placa de Nível Máximo / Hall da Fama e início da contagem de atividade
	if user.Nivel == 12 {
		user.HasMaxLevelPlaque = true
		nowPromo := time.Now()
		user.Level12ReachedAt = &nowPromo
		if user.LastReferralCreatedAt == nil {
			user.LastReferralCreatedAt = &nowPromo
		}
		user.Level12BonusSuspended = false
	}

	txID, err := id.New("tx-lvl")
	if err != nil {
		return nil, err
	}

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	now := time.Now()
	tx := &domain.Transaction{
		ID:        txID,
		UserID:    userID,
		Tipo:      fmt.Sprintf("Promoção Nível %d (%s)", user.Nivel, nextLevelConfig.Title),
		Valor:     0.0,
		Status:    domain.TxStatusSucesso,
		CreatedAt: now,
		PaidAt:    &now,
	}
	_ = uc.txRepo.Create(ctx, tx)

	if uc.logRepo != nil {
		_ = uc.logRepo.LogSecurity(ctx, &userID, fmt.Sprintf("LEVEL_UP_TO_%d", user.Nivel), "", "", nextLevelConfig.SpecialRewardText)
	}

	return &LevelUpResponse{
		Message:  fmt.Sprintf("🎉 Parabéns! Você foi promovido para o Nível %d (%s)! Prêmio desbloqueado: %s", user.Nivel, nextLevelConfig.Title, nextLevelConfig.SpecialRewardText),
		NewLevel: user.Nivel,
		User:     user,
	}, nil
}

// RecoverLostDaysWithInsurance ativa o Vale Seguro recuperando até 2 dias de rendimento da frota ativa
func (uc *GarageUseCase) RecoverLostDaysWithInsurance(ctx context.Context, userID string) (*InsuranceRecoverResponse, error) {
	if financial, ok := uc.txRepo.(interface {
		RecoverInsuranceAtomic(context.Context, string) (*domain.User, float64, error)
	}); ok {
		user, amount, err := financial.RecoverInsuranceAtomic(ctx, userID)
		if err != nil {
			return nil, err
		}
		if uc.logRepo != nil {
			_ = uc.logRepo.LogUsage(ctx, "INSURANCE_RECOVER_2_DAYS", amount)
		}
		return &InsuranceRecoverResponse{Message: fmt.Sprintf("🛡️ Vale Seguro ativado! R$ %.2f creditados na sua conta referente a 2 dias de produção recuperados.", amount), RecoveredAmount: amount, NewBalance: user.Balance, RemainingVouchers: user.InsuranceVouchersCount}, nil
	}
	user, err := uc.userRepo.GetByIDForUpdate(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	if user.InsuranceVouchersCount <= 0 {
		return nil, errors.New("você não possui nenhum Vale Seguro disponível para resgate")
	}

	machines, err := uc.machineRepo.GetByUserID(ctx, userID)
	if err != nil || len(machines) == 0 {
		return nil, errors.New("você não possui máquinas ativas na garagem para calcular o rendimento do seguro")
	}

	// Calcula rendimento base de 2 dias de todas as máquinas ativas
	recoveredDailySum := 0.0
	for _, m := range machines {
		recoveredDailySum += m.YieldAmount
	}
	totalRecovered := math.Round(recoveredDailySum*2.0*100) / 100

	if totalRecovered <= 0 {
		return nil, errors.New("nenhum valor de rendimento elegível para recuperação")
	}

	txID, err := id.New("tx-ins")
	if err != nil {
		return nil, err
	}

	user.Balance = math.Round((user.Balance+totalRecovered)*100) / 100
	user.YieldBalanceAvailable = math.Round((user.YieldBalanceAvailable+totalRecovered)*100) / 100
	user.InsuranceVouchersCount--

	if err := uc.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	now := time.Now()
	tx := &domain.Transaction{
		ID:        txID,
		UserID:    userID,
		Tipo:      "Resgate Vale Seguro CAT (2 Dias)",
		Valor:     totalRecovered,
		Status:    domain.TxStatusSucesso,
		CreatedAt: now,
		PaidAt:    &now,
	}
	_ = uc.txRepo.Create(ctx, tx)

	if uc.logRepo != nil {
		_ = uc.logRepo.LogUsage(ctx, "INSURANCE_RECOVER_2_DAYS", totalRecovered)
	}

	return &InsuranceRecoverResponse{
		Message:           fmt.Sprintf("🛡️ Vale Seguro ativado! R$ %.2f creditados na sua conta referente a 2 dias de produção recuperados.", totalRecovered),
		RecoveredAmount:   totalRecovered,
		NewBalance:        user.Balance,
		RemainingVouchers: user.InsuranceVouchersCount,
	}, nil
}

// SimulateMachineState altera o estado temporal da garagem para testes em sandbox
func (uc *GarageUseCase) SimulateMachineState(ctx context.Context, userID, state string) error {
	machines, err := uc.machineRepo.GetByUserID(ctx, userID)
	if err != nil || len(machines) == 0 {
		return domain.ErrMachineNotFound
	}

	now := time.Now()
	for _, m := range machines {
		switch state {
		case "disponivel", "available":
			m.LastCollectedAt = now.Add(-24 * time.Hour)
			m.OverrideState = nil
			m.OverrideSecondsRemaining = nil
		case "expirado", "expired":
			m.LastCollectedAt = now.Add(-24*time.Hour - 15*time.Minute)
			m.OverrideState = nil
			m.OverrideSecondsRemaining = nil
		case "aguardando", "waiting":
			m.LastCollectedAt = now
			m.OverrideState = nil
			m.OverrideSecondsRemaining = nil
		default:
			m.OverrideState = nil
			m.OverrideSecondsRemaining = nil
		}
		_ = uc.machineRepo.Update(ctx, m)
	}

	return nil
}
