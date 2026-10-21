package jwt

import (
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("token JWT inválido ou corrompido")
	ErrExpiredToken = errors.New("sessão expirada, realize login novamente")
)

// CustomClaims encapsula as informações do operador/gestor assinadas no token
type CustomClaims struct {
	UserID   string          `json:"userId"`
	Username string          `json:"username"`
	Role     domain.UserRole `json:"role"`
	IsAdmin  bool            `json:"isAdmin"`
	jwt.RegisteredClaims
}

// TokenManager gerencia geração e validação de tokens JWT
type TokenManager struct {
	secretKey     []byte
	durationHours time.Duration
}

// NewTokenManager cria uma nova instância com a chave secreta e duração configuradas
func NewTokenManager(secretKey string, durationHours int) *TokenManager {
	if secretKey == "" {
		panic("JWT_SECRET é obrigatório")
	}
	if durationHours <= 0 {
		durationHours = 168 // 7 dias
	}
	return &TokenManager{
		secretKey:     []byte(secretKey),
		durationHours: time.Duration(durationHours) * time.Hour,
	}
}

// GenerateToken cria e assina um novo token JWT
func (m *TokenManager) GenerateToken(user *domain.User) (string, error) {
	return m.GenerateTokenWithTTL(user, m.durationHours)
}

func (m *TokenManager) GenerateTokenWithTTL(user *domain.User, ttl time.Duration) (string, error) {
	if ttl <= 0 || ttl > m.durationHours {
		return "", fmt.Errorf("duração de token inválida")
	}
	now := time.Now()
	claims := CustomClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		IsAdmin:  user.IsAdmin(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "catbrasil-backend",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", fmt.Errorf("erro ao assinar token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken decodifica e valida a assinatura criptográfica do token
func (m *TokenManager) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
