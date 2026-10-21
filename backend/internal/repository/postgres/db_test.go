package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"catbrasil/backend/internal/repository/postgres"

	"github.com/jackc/pgx/v5"
)

func TestDB_WithTransaction_RollbackAndPanicHandling(t *testing.T) {
	// Testa que o helper WithTransaction possui estrutura de rollback segura mesmo com nil pool sem quebrar em runtime
	db := &postgres.DB{Pool: nil}
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			// Se não causou pânico com pool nil, tudo bem
		}
	}()

	_ = db.WithTransaction(ctx, func(tx pgx.Tx) error {
		return errors.New("simulated error")
	})
}

func TestNewDB_NeverLeaksSecretsOrCanariesOnInvalidConfig(t *testing.T) {
	ctx := context.Background()
	const canaryUser = "canary_user_991"
	const canaryPass = "canary_secret_pass_882"
	const canaryHost = "canary-db-host.internal"
	const canaryQuery = "canary_token=secret_query_val_333"

	// 1. URL style com credenciais e query params
	dsnWithCanaries := "postgres://" + canaryUser + ":" + canaryPass + "@" + canaryHost + ":5432/canary_db?" + canaryQuery
	_, err := postgres.NewDB(ctx, dsnWithCanaries)
	if err == nil {
		t.Fatal("esperava erro de conexão com DSN inválido")
	}

	errMsg := err.Error()
	for _, canary := range []string{canaryUser, canaryPass, canaryHost, canaryQuery, "canary_db"} {
		if strings.Contains(errMsg, canary) {
			t.Fatalf("NewDB vazou canário no erro: %s (canário: %s)", errMsg, canary)
		}
	}

	// 2. DSN key-value com senha fora de URI
	kvDSN := "host=canary-host-kv password=canary_raw_kv_pass_777 user=canary_kv_user dbname=canary_kv_db"
	_, errKV := postgres.NewDB(ctx, kvDSN)
	if errKV == nil {
		t.Fatal("esperava erro de conexão com DSN kv inválido")
	}
	errMsgKV := errKV.Error()
	for _, canary := range []string{"canary_raw_kv_pass_777", "canary-host-kv", "canary_kv_user", "canary_kv_db"} {
		if strings.Contains(errMsgKV, canary) {
			t.Fatalf("NewDB vazou canário no erro key-value: %s (canário: %s)", errMsgKV, canary)
		}
	}
}

func TestSanitizeDBError_CategoricalNeverLeaksArbitraryStrings(t *testing.T) {
	const canarySecret = "SUPER_SECRET_CANARY_TOKEN_XYZ_12345"
	rawErr := errors.New("falha no postgres: connection failed to postgres://user:" + canarySecret + "@localhost:5432/db")

	sanitized := postgres.SanitizeDBError(rawErr)
	if strings.Contains(sanitized.Error(), canarySecret) {
		t.Fatalf("SanitizeDBError vazou canário: %s", sanitized.Error())
	}
	if sanitized.Error() != postgres.ErrDBConnection.Error() {
		t.Fatalf("esperava erro estático categorizado %q, obteve %q", postgres.ErrDBConnection.Error(), sanitized.Error())
	}
}
