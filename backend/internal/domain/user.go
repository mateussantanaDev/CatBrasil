package domain

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

type UserRole string

const (
	RoleUser      UserRole = "user"
	RoleAdmin     UserRole = "admin"
	RoleGestor    UserRole = "gestor"
	RoleDiretoria UserRole = "diretoria"
)

// PartsInventory representa as peças para montagem da Escavadeira CAT 320 na Oficina
type PartsInventory struct {
	MotorC15         int `json:"motorC15"`
	EsteiraReforcada int `json:"esteiraReforcada"`
	FiltroOleo       int `json:"filtroOleo"`
	TurboGarrett     int `json:"turboGarrett"`
}

// FinancialStats representa as estatísticas financeiras consolidadas do operador
type FinancialStats struct {
	TotalDeposited float64 `json:"totalDeposited"`
	TotalWithdrawn float64 `json:"totalWithdrawn"`
	NetLtvProfit   float64 `json:"netLtvProfit"`
}

// User é a entidade pura de domínio do Operador / Gestor
type User struct {
	ID                     string   `json:"id"`
	Username               string   `json:"username"`
	Phone                  string   `json:"phone"`
	PasswordHash           string   `json:"-"`
	SecurityPassword       string   `json:"-"`
	Role                   UserRole `json:"role"`
	Balance                float64  `json:"balance"`
	YieldBalanceAvailable  float64  `json:"yieldBalanceAvailable"`
	XP                     int      `json:"xp"`
	Nivel                  int      `json:"nivel"`
	ColetasCompleted       int      `json:"coletasCompleted"`
	TratorRented           bool     `json:"tratorRented"`
	AlreadySpunToday       bool     `json:"alreadySpunToday"`
	PromoUnlocked          bool     `json:"promoUnlocked"`
	FreeWithdrawActive     bool     `json:"freeWithdrawActive"`
	WithdrawLocked         bool     `json:"withdrawLocked"`
	ReferralBonusSuspended bool     `json:"referralBonusSuspended"`
	MultiAccountFlag       bool     `json:"multiAccountFlag"`
	RequiredRollover       float64  `json:"requiredRollover"`
	CurrentRollover        float64  `json:"currentRollover"`
	IPAddress              string   `json:"ipAddress"`
	DeviceFingerprint      string   `json:"deviceFingerprint"`
	TwoFactorSecret        string   `json:"-"`
	TwoFactorEnabled       bool     `json:"twoFactorEnabled"`

	// Inventário de Peças Caterpillar
	MotorC15Count         int `json:"motorC15Count"`
	EsteiraReforcadaCount int `json:"esteiraReforcadaCount"`
	FiltroOleoCount       int `json:"filtroOleoCount"`
	TurboGarrettCount     int `json:"turboGarrettCount"`

	// Passe de Batalha, Vouchers & Temporada
	InsuranceVouchersCount          int        `json:"insuranceVouchersCount"`
	MaxMachinesAllowed              int        `json:"maxMachinesAllowed"`
	HasMaxLevelPlaque               bool       `json:"hasMaxLevelPlaque"`
	SeasonNumber                    int        `json:"seasonNumber"`
	CustomReferralCommissionPercent float64    `json:"customReferralCommissionPercent"`
	LastReferralCreatedAt           *time.Time `json:"lastReferralCreatedAt,omitempty"`
	Level12ReachedAt                *time.Time `json:"level12ReachedAt,omitempty"`
	Level12BonusSuspended           bool       `json:"level12BonusSuspended"`
	LastSpinDate                    string     `json:"lastSpinDate,omitempty"`
	LastSpinAt                      *time.Time `json:"lastSpinAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Version   int64     `json:"-"`
}

// HasSpunToday deriva a elegibilidade do giro prioritariamente por last_spin_date
func (u *User) HasSpunToday(todayDate string) bool {
	if u.LastSpinDate != "" {
		return u.LastSpinDate == todayDate
	}
	return u.AlreadySpunToday
}

// MarshalJSON exposes only PIN enrollment status, never its hash.
func (u User) MarshalJSON() ([]byte, error) {
	type publicUser User
	return json.Marshal(struct {
		HasSecurityPassword bool `json:"hasSecurityPassword"`
		publicUser
	}{HasSecurityPassword: u.SecurityPassword != "", publicUser: publicUser(u)})
}

// IsAdmin verifica se o usuário possui cargo administrativo
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleGestor || u.Role == RoleDiretoria
}

// CanLevelUp valida se o operador cumpre todos os requisitos obrigatórios para promoção de nível:
// 1. 5 ou mais coletas concluídas
// 2. Possuir pelo menos 1 contrato do Trator D8T ativo
// 3. Possuir no mínimo 3 indicados diretos no status 'ativo'
func (u *User) CanLevelUp(activeReferralsCount int) bool {
	return u.ColetasCompleted >= 5 && u.TratorRented && activeReferralsCount >= 3
}

// CalculateYieldWithLevelBonus calcula o rendimento diário aplicando o bônus de nível de carreira:
// Rendimento Total = yieldAmount * (1 + (nivel - 1) * 0.05)
func (u *User) CalculateYieldWithLevelBonus(baseYield float64) float64 {
	multiplier := 1.0 + float64(u.Nivel-1)*0.05
	return math.Round(baseYield*multiplier*100) / 100
}

// HasRequiredRollover verifica se o usuário já completou o giro mínimo exigido de contratação de frotas
func (u *User) HasRequiredRollover() bool {
	if u.RequiredRollover <= 0 {
		return true
	}
	return u.CurrentRollover >= u.RequiredRollover
}

// RolloverProgressPercent calcula a porcentagem (0 a 100%) de conclusão do rollover
func (u *User) RolloverProgressPercent() float64 {
	if u.RequiredRollover <= 0 {
		return 100.0
	}
	pct := (u.CurrentRollover / u.RequiredRollover) * 100.0
	if pct > 100.0 {
		return 100.0
	}
	return math.Round(pct*10) / 10
}

// HasCompleteParts verifica se o usuário possui todas as 4 peças necessárias para montagem
func (u *User) HasCompleteParts() bool {
	return u.MotorC15Count >= 1 && u.EsteiraReforcadaCount >= 1 && u.FiltroOleoCount >= 1 && u.TurboGarrettCount >= 1
}

// ConsumePartsForAssembly debita 1 unidade de cada peça para montagem na oficina
func (u *User) ConsumePartsForAssembly() bool {
	if !u.HasCompleteParts() {
		return false
	}
	u.MotorC15Count--
	u.EsteiraReforcadaCount--
	u.FiltroOleoCount--
	u.TurboGarrettCount--
	return true
}

// GetPartsInventory retorna o DTO formatado do inventário
func (u *User) GetPartsInventory() PartsInventory {
	return PartsInventory{
		MotorC15:         u.MotorC15Count,
		EsteiraReforcada: u.EsteiraReforcadaCount,
		FiltroOleo:       u.FiltroOleoCount,
		TurboGarrett:     u.TurboGarrettCount,
	}
}

// UserRepository define o contrato da camada de persistência para Usuários
type UserRepository interface {
	GetByID(ctx context.Context, id string) (*User, error)
	GetByIDForUpdate(ctx context.Context, id string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetByFingerprintOrIP(ctx context.Context, fingerprint, ip string) ([]*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	AdjustBalance(ctx context.Context, id string, deltaBalance, deltaYield float64) (*User, error)
	List(ctx context.Context, query, status string, limit, offset int) ([]*User, int, error)
	Count(ctx context.Context) (int64, error)
	ResetDailySpins(ctx context.Context) error
}
