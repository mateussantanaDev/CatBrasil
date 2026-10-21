package postgres_test

import (
	"context"
	"os"
	"testing"

	"catbrasil/backend/internal/domain"
	"catbrasil/backend/internal/repository/postgres"
	"catbrasil/backend/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func configRepoDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CATBRASIL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina CATBRASIL_TEST_DATABASE_URL para testes PostgreSQL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = migrations.Apply(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = migrations.CheckCurrent(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Prova que update do gateway com segredo vazio NÃO destrói a credencial
// persistida (COALESCE(NULLIF(...))) e que GetConfig continua devolvendo o
// segredo real para o uso interno (redação é exclusiva da camada HTTP).
func TestConfigRepository_UpdateGatewayConfig_EmptySecretPreservesStoredCredential(t *testing.T) {
	pool := configRepoDB(t)
	ctx := context.Background()
	repo := postgres.NewConfigRepository(pool)

	const (
		originalSuitPayID       = "ci-canary-suitpay-id-12345"
		originalSecret          = "cs-canary-secret-integration"
		originalSuitPayCallback = "https://canary.app/cb?token=token_canary_suitpay_123"
		originalParadiseKey     = "pk-canary-paradise-integration"
		originalParadiseHash    = "hash-canary-paradise-product-456"
		originalParadisePost    = "https://canary.app/pb?auth=auth_canary_paradise_789"
	)

	cfg := &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			Name:                 "Gateway PIX Padrão",
			DepositFeePercent:    1.5,
			WithdrawFeeFixed:     1.0,
			PixExpirationMinutes: 15,
			ActiveProvider:       "suitpay",
			SuitPayClientID:      originalSuitPayID,
			SuitPayClientSecret:  originalSecret,
			SuitPayCallbackURL:   originalSuitPayCallback,
			SuitPayEnabled:       true,
			ParadiseSecretKey:    originalParadiseKey,
			ParadiseProductHash:  originalParadiseHash,
			ParadisePostbackURL:  originalParadisePost,
			ParadiseEnabled:      true,
		},
	}
	if err := repo.UpdateConfig(ctx, cfg); err != nil {
		t.Fatalf("seed de configuração falhou: %v", err)
	}

	// Uso interno: GetConfig deve devolver os valores em texto claro para os usecases
	stored, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig falhou: %v", err)
	}
	if stored.Gateway.SuitPayClientID != originalSuitPayID {
		t.Fatalf("GetConfig deve expor SuitPayClientID ao uso interno, obteve %q", stored.Gateway.SuitPayClientID)
	}
	if stored.Gateway.SuitPayClientSecret != originalSecret {
		t.Fatalf("GetConfig deve expor SuitPayClientSecret ao uso interno, obteve %q", stored.Gateway.SuitPayClientSecret)
	}
	if stored.Gateway.SuitPayCallbackURL != originalSuitPayCallback {
		t.Fatalf("GetConfig deve expor SuitPayCallbackURL ao uso interno, obteve %q", stored.Gateway.SuitPayCallbackURL)
	}
	if stored.Gateway.ParadiseSecretKey != originalParadiseKey {
		t.Fatalf("GetConfig deve expor ParadiseSecretKey ao uso interno, obteve %q", stored.Gateway.ParadiseSecretKey)
	}
	if stored.Gateway.ParadiseProductHash != originalParadiseHash {
		t.Fatalf("GetConfig deve expor ParadiseProductHash ao uso interno, obteve %q", stored.Gateway.ParadiseProductHash)
	}
	if stored.Gateway.ParadisePostbackURL != originalParadisePost {
		t.Fatalf("GetConfig deve expor ParadisePostbackURL ao uso interno, obteve %q", stored.Gateway.ParadisePostbackURL)
	}

	// PUT vazio: preserva todas as credenciais e URLs canário, atualiza os demais campos
	if err := repo.UpdateGatewayConfig(ctx, domain.GatewayConfig{
		Name:                 "Gateway PIX Atualizado",
		DepositFeePercent:    2.0,
		WithdrawFeeFixed:     1.5,
		PixExpirationMinutes: 20,
		ActiveProvider:       "suitpay",
		SuitPayClientID:      "",
		SuitPayClientSecret:  "",
		SuitPayCallbackURL:   "",
		SuitPayEnabled:       true,
		ParadiseSecretKey:    "",
		ParadiseProductHash:  "",
		ParadisePostbackURL:  "",
		ParadiseEnabled:      true,
	}, 12); err != nil {
		t.Fatalf("UpdateGatewayConfig com segredo vazio falhou: %v", err)
	}

	preserved, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig após update vazio falhou: %v", err)
	}
	if preserved.Gateway.SuitPayClientID != originalSuitPayID {
		t.Fatalf("SuitPayClientID deveria ser preservado com update vazio, obteve %q", preserved.Gateway.SuitPayClientID)
	}
	if preserved.Gateway.SuitPayClientSecret != originalSecret {
		t.Fatalf("SuitPayClientSecret deveria ser preservado com update vazio, obteve %q", preserved.Gateway.SuitPayClientSecret)
	}
	if preserved.Gateway.SuitPayCallbackURL != originalSuitPayCallback {
		t.Fatalf("SuitPayCallbackURL deveria ser preservada com update vazio, obteve %q", preserved.Gateway.SuitPayCallbackURL)
	}
	if preserved.Gateway.ParadiseSecretKey != originalParadiseKey {
		t.Fatalf("ParadiseSecretKey deveria ser preservada com update vazio, obteve %q", preserved.Gateway.ParadiseSecretKey)
	}
	if preserved.Gateway.ParadiseProductHash != originalParadiseHash {
		t.Fatalf("ParadiseProductHash deveria ser preservado com update vazio, obteve %q", preserved.Gateway.ParadiseProductHash)
	}
	if preserved.Gateway.ParadisePostbackURL != originalParadisePost {
		t.Fatalf("ParadisePostbackURL deveria ser preservada com update vazio, obteve %q", preserved.Gateway.ParadisePostbackURL)
	}
	if preserved.Gateway.Name != "Gateway PIX Atualizado" || preserved.Gateway.DepositFeePercent != 2.0 {
		t.Fatal("campos não sensíveis do gateway devem ser atualizados normalmente")
	}

	// PUT preenchido: rotação permitida
	const (
		rotatedSecret   = "cs-rotated-secret-integration"
		rotatedID       = "ci-rotated-id-integration"
		rotatedCallback = "https://canary.app/cb-rotated"
	)
	if err := repo.UpdateGatewayConfig(ctx, domain.GatewayConfig{
		Name:                 "Gateway PIX Atualizado",
		DepositFeePercent:    2.0,
		WithdrawFeeFixed:     1.5,
		PixExpirationMinutes: 20,
		ActiveProvider:       "suitpay",
		SuitPayClientID:      rotatedID,
		SuitPayClientSecret:  rotatedSecret,
		SuitPayCallbackURL:   rotatedCallback,
		SuitPayEnabled:       true,
		ParadiseSecretKey:    "",
		ParadiseProductHash:  "",
		ParadisePostbackURL:  "",
	}, 12); err != nil {
		t.Fatalf("UpdateGatewayConfig com rotação falhou: %v", err)
	}

	rotated, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig após rotação falhou: %v", err)
	}
	if rotated.Gateway.SuitPayClientID != rotatedID {
		t.Fatalf("SuitPayClientID preenchido deveria substituir o anterior, obteve %q", rotated.Gateway.SuitPayClientID)
	}
	if rotated.Gateway.SuitPayClientSecret != rotatedSecret {
		t.Fatalf("SuitPayClientSecret preenchido deveria substituir o anterior, obteve %q", rotated.Gateway.SuitPayClientSecret)
	}
	if rotated.Gateway.SuitPayCallbackURL != rotatedCallback {
		t.Fatalf("SuitPayCallbackURL preenchida deveria substituir a anterior, obteve %q", rotated.Gateway.SuitPayCallbackURL)
	}
	if rotated.Gateway.ParadiseSecretKey != originalParadiseKey {
		t.Fatalf("ParadiseSecretKey vazia deveria seguir preservada, obteve %q", rotated.Gateway.ParadiseSecretKey)
	}
	if rotated.Gateway.ParadiseProductHash != originalParadiseHash {
		t.Fatalf("ParadiseProductHash vazio deveria seguir preservado, obteve %q", rotated.Gateway.ParadiseProductHash)
	}
	if rotated.Gateway.ParadisePostbackURL != originalParadisePost {
		t.Fatalf("ParadisePostbackURL vazia deveria seguir preservada, obteve %q", rotated.Gateway.ParadisePostbackURL)
	}
}

// UpdateConfig (upsert usado pelo sistema) também preserva credenciais e URLs quando campos vêm vazios.
func TestConfigRepository_UpdateConfig_EmptySecretPreservesStoredCredential(t *testing.T) {
	pool := configRepoDB(t)
	ctx := context.Background()
	repo := postgres.NewConfigRepository(pool)

	const (
		originalSuitPayID       = "ci-upsert-canary-id"
		originalSecret          = "cs-upsert-canary-secret"
		originalSuitPayCallback = "https://canary.app/upsert-cb"
		originalParadiseKey     = "pk-upsert-canary-key"
		originalParadiseHash    = "hash-upsert-canary"
		originalParadisePost    = "https://canary.app/upsert-pb"
	)

	if err := repo.UpdateConfig(ctx, &domain.SystemConfig{
		Gateway: domain.GatewayConfig{
			Name:                "Gateway PIX Padrão",
			SuitPayClientID:     originalSuitPayID,
			SuitPayClientSecret: originalSecret,
			SuitPayCallbackURL:  originalSuitPayCallback,
			ParadiseSecretKey:   originalParadiseKey,
			ParadiseProductHash: originalParadiseHash,
			ParadisePostbackURL: originalParadisePost,
		},
	}); err != nil {
		t.Fatalf("seed de configuração falhou: %v", err)
	}

	if err := repo.UpdateConfig(ctx, &domain.SystemConfig{
		AutoApproveEnabled: true,
		Gateway: domain.GatewayConfig{
			Name:                "Gateway PIX Padrão",
			SuitPayClientID:     "",
			SuitPayClientSecret: "",
			SuitPayCallbackURL:  "",
			ParadiseSecretKey:   "",
			ParadiseProductHash: "",
			ParadisePostbackURL: "",
		},
	}); err != nil {
		t.Fatalf("UpdateConfig com segredos vazios falhou: %v", err)
	}

	got, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig falhou: %v", err)
	}
	if got.Gateway.SuitPayClientID != originalSuitPayID {
		t.Fatalf("upsert destruiu SuitPayClientID, obteve %q", got.Gateway.SuitPayClientID)
	}
	if got.Gateway.SuitPayClientSecret != originalSecret {
		t.Fatalf("upsert destruiu SuitPayClientSecret, obteve %q", got.Gateway.SuitPayClientSecret)
	}
	if got.Gateway.SuitPayCallbackURL != originalSuitPayCallback {
		t.Fatalf("upsert destruiu SuitPayCallbackURL, obteve %q", got.Gateway.SuitPayCallbackURL)
	}
	if got.Gateway.ParadiseSecretKey != originalParadiseKey {
		t.Fatalf("upsert destruiu ParadiseSecretKey, obteve %q", got.Gateway.ParadiseSecretKey)
	}
	if got.Gateway.ParadiseProductHash != originalParadiseHash {
		t.Fatalf("upsert destruiu ParadiseProductHash, obteve %q", got.Gateway.ParadiseProductHash)
	}
	if got.Gateway.ParadisePostbackURL != originalParadisePost {
		t.Fatalf("upsert destruiu ParadisePostbackURL, obteve %q", got.Gateway.ParadisePostbackURL)
	}
	if !got.AutoApproveEnabled {
		t.Fatal("demais campos do upsert devem ser aplicados")
	}
}
