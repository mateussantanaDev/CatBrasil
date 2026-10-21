package domain

import (
	"context"
	"time"
)

// SMSVerification representa um código de autenticação/cadastro temporário enviado por SMS
type SMSVerification struct {
	ID        int64     `json:"id"`
	Phone     string    `json:"phone"`
	Code      string    `json:"-"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"createdAt"`
}

// SMSRepository define o contrato para verificação e persistência de códigos SMS
type SMSRepository interface {
	CreateCode(ctx context.Context, phone, code string, expiresAt time.Time) error
	InvalidateCode(ctx context.Context, phone, code string) error
	VerifyCode(ctx context.Context, phone, code string) (bool, error)
}
