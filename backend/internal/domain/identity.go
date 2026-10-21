package domain

import (
	"context"
	"errors"
)

var ErrIdentityNotVerified = errors.New("identidade pendente de conferência")
var ErrIdentityAlreadySubmitted = errors.New("identidade já enviada para conferência")
var ErrIdentityAlreadyExists = errors.New("documento já cadastrado")
var ErrIdentityState = errors.New("estado de identidade inválido")

type IdentityPerson struct {
	CPF       string
	LegalName string
	Email     string
}

// IdentityRepository mantém dados pessoais fora do modelo público do usuário.
type IdentityRepository interface {
	CreateUserWithIdentity(context.Context, *User, IdentityPerson) error
	Submit(context.Context, string, IdentityPerson) error
	Status(context.Context, string) (string, error)
	VerifiedPerson(context.Context, string) (IdentityPerson, error)
	Decide(context.Context, string, string, string, string) error
	Pending(context.Context) ([]string, error)
	Inspect(context.Context, string, string) (IdentityPerson, error)
}
