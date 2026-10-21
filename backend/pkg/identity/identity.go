package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"unicode"
)

var ErrInvalidCPF = errors.New("CPF inválido")
var ErrInvalidIdentity = errors.New("nome ou email inválido")

type Person struct {
	CPF       string `json:"cpf"`
	LegalName string `json:"legalName"`
	Email     string `json:"email"`
}

func NormalizeCPF(input string) (string, error) {
	var digits strings.Builder
	for _, char := range input {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		} else if char != '.' && char != '-' && char != ' ' {
			return "", ErrInvalidCPF
		}
	}
	cpf := digits.String()
	if len(cpf) != 11 || strings.Count(cpf, cpf[:1]) == 11 {
		return "", ErrInvalidCPF
	}
	for size := 9; size <= 10; size++ {
		sum := 0
		for index := 0; index < size; index++ {
			sum += int(cpf[index]-'0') * (size + 1 - index)
		}
		check := (sum * 10) % 11
		if check == 10 {
			check = 0
		}
		if int(cpf[size]-'0') != check {
			return "", ErrInvalidCPF
		}
	}
	return cpf, nil
}

func NormalizePerson(person Person) (Person, error) {
	var err error
	person.CPF, err = NormalizeCPF(person.CPF)
	if err != nil {
		return Person{}, err
	}
	person.LegalName = strings.Join(strings.Fields(person.LegalName), " ")
	person.Email = strings.ToLower(strings.TrimSpace(person.Email))
	if len([]rune(person.LegalName)) < 5 || len([]rune(person.LegalName)) > 120 || len(strings.Fields(person.LegalName)) < 2 {
		return Person{}, ErrInvalidIdentity
	}
	for _, char := range person.LegalName {
		if unicode.IsControl(char) || unicode.IsDigit(char) {
			return Person{}, ErrInvalidIdentity
		}
	}
	address, err := mail.ParseAddress(person.Email)
	parts := strings.Split(person.Email, "@")
	if err != nil || address.Address != person.Email || len(person.Email) > 254 || len(parts) != 2 || !strings.Contains(parts[1], ".") {
		return Person{}, ErrInvalidIdentity
	}
	return person, nil
}

type Cipher struct {
	aead      cipher.AEAD
	lookupKey [32]byte
}

func NewCipher(secret string) (*Cipher, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("IDENTITY_SECRET deve ter 32 caracteres ou mais")
	}
	encryptionKey := sha256.Sum256([]byte("catbrasil-identity-encryption-v1:" + secret))
	lookupKey := sha256.Sum256([]byte("catbrasil-identity-lookup-v1:" + secret))
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead, lookupKey: lookupKey}, nil
}

func (c *Cipher) Digest(cpf string) string {
	hash := hmac.New(sha256.New, c.lookupKey[:])
	_, _ = hash.Write([]byte(cpf))
	return hex.EncodeToString(hash.Sum(nil))
}

func (c *Cipher) Encrypt(person Person) ([]byte, error) {
	plain, err := json.Marshal(person)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plain, nil), nil
}

func (c *Cipher) Decrypt(data []byte) (Person, error) {
	nonceSize := c.aead.NonceSize()
	if len(data) < nonceSize {
		return Person{}, errors.New("dados de identidade inválidos")
	}
	plain, err := c.aead.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return Person{}, errors.New("não foi possível decifrar identidade")
	}
	var person Person
	if err := json.Unmarshal(plain, &person); err != nil {
		return Person{}, errors.New("dados de identidade inválidos")
	}
	return person, nil
}
