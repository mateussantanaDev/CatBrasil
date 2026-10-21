package postgres_test

import (
	"strings"
	"testing"

	"catbrasil/backend/migrations"
)

func TestSchema_MigrationIntegrity(t *testing.T) {
	upSQL := migrations.SchemaUpSQL
	downSQL := migrations.SchemaDownSQL

	if len(upSQL) == 0 {
		t.Fatal("SchemaUpSQL está vazio ou não foi devidamente embutido via //go:embed")
	}

	if len(downSQL) == 0 {
		t.Fatal("SchemaDownSQL está vazio ou não foi devidamente embutido via //go:embed")
	}

	expectedTables := []string{
		"users",
		"fleet_catalog",
		"machines",
		"referrals",
		"transactions",
		"roulette_slices",
		"announcements",
		"system_config",
		"user_promos",
		"mining_race_weekly",
		"usage_logs",
		"security_logs",
		"sms_verifications",
	}

	// 1. Verifica presença de todas as 13 tabelas na migração UP
	for _, tbl := range expectedTables {
		createPattern := "CREATE TABLE IF NOT EXISTS " + tbl
		if !strings.Contains(upSQL, createPattern) {
			t.Errorf("Tabela obrigatória '%s' não encontrada em 000001_init_schema.up.sql", tbl)
		}
	}

	// 2. Verifica presença de todas as 13 tabelas no rollback DOWN
	for _, tbl := range expectedTables {
		dropPattern := "DROP TABLE IF EXISTS " + tbl + " CASCADE;"
		if !strings.Contains(downSQL, dropPattern) {
			t.Errorf("Tabela obrigatória '%s' não possui comando de DROP CASCADE em 000001_init_schema.down.sql", tbl)
		}
	}

	// 3. Verifica Constraints Financeiras de Segurança no UP
	requiredConstraints := []string{
		"CHECK (balance >= 0)",
		"CHECK (yield_balance_available >= 0)",
		"CHECK (price >= 0)",
		"CHECK (daily_yield >= 0)",
		"CHECK (cost >= 0)",
		"CHECK (yield_amount >= 0)",
		"CHECK (probability >= 0 AND probability <= 100)",
	}

	for _, c := range requiredConstraints {
		if !strings.Contains(upSQL, c) {
			t.Errorf("Constraint financeira obrigatória '%s' ausente no schema UP", c)
		}
	}

	// 4. Verifica Índices de Performance Críticos
	requiredIndexes := []string{
		"idx_users_username_lower",
		"idx_users_phone",
		"idx_machines_user_id",
		"idx_transactions_user_id",
		"idx_transactions_created_at_desc",
		"idx_mining_race_tonnage_desc",
	}

	for _, idx := range requiredIndexes {
		if !strings.Contains(upSQL, idx) {
			t.Errorf("Índice de alta performance '%s' ausente no schema UP", idx)
		}
	}

	// 5. Verifica Seeds Oficiais de Frota e Diretoria
	requiredSeeds := []string{
		"trator_d8t",
		"cat_320",
		"cat_966m",
		"cat_140m",
		"cat_745",
		"cat_cs56b",
		"cat_320_reinvest",
	}
	for _, demoID := range []string{"gst-001", "CB-458F"} {
		if strings.Contains(upSQL, demoID) {
			t.Errorf("Conta de demonstração '%s' não deve ser criada pela migração", demoID)
		}
	}

	for _, seed := range requiredSeeds {
		if !strings.Contains(upSQL, seed) {
			t.Errorf("Seed oficial '%s' ausente em 000001_init_schema.up.sql", seed)
		}
	}
}
