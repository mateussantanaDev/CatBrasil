package crypto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GenerateTOTPSecret gera um segredo Base32 de 20 bytes para 2FA
func GenerateTOTPSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateTOTPCode gera o código de 6 dígitos para o segredo e timestamp fornecidos
func GenerateTOTPCode(secret string, timestamp time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		return "", fmt.Errorf("segredo 2FA inválido: %w", err)
	}

	timeStep := uint64(timestamp.Unix() / 30)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timeStep)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff
	code = code % 1000000

	return fmt.Sprintf("%06d", code), nil
}

// VerifyTOTPCode verifica se o código informado confere com a janela de tempo atual (-30s a +30s)
func VerifyTOTPCode(secret, code string) bool {
	if len(code) != 6 {
		return false
	}
	now := time.Now()
	// Tolera pequena variação de relógio do usuário (-1, 0, +1 intervalos de 30s)
	for _, offset := range []time.Duration{-30 * time.Second, 0, 30 * time.Second} {
		expected, err := GenerateTOTPCode(secret, now.Add(offset))
		if err == nil && expected == code {
			return true
		}
	}
	return false
}

// GenerateTOTPURL gera o URI padrão otpauth para leitura em Google Authenticator / Authy
func GenerateTOTPURL(username, secret string) string {
	issuer := "CatBrasil Mining"
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(issuer),
		url.PathEscape(username),
		secret,
		url.QueryEscape(issuer),
	)
}
