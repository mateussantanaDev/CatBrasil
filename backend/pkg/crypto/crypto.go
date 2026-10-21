package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const BcryptCost = 12

// HashPassword gera o hash bcrypt seguro da senha
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("falha ao criptografar senha: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword compara a senha em texto puro com o hash bcrypt (timing-safe)
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRandomHex gera uma string hexadecimal criptograficamente segura de N bytes
func GenerateRandomHex(nBytes int) (string, error) {
	bytes := make([]byte, nBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRandomNumericCode gera um código numérico de N dígitos (ex: 6 dígitos para SMS/PIN)
func GenerateRandomNumericCode(digits int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(fmt.Errorf("falha ao gerar código aleatório: %w", err))
	}
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, n.Int64())
}

// GenerateHMACSHA256 gera a assinatura HMAC-SHA256 de um payload com uma chave secreta
func GenerateHMACSHA256(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMACSHA256 valida a assinatura HMAC em tempo constante para prevenir timing attacks
func VerifyHMACSHA256(payload, signature, secret string) bool {
	expected := GenerateHMACSHA256(payload, secret)
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1 ||
		subtle.ConstantTimeCompare([]byte(signature), []byte("sha256="+expected)) == 1
}
