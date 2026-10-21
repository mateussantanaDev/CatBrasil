package domain

import (
	"context"
	"time"
)

type ReferralStatus string

const (
	ReferralStatusAtivo   ReferralStatus = "ativo"
	ReferralStatusInativo ReferralStatus = "inativo"
)

// Referral representa um operador indicado na rede de afiliados
type Referral struct {
	ID             string         `json:"id"`
	ReferrerID     string         `json:"referrerId"`
	ReferredUserID *string        `json:"referredUserId,omitempty"`
	Nome           string         `json:"nome"`
	Status         ReferralStatus `json:"status"`
	Deposito       float64        `json:"deposito"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// ReferralRepository define o contrato da camada de persistência para a Rede de Afiliados
type ReferralRepository interface {
	GetByReferrerID(ctx context.Context, referrerID string) ([]*Referral, error)
	GetAll(ctx context.Context) ([]*Referral, error)
	GetByID(ctx context.Context, id string) (*Referral, error)
	GetByReferredName(ctx context.Context, name string) (*Referral, error)
	Create(ctx context.Context, ref *Referral) error
	Update(ctx context.Context, ref *Referral) error
	CountActiveByReferrerID(ctx context.Context, referrerID string) (int, error)
}
