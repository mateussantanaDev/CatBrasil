package domain

import (
	"context"
	"time"
)

// GatewayConfig representa as taxas e limites do Gateway de Pagamento PIX e credenciais SuitPay / Paradise
type GatewayConfig struct {
	Name                 string  `json:"name"`
	DepositFeePercent    float64 `json:"depositFeePercent"`
	WithdrawFeeFixed     float64 `json:"withdrawFeeFixed"`
	PixExpirationMinutes int     `json:"pixExpirationMinutes"`
	ActiveProvider       string  `json:"activeProvider"` // "simulated", "suitpay", "paradise"
	SuitPayClientID      string  `json:"-"`
	SuitPayClientSecret  string  `json:"-"`
	SuitPaySandbox       bool    `json:"suitpaySandbox"`
	SuitPayCallbackURL   string  `json:"-"`
	SuitPayEnabled       bool    `json:"suitpayEnabled"`
	ParadiseSecretKey    string  `json:"-"`
	ParadiseProductHash  string  `json:"-"`
	ParadisePostbackURL  string  `json:"-"`
	ParadiseEnabled      bool    `json:"paradiseEnabled"`
}

// SystemConfig representa a parametrização global do ERP CatBrasil
type SystemConfig struct {
	ID                            int           `json:"id"`
	GlobalWithdrawalsLocked       bool          `json:"globalWithdrawalsLocked"`
	AutoApproveEnabled            bool          `json:"autoApproveEnabled"`
	AutoApproveWithdrawalsUnder   float64       `json:"autoApproveWithdrawalsUnder"`
	ReferralCommissionPercent     float64       `json:"referralCommissionPercent"`
	RequiredReferralsCount        int           `json:"requiredReferralsCount"`
	RequireReferralsForWithdrawal bool          `json:"requireReferralsForWithdrawal"`
	AntiSelfReferralLock          bool          `json:"antiSelfReferralLock"`
	BonusPerMutiraoAmount         float64       `json:"bonusPerMutiraoAmount"`
	CollectionGraceMinutes        int           `json:"collectionGraceMinutes"`
	Gateway                       GatewayConfig `json:"gateway"`
	StrictModalMode               bool          `json:"strictModalMode"`
	RequireDepositToSpin          bool          `json:"requireDepositToSpin"`
	SeasonDurationDays            int           `json:"seasonDurationDays"`
	CurrentSeasonNumber           int           `json:"currentSeasonNumber"`
	SeasonStartedAt               time.Time     `json:"seasonStartedAt"`
	Level12MaintenanceDays        int           `json:"level12MaintenanceDays"`
	Level12RequiredReferrals      int           `json:"level12RequiredReferrals"`
	CreatedAt                     time.Time     `json:"createdAt"`
	UpdatedAt                     time.Time     `json:"updatedAt"`
}

// ConfigRepository define o contrato da camada de persistência para as Configurações Globais
type ConfigRepository interface {
	GetConfig(ctx context.Context) (*SystemConfig, error)
	UpdateConfig(ctx context.Context, config *SystemConfig) error
	ToggleGlobalWithdrawLock(ctx context.Context, locked bool) error
	UpdateAutoApprove(ctx context.Context, enabled bool, under float64) error
	UpdateGatewayConfig(ctx context.Context, gw GatewayConfig, graceMinutes int) error
	UpdateReferralConfig(ctx context.Context, commPercent float64, reqCount int, bonusMutirao float64, reqForWith, antiSelf bool) error
	ResetSeason(ctx context.Context, durationDays int) error
	UpdateLevel12Config(ctx context.Context, maintenanceDays, requiredReferrals int) error
}
