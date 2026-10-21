package domain

import (
	"context"
	"time"
)

// LevelConfig representa as regras e premiações editáveis de um nível (1 a 12)
type LevelConfig struct {
	Level                  int       `json:"level"`
	Title                  string    `json:"title"`
	TierName               string    `json:"tierName"`
	RequiredReferrals      int       `json:"requiredReferrals"`
	RequiredMachines       int       `json:"requiredMachines"`
	RequiredColetas        int       `json:"requiredColetas"`
	RequiredTonnage        float64   `json:"requiredTonnage"`
	UnlockedMachineID      *string   `json:"unlockedMachineId,omitempty"`
	UnlockedMachineName    string    `json:"unlockedMachineName,omitempty"`
	CommissionPercent      float64   `json:"commissionPercent"`
	BonusYieldPercent      float64   `json:"bonusYieldPercent"`
	HasInsuranceVoucher    bool      `json:"hasInsuranceVoucher"`
	InsuranceDaysRecovered int       `json:"insuranceDaysRecovered"`
	HasExtraRouletteSpin   bool      `json:"hasExtraRouletteSpin"`
	HasVIPRoulette         bool      `json:"hasVipRoulette"`
	MaxMachinesAllowed     int       `json:"maxMachinesAllowed"`
	SpecialRewardText      string    `json:"specialRewardText"`
	BadgeName              string    `json:"badgeName"`
	BadgeIcon              string    `json:"badgeIcon"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// BattlePassStatus consolida o status do passe de batalha para o operador
type BattlePassStatus struct {
	CurrentLevel          int            `json:"currentLevel"`
	CurrentSeasonNumber   int            `json:"currentSeasonNumber"`
	SeasonDurationDays    int            `json:"seasonDurationDays"`
	SeasonDaysRemaining   int            `json:"seasonDaysRemaining"`
	SeasonStartedAt       time.Time      `json:"seasonStartedAt"`
	SeasonEndsAt          time.Time      `json:"seasonEndsAt"`
	CurrentReferrals      int            `json:"currentReferrals"`
	CurrentActiveMachines int            `json:"currentActiveMachines"`
	CurrentColetas        int            `json:"currentColetas"`
	CurrentTonnage        float64        `json:"currentTonnage"`
	InsuranceVouchers     int            `json:"insuranceVouchers"`
	MaxMachinesAllowed    int            `json:"maxMachinesAllowed"`
	CommissionPercent     float64        `json:"commissionPercent"`
	BonusYieldPercent     float64        `json:"bonusYieldPercent"`
	HasMaxLevelPlaque     bool           `json:"hasMaxLevelPlaque"`
	CanLevelUp                      bool           `json:"canLevelUp"`
	MissingRequirements             []string       `json:"missingRequirements"`
	NextLevelConfig                 *LevelConfig   `json:"nextLevelConfig,omitempty"`
	AllLevels                       []*LevelConfig `json:"allLevels"`
	IsLevel12                       bool           `json:"isLevel12"`
	Level12MaintenanceDaysRemaining int            `json:"level12MaintenanceDaysRemaining"`
	Level12BonusSuspended           bool           `json:"level12BonusSuspended"`
	Level12WarningActive            bool           `json:"level12WarningActive"`
	Level12WarningMessage           string         `json:"level12WarningMessage"`
}

// LevelRepository define o contrato para persistência e consulta dos níveis
type LevelRepository interface {
	GetAll(ctx context.Context) ([]*LevelConfig, error)
	GetByLevel(ctx context.Context, level int) (*LevelConfig, error)
	Update(ctx context.Context, config *LevelConfig) error
}
