package id

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidPrefix = errors.New("prefixo de identificador inválido")
	ErrEntropyFailed = errors.New("falha ao obter entropia criptográfica")
)

// prefixRegex valida prefixos estritamente alfanuméricos e hífen de 1 a 30 caracteres.
var prefixRegex = regexp.MustCompile(`^[a-zA-Z0-9-]{1,30}$`)

const entropyBytes = 16 // 16 bytes = 128 bits de entropia crypto/rand -> 32 hex chars

// ValidatePrefix valida se o prefixo atende ao padrão alfanumérico e hífen (1 a 30 caracteres).
func ValidatePrefix(prefix string) error {
	if !prefixRegex.MatchString(prefix) {
		return ErrInvalidPrefix
	}
	return nil
}

// New gera um identificador seguro com 16 bytes (128 bits) de entropia crypto/rand: "<prefix>-<32 hex chars>".
// Retorna erro caso o prefixo seja inválido ou o gerador de entropia falhe (fail-closed).
func New(prefix string) (string, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return "", err
	}
	var b [entropyBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("%w: %v", ErrEntropyFailed, err)
	}
	return prefix + "-" + hex.EncodeToString(b[:]), nil
}

// HasPrefix valida se o ID gerado possui o prefixo esperado com o delimitador "-".
func HasPrefix(generatedID, prefix string) bool {
	return strings.HasPrefix(generatedID, prefix+"-")
}
