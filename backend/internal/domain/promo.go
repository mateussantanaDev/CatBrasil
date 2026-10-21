package domain

import (
	"context"
	"time"
)

// UserPromo representa uma oferta promocional customizada para um usuário ou rede de afiliados
type UserPromo struct {
	ID                        string    `json:"id"`
	Title                     string    `json:"title"`
	TargetType                string    `json:"targetType"` // "user", "network", "all"
	TargetID                  string    `json:"targetId"`
	MachineID                 string    `json:"machineId"`
	CustomPrice               float64   `json:"customPrice"`
	CustomReturnPercentPerDay float64   `json:"customReturnPercentPerDay"`
	CustomMaxPurchases        int       `json:"customMaxPurchases"`
	Active                    bool      `json:"active"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// PromoRepository define o contrato de persistência para Ofertas Promocionais Customizadas
type PromoRepository interface {
	GetAll(ctx context.Context) ([]*UserPromo, error)
	GetByTarget(ctx context.Context, targetType, targetID string) ([]*UserPromo, error)
	GetByID(ctx context.Context, id string) (*UserPromo, error)
	Create(ctx context.Context, promo *UserPromo) error
	Delete(ctx context.Context, id string) error
}
