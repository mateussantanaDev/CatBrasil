// bootstrap-admin cria a primeira conta administrativa em um banco greenfield.
// Senha e segredo TOTP são lidos apenas de um terminal interativo.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"catbrasil/backend/migrations"
	"catbrasil/backend/pkg/crypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{3,31}$`)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap administrativo recusado:", err)
		os.Exit(1)
	}
}

func run() error {
	username := flag.String("username", "", "identificador exclusivo do primeiro diretor")
	flag.Parse()
	if flag.NArg() != 0 || !validUsername(*username) {
		return errors.New("informe --username com 4 a 32 caracteres não reservados")
	}
	if os.Getenv("APP_ENV") != "production" && os.Getenv("APP_ENV") != "staging" {
		return errors.New("APP_ENV deve ser production ou staging")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL externo é obrigatório")
	}
	if len(os.Getenv("ADMIN_SECRET_PIN")) < 6 {
		return errors.New("ADMIN_SECRET_PIN externo é obrigatório")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return errors.New("terminal interativo obrigatório; segredos não são aceitos em argv, pipe ou logs")
	}

	checkCtx, checkCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer checkCancel()
	pool, err := pgxpool.New(checkCtx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("DATABASE_URL inválida")
	}
	defer pool.Close()
	if err := pool.Ping(checkCtx); err != nil {
		return errors.New("PostgreSQL indisponível")
	}
	if err := migrations.CheckCurrent(checkCtx, pool); err != nil {
		return fmt.Errorf("migrações incompletas: %w", err)
	}
	var users int
	if err := pool.QueryRow(checkCtx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		return errors.New("não foi possível verificar o estado inicial de usuários")
	}
	if users != 0 {
		return errors.New("bootstrap exige banco sem usuários; não promove nem substitui contas existentes")
	}

	password, err := readHidden("Senha exclusiva do diretor (24 a 72 bytes, 3 classes): ")
	if err != nil {
		return err
	}
	if err := validatePassword(password, *username, os.Getenv("ADMIN_SECRET_PIN")); err != nil {
		return err
	}
	confirmation, err := readHidden("Repita a senha: ")
	if err != nil {
		return err
	}
	if password != confirmation {
		return errors.New("confirmação de senha diferente")
	}
	confirmation = ""

	totpSecret, err := crypto.GenerateTOTPSecret()
	if err != nil {
		return errors.New("falha ao gerar segredo 2FA")
	}
	fmt.Fprintln(os.Stdout, "Registre o TOTP no autenticador. Este segredo aparece uma única vez neste terminal:")
	fmt.Fprintln(os.Stdout, crypto.GenerateTOTPURL(*username, totpSecret))
	code, err := readHidden("Código TOTP atual: ")
	if err != nil {
		return err
	}
	if !crypto.VerifyTOTPCode(totpSecret, code) {
		return errors.New("código TOTP inválido; nenhuma conta foi criada")
	}
	code = ""

	hash, err := crypto.HashPassword(password)
	password = ""
	if err != nil {
		return errors.New("falha ao derivar hash da senha")
	}
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return errors.New("falha ao gerar ID do diretor")
	}
	id := "CB-" + strings.ToUpper(hex.EncodeToString(idBytes))
	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer createCancel()
	if err := createFirstAdmin(createCtx, pool, id, *username, hash, totpSecret); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Primeiro diretor criado com senha bcrypt, 2FA ativo e saldos zerados. ID:", id)
	return nil
}

func validUsername(username string) bool {
	if !usernamePattern.MatchString(username) {
		return false
	}
	lower := strings.ToLower(username)
	if strings.Contains(lower, "demo") || strings.Contains(lower, "seed") {
		return false
	}
	switch lower {
	case "admin", "catadmin", "gestor_master", "teste":
		return false
	}
	return true
}

func validatePassword(password, username, pin string) error {
	if len(password) < 24 || len(password) > 72 || strings.TrimSpace(password) != password {
		return errors.New("senha deve ter 24 a 72 bytes, sem espaços nas extremidades")
	}
	if strings.Contains(strings.ToLower(password), strings.ToLower(username)) || password == pin {
		return errors.New("senha deve ser diferente do usuário e do PIN administrativo")
	}
	var lower, upper, digit, symbol bool
	for _, char := range password {
		switch {
		case char >= 'a' && char <= 'z':
			lower = true
		case char >= 'A' && char <= 'Z':
			upper = true
		case char >= '0' && char <= '9':
			digit = true
		default:
			symbol = true
		}
	}
	classes := 0
	for _, present := range []bool{lower, upper, digit, symbol} {
		if present {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("senha deve combinar ao menos três classes de caracteres")
	}
	return nil
}

func readHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stdout, prompt)
	input, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stdout)
	if err != nil {
		return "", errors.New("falha ao ler segredo do terminal")
	}
	return string(input), nil
}

func createFirstAdmin(ctx context.Context, pool *pgxpool.Pool, id, username, hash, totpSecret string) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return errors.New("falha ao iniciar transação de bootstrap")
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5847192018)`); err != nil {
		return errors.New("falha ao bloquear bootstrap concorrente")
	}
	var users int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		return errors.New("falha ao verificar usuários na transação")
	}
	if users != 0 {
		return errors.New("banco deixou de estar vazio durante o bootstrap")
	}
	_, err = tx.Exec(ctx, `INSERT INTO users
		(id, username, password_hash, role, balance, yield_balance_available, two_factor_secret, two_factor_enabled)
		VALUES ($1, $2, $3, 'diretoria', 0, 0, $4, TRUE)`, id, username, hash, totpSecret)
	if err != nil {
		return errors.New("falha ao criar diretor; confira unicidade e schema")
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("falha ao confirmar transação de bootstrap")
	}
	return nil
}
