package domain

import (
	"context"
	"time"
)

type TxStatus string

const (
	TxStatusPendente    TxStatus = "pendente"
	TxStatusSucesso     TxStatus = "sucesso"
	TxStatusRejeitado   TxStatus = "rejeitado"
	TxStatusCancelado   TxStatus = "cancelado"
	TxStatusProcessando TxStatus = "processando"
)

// Transaction representa uma movimentação financeira na plataforma
type Transaction struct {
	ID                string     `json:"id"`
	UserID            string     `json:"userId"`
	Tipo              string     `json:"tipo"`
	Valor             float64    `json:"valor"`
	Status            TxStatus   `json:"status"`
	ExternalID        *string    `json:"externalId,omitempty"`
	PixKey            *string    `json:"pixKey,omitempty"`
	PixCode           *string    `json:"pixCode,omitempty"`
	QRCodeURL         *string    `json:"qrCodeUrl,omitempty"`
	NetAmount         *float64   `json:"netAmount,omitempty"`
	FeeAmount         *float64   `json:"feeAmount,omitempty"`
	ValorEfetivoPago  *float64   `json:"valorEfetivoPago,omitempty"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	RejectionReason   *string    `json:"rejectionReason,omitempty"`
	GatewayProvider   string     `json:"gatewayProvider,omitempty"`
	IdempotencyKey    *string    `json:"-"`
	ReferenceID       *string    `json:"referenceId,omitempty"`
	PayoutAttemptedAt *time.Time `json:"payoutAttemptedAt,omitempty"`
	ReconciledAt      *time.Time `json:"reconciledAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ReconciliationIssue struct {
	TransactionID string    `json:"transactionId,omitempty"`
	UserID        string    `json:"userId,omitempty"`
	ExternalID    string    `json:"externalId,omitempty"`
	Kind          string    `json:"kind"`
	Amount        float64   `json:"amount"`
	Since         time.Time `json:"since"`
	QueueID       int64     `json:"queueId,omitempty"`
	Reason        string    `json:"reason,omitempty"`
}

// TransactionRepository define o contrato da camada de persistência para Transações Financeiras
type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByID(ctx context.Context, id string) (*Transaction, error)
	GetByIDForUpdate(ctx context.Context, id string) (*Transaction, error)
	GetByUserID(ctx context.Context, userID string, filter string) ([]*Transaction, error)
	GetAll(ctx context.Context, status string, limit, offset int) ([]*Transaction, int, error)
	Update(ctx context.Context, tx *Transaction) error
	ApproveWithdrawal(ctx context.Context, txID string) (*Transaction, error)
	RejectWithdrawal(ctx context.Context, txID, reason string, lockAccount bool) (*Transaction, error)
}
