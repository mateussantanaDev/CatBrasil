package domain

import (
	"context"
	"time"
)

type MachineState string

const (
	MachineStateAguardando MachineState = "aguardando"
	MachineStateDisponivel MachineState = "disponivel"
	MachineStateExpirado   MachineState = "expirado"
)

// Machine representa um contrato ativo de máquina de mineração na garagem do operador
type Machine struct {
	ID                       string       `json:"id"`
	UserID                   string       `json:"userId"`
	Name                     string       `json:"name"`
	Cost                     float64      `json:"cost"`
	YieldAmount              float64      `json:"yieldAmount"`
	LastCollectedAt          time.Time    `json:"lastCollectedAt"`
	State                    MachineState `json:"state"`
	OverrideState            *string      `json:"overrideState,omitempty"`
	OverrideSecondsRemaining *int64       `json:"overrideSecondsRemaining,omitempty"`
	SecondsRemaining         int64        `json:"secondsRemaining"`
	CreatedAt                time.Time    `json:"createdAt"`
	UpdatedAt                time.Time    `json:"updatedAt"`
}

// CalculateState recalcula o estado dinâmico da máquina com base no ciclo de 24h e tolerância de coleta (ex: 10 min)
func (m *Machine) CalculateState(now time.Time, graceMinutes int) (MachineState, int64) {
	if m.OverrideState != nil && *m.OverrideState != "" {
		secs := int64(0)
		if m.OverrideSecondsRemaining != nil {
			secs = *m.OverrideSecondsRemaining
		}
		return MachineState(*m.OverrideState), secs
	}

	cycleDuration := 24 * time.Hour
	graceDuration := time.Duration(graceMinutes) * time.Minute
	nextRelease := m.LastCollectedAt.Add(cycleDuration)
	limitTime := nextRelease.Add(graceDuration)

	if now.Before(nextRelease) {
		secs := int64(nextRelease.Sub(now).Seconds())
		if secs < 0 {
			secs = 0
		}
		return MachineStateAguardando, secs
	}

	if (now.Equal(nextRelease) || now.After(nextRelease)) && (now.Before(limitTime) || now.Equal(limitTime)) {
		secs := int64(limitTime.Sub(now).Seconds())
		if secs < 0 {
			secs = 0
		}
		return MachineStateDisponivel, secs
	}

	return MachineStateExpirado, 0
}

// ResetCycle reinicia o ciclo de contagem da máquina definindo o timestamp da última coleta
func (m *Machine) ResetCycle(now time.Time) {
	m.LastCollectedAt = now
	m.OverrideState = nil
	m.OverrideSecondsRemaining = nil
}

// FleetCatalogItem representa um modelo de máquina no catálogo oficial de frota da plataforma
type FleetCatalogItem struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Category            string    `json:"category"`
	Price               float64   `json:"price"`
	DailyYield          float64   `json:"dailyYield"`
	ReturnPercentPerDay float64   `json:"returnPercentPerDay"`
	DurationDays        int       `json:"durationDays"`
	TotalContractReturn float64   `json:"totalContractReturn"`
	StockQuantity       int       `json:"stockQuantity"`
	MaxPurchasesPerUser int       `json:"maxPurchasesPerUser"`
	ReinvestOnly        bool      `json:"reinvestOnly"`
	ImageURL            string    `json:"imageUrl"`
	Active              bool      `json:"active"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// MachineRepository define o contrato da camada de persistência para Máquinas da Garagem
type MachineRepository interface {
	GetByID(ctx context.Context, id string) (*Machine, error)
	GetByIDForUpdate(ctx context.Context, id string) (*Machine, error)
	GetByUserID(ctx context.Context, userID string) ([]*Machine, error)
	Create(ctx context.Context, machine *Machine) error
	Update(ctx context.Context, machine *Machine) error
	Delete(ctx context.Context, id string) error
}

// CatalogRepository define o contrato da camada de persistência para o Catálogo Oficial de Frota
type CatalogRepository interface {
	GetAll(ctx context.Context) ([]*FleetCatalogItem, error)
	GetActive(ctx context.Context) ([]*FleetCatalogItem, error)
	GetByID(ctx context.Context, id string) (*FleetCatalogItem, error)
	Upsert(ctx context.Context, item *FleetCatalogItem) error
	SaveCatalog(ctx context.Context, items []*FleetCatalogItem) error
	Delete(ctx context.Context, id string) error
}
