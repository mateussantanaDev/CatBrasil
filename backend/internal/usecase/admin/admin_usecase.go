package admin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/usecase/payment"
	"catbrasil/backend/pkg/id"
)

type AdminUseCase struct {
	userRepo     domain.UserRepository
	machineRepo  domain.MachineRepository
	txRepo       domain.TransactionRepository
	dreRepo      domain.DRERepository
	catalogRepo  domain.CatalogRepository
	rouletteRepo domain.RouletteRepository
	announceRepo domain.AnnouncementRepository
	configRepo   domain.ConfigRepository
	logRepo      domain.LogRepository
	paymentUC    *payment.PaymentUseCase
}

func (uc *AdminUseCase) SetPaymentUseCase(paymentUC *payment.PaymentUseCase) {
	uc.paymentUC = paymentUC
}

func NewAdminUseCase(
	userRepo domain.UserRepository,
	machineRepo domain.MachineRepository,
	txRepo domain.TransactionRepository,
	dreRepo domain.DRERepository,
	catalogRepo domain.CatalogRepository,
	rouletteRepo domain.RouletteRepository,
	announceRepo domain.AnnouncementRepository,
	configRepo domain.ConfigRepository,
	logRepo domain.LogRepository,
) *AdminUseCase {
	return &AdminUseCase{
		userRepo:     userRepo,
		machineRepo:  machineRepo,
		txRepo:       txRepo,
		dreRepo:      dreRepo,
		catalogRepo:  catalogRepo,
		rouletteRepo: rouletteRepo,
		announceRepo: announceRepo,
		configRepo:   configRepo,
		logRepo:      logRepo,
	}
}

// GetDRE calcula o demonstrativo consolidado de resultados da casa em tempo real
func (uc *AdminUseCase) GetDRE(ctx context.Context) (*domain.DREMetrics, error) {
	return uc.dreRepo.GetMetrics(ctx)
}

// ListUsers lista os operadores com filtros e paginação
func (uc *AdminUseCase) ListUsers(ctx context.Context, query, status string, limit, offset int) ([]*domain.User, int, error) {
	return uc.userRepo.List(ctx, query, status, limit, offset)
}

// UpdateUser realiza alterações administrativas no cadastro do operador
func (uc *AdminUseCase) UpdateUser(ctx context.Context, user *domain.User) error {
	return uc.userRepo.Update(ctx, user)
}

// AdjustUserBalance altera o saldo de um operador
func (uc *AdminUseCase) AdjustUserBalance(ctx context.Context, userID string, deltaBalance float64, audit ...string) (*domain.User, error) {
	if financial, ok := uc.txRepo.(interface {
		AdjustAdminBalanceAtomic(context.Context, string, string, string, string, float64) (*domain.User, error)
	}); ok {
		if len(audit) < 3 {
			return nil, fmt.Errorf("auditoria de ajuste financeiro obrigatória")
		}
		return financial.AdjustAdminBalanceAtomic(ctx, userID, audit[0], audit[1], audit[2], deltaBalance)
	}
	return uc.userRepo.AdjustBalance(ctx, userID, deltaBalance, 0)
}

// ListTransactions lista o histórico consolidado de transações do sistema
func (uc *AdminUseCase) ListTransactions(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	return uc.txRepo.GetAll(ctx, status, limit, offset)
}

// ApproveWithdrawal aprova uma solicitação de saque pendente no banco
func (uc *AdminUseCase) ApproveWithdrawal(ctx context.Context, txID string) (*domain.Transaction, error) {
	if uc.paymentUC != nil {
		return uc.paymentUC.ApprovePayout(ctx, txID)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		return nil, fmt.Errorf("serviço financeiro indisponível")
	}
	return uc.txRepo.ApproveWithdrawal(ctx, txID)
}

// RejectWithdrawal rejeita um saque, estorna o saldo ao operador e opcionalmente bloqueia saques futuros
func (uc *AdminUseCase) RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*domain.Transaction, error) {
	return uc.txRepo.RejectWithdrawal(ctx, txID, reason, lockAccount)
}

// ListCatalog lista todos os modelos de frota cadastrados
func (uc *AdminUseCase) ListCatalog(ctx context.Context) ([]*domain.FleetCatalogItem, error) {
	return uc.catalogRepo.GetAll(ctx)
}

// SaveCatalogItem salva ou atualiza um modelo de máquina na frota oficial
func (uc *AdminUseCase) SaveCatalogItem(ctx context.Context, item *domain.FleetCatalogItem) error {
	return uc.catalogRepo.Upsert(ctx, item)
}

// DeleteCatalogItem remove um modelo de frota
func (uc *AdminUseCase) DeleteCatalogItem(ctx context.Context, id string) error {
	return uc.catalogRepo.Delete(ctx, id)
}

// ListWithdrawals lista a fila de saques por status
func (uc *AdminUseCase) ListWithdrawals(ctx context.Context, status string, limit, offset int) ([]*domain.Transaction, int, error) {
	if repo, ok := uc.txRepo.(interface {
		ListWithdrawals(context.Context, string, int, int) ([]*domain.Transaction, int, error)
	}); ok {
		return repo.ListWithdrawals(ctx, status, limit, offset)
	}
	return uc.txRepo.GetAll(ctx, status, limit, offset)
}

func (uc *AdminUseCase) ListFinancialReconciliation(ctx context.Context) ([]domain.ReconciliationIssue, error) {
	if repo, ok := uc.txRepo.(interface {
		ListFinancialReconciliation(context.Context) ([]domain.ReconciliationIssue, error)
	}); ok {
		return repo.ListFinancialReconciliation(ctx)
	}
	return nil, fmt.Errorf("conciliação indisponível")
}

// GetRouletteMatrix calcula a matriz de fatias, regras e valor esperado EV
func (uc *AdminUseCase) GetRouletteMatrix(ctx context.Context) (*domain.RouletteMatrix, error) {
	slices, err := uc.rouletteRepo.GetSlices(ctx)
	if err != nil {
		return nil, err
	}
	ev := domain.CalculateExpectedValue(slices)
	totalProb, isValid := domain.ValidateProbabilities(slices)

	rules := domain.RouletteRules{
		StrictModalMode:      true,
		RequireDepositToSpin: false,
	}
	cfg, err := uc.configRepo.GetConfig(ctx)
	if err == nil && cfg != nil {
		rules.StrictModalMode = cfg.StrictModalMode
		rules.RequireDepositToSpin = cfg.RequireDepositToSpin
	}

	return &domain.RouletteMatrix{
		Slices: slices,
		Metrics: domain.RouletteMetrics{
			TotalProb:       totalProb,
			ExpectedValueEV: ev,
			IsProbValid:     isValid,
		},
		Rules: rules,
	}, nil
}

// InjectBonusMachine insere diretamente um modelo de frota na garagem do operador
func (uc *AdminUseCase) InjectBonusMachine(ctx context.Context, userID, machineModel string) (*domain.FleetCatalogItem, error) {
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	item, err := uc.catalogRepo.GetByID(ctx, machineModel)
	if err != nil || item == nil {
		return nil, domain.ErrMachineCatalogNotFound
	}

	machID, err := id.New("mach-bonus")
	if err != nil {
		return nil, err
	}
	newMachine := &domain.Machine{
		ID:              machID,
		UserID:          userID,
		Name:            item.Name + " (Bônus Gestão) 🎁",
		Cost:            0.0,
		YieldAmount:     item.DailyYield,
		LastCollectedAt: time.Now(),
		State:           domain.MachineStateAguardando,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := uc.machineRepo.Create(ctx, newMachine); err != nil {
		return nil, err
	}

	return item, nil
}

// UpdateRouletteConfig atualiza a matriz de probabilidades da roleta com validação de 100%
func (uc *AdminUseCase) UpdateRouletteConfig(ctx context.Context, slices []*domain.RouletteSlice, rules ...*domain.RouletteRules) error {
	_, isValid := domain.ValidateProbabilities(slices)
	if !isValid {
		return domain.ErrProbabilitiesMustEqual100
	}
	if len(rules) > 0 && rules[0] != nil {
		cfg, err := uc.configRepo.GetConfig(ctx)
		if err == nil && cfg != nil {
			cfg.StrictModalMode = rules[0].StrictModalMode
			cfg.RequireDepositToSpin = rules[0].RequireDepositToSpin
			_ = uc.configRepo.UpdateConfig(ctx, cfg)
		}
	}
	return uc.rouletteRepo.SaveSlices(ctx, slices)
}

// ListAnnouncements lista todos os comunicados do mural
func (uc *AdminUseCase) ListAnnouncements(ctx context.Context) ([]*domain.Announcement, error) {
	return uc.announceRepo.GetAll(ctx)
}

// CreateAnnouncement cria um novo comunicado no mural
func (uc *AdminUseCase) CreateAnnouncement(ctx context.Context, ann *domain.Announcement) error {
	if ann.ID == "" {
		evtID, err := id.New("evt")
		if err != nil {
			return err
		}
		ann.ID = evtID
	}
	ann.CreatedAt = time.Now()
	ann.Active = true
	return uc.announceRepo.Create(ctx, ann)
}

// DeleteAnnouncement remove um comunicado do mural
func (uc *AdminUseCase) DeleteAnnouncement(ctx context.Context, id string) error {
	return uc.announceRepo.Delete(ctx, id)
}

// GetSystemConfig obtém a configuração global do sistema
func (uc *AdminUseCase) GetSystemConfig(ctx context.Context) (*domain.SystemConfig, error) {
	return uc.configRepo.GetConfig(ctx)
}

// UpdateSystemConfig atualiza as configurações globais de saques e gateway
func (uc *AdminUseCase) UpdateSystemConfig(ctx context.Context, cfg *domain.SystemConfig) error {
	return uc.configRepo.UpdateConfig(ctx, cfg)
}
