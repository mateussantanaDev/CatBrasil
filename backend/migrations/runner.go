package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.up.sql
var upFiles embed.FS

type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

type Result struct {
	Baselined []int
	Applied   []int
}

func Available() ([]Migration, error) {
	names, err := fs.Glob(upFiles, "*.up.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, errors.New("nenhuma migração SQL embutida")
	}
	migrations := make([]Migration, 0, len(names))
	for index, name := range names {
		prefix, _, ok := strings.Cut(name, "_")
		version, err := strconv.Atoi(prefix)
		if !ok || err != nil || version != index+1 {
			return nil, fmt.Errorf("sequência de migrações inválida em %s", name)
		}
		body, err := upFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(body)
		migrations = append(migrations, Migration{Version: version, Name: name, SQL: string(body), Checksum: hex.EncodeToString(hash[:])})
	}
	return migrations, nil
}

func LatestVersion() (int, error) {
	all, err := Available()
	if err != nil {
		return 0, err
	}
	return all[len(all)-1].Version, nil
}

// Apply executa todas as migrações pendentes em uma transação e sob lock global.
// Um schema legado sem histórico é reconhecido pela estrutura e marcado sem replay de seeds.
func Apply(ctx context.Context, pool *pgxpool.Pool) (Result, error) {
	var result Result
	all, err := Available()
	if err != nil {
		return result, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(5847192016)`); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum CHAR(64) NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return result, fmt.Errorf("criar histórico de migrações: %w", err)
	}
	known, err := appliedVersions(ctx, tx)
	if err != nil {
		return result, err
	}
	if len(known) == 0 {
		legacyVersion, err := detectLegacyVersion(ctx, tx)
		if err != nil {
			return result, err
		}
		for _, migration := range all {
			if migration.Version > legacyVersion {
				break
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`, migration.Version, migration.Name, migration.Checksum); err != nil {
				return result, err
			}
			known[migration.Version] = migration.Checksum
			result.Baselined = append(result.Baselined, migration.Version)
		}
	}
	if err := validateHistory(all, known); err != nil {
		return result, err
	}
	for _, migration := range all {
		if _, ok := known[migration.Version]; ok {
			continue
		}
		if _, err := tx.Exec(ctx, migration.SQL, pgx.QueryExecModeSimpleProtocol); err != nil {
			return result, fmt.Errorf("migração %s falhou: %w", migration.Name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`, migration.Version, migration.Name, migration.Checksum); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, migration.Version)
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

// CheckCurrent é usada pelo startup da API e pelo endpoint de readiness.
func CheckCurrent(ctx context.Context, pool *pgxpool.Pool) error {
	all, err := Available()
	if err != nil {
		return err
	}
	known, err := appliedVersions(ctx, pool)
	if err != nil {
		return fmt.Errorf("histórico de migrações indisponível: %w", err)
	}
	if err := validateHistory(all, known); err != nil {
		return err
	}
	if len(known) != len(all) {
		return fmt.Errorf("schema desatualizado: %d de %d migrações aplicadas", len(known), len(all))
	}
	structureVersion, err := detectLegacyVersion(ctx, pool)
	if err != nil {
		return err
	}
	// A checagem de tabelas e colunas reconhece até a versão 10. As versões 11
	// e 12 alteram dados, defaults e restrições sem novas tabelas ou colunas.
	expectedStructure := all[len(all)-1].Version
	if expectedStructure >= 11 {
		expectedStructure = 10
	}
	if structureVersion != expectedStructure {
		return fmt.Errorf("estrutura do schema incompleta: versão %d de %d", structureVersion, expectedStructure)
	}
	if all[len(all)-1].Version >= 12 {
		var pinConstraint bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_constraint
			WHERE conrelid = 'users'::regclass AND conname = 'users_security_password_bcrypt_only'
		)`).Scan(&pinConstraint); err != nil {
			return err
		}
		if !pinConstraint {
			return errors.New("restrição de hash do PIN financeiro ausente")
		}
	}
	if all[len(all)-1].Version >= 13 {
		var rateTable bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.http_rate_limits') IS NOT NULL`).Scan(&rateTable); err != nil {
			return err
		}
		if !rateTable {
			return errors.New("tabela de rate limit compartilhado ausente")
		}
	}
	if all[len(all)-1].Version >= 14 {
		var cronTable bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.cron_executions') IS NOT NULL`).Scan(&cronTable); err != nil {
			return err
		}
		if !cronTable {
			return errors.New("tabela de execuções cron ausente")
		}
		hasPeriodStart, err := hasColumn(ctx, pool, "mining_race_weekly", "period_start")
		if err != nil {
			return err
		}
		if !hasPeriodStart {
			return errors.New("coluna period_start ausente em mining_race_weekly")
		}
		hasLastSpin, err := hasColumn(ctx, pool, "users", "last_spin_date")
		if err != nil {
			return err
		}
		if !hasLastSpin {
			return errors.New("coluna last_spin_date ausente em users")
		}
	}
	return nil
}

func Status(ctx context.Context, pool *pgxpool.Pool) (int, int, error) {
	all, err := Available()
	if err != nil {
		return 0, 0, err
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return 0, 0, err
	}
	if !exists {
		return 0, len(all), nil
	}
	known, err := appliedVersions(ctx, pool)
	if err != nil {
		return 0, 0, err
	}
	if err := validateHistory(all, known); err != nil {
		return 0, 0, err
	}
	return len(known), len(all), nil
}

type rowQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func appliedVersions(ctx context.Context, db rowQuerier) (map[int]string, error) {
	rows, err := db.Query(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		known[version] = strings.TrimSpace(checksum)
	}
	return known, rows.Err()
}

func validateHistory(all []Migration, known map[int]string) error {
	if len(known) > len(all) {
		return errors.New("banco contém versão de migração mais nova que esta API")
	}
	for index, migration := range all {
		checksum, exists := known[migration.Version]
		if index < len(known) && !exists {
			return fmt.Errorf("lacuna no histórico de migrações na versão %d", migration.Version)
		}
		if exists && checksum != migration.Checksum {
			return fmt.Errorf("checksum alterado na migração %s", migration.Name)
		}
	}
	return nil
}

func detectLegacyVersion(ctx context.Context, db rowQuerier) (int, error) {
	baseTables := []string{"users", "fleet_catalog", "machines", "referrals", "transactions", "roulette_slices", "announcements", "system_config", "user_promos", "mining_race_weekly", "usage_logs", "security_logs", "sms_verifications"}
	baseCount := 0
	for _, table := range baseTables {
		exists, err := hasTable(ctx, db, table)
		if err != nil {
			return 0, err
		}
		if exists {
			baseCount++
		}
	}
	if baseCount == 0 {
		for _, table := range []string{"level_configs", "battle_pass_season_history", "ranking_season_history"} {
			exists, err := hasTable(ctx, db, table)
			if err != nil {
				return 0, err
			}
			if exists {
				return 0, fmt.Errorf("schema parcial: tabela %s sem schema base", table)
			}
		}
		return 0, nil
	}
	if baseCount != len(baseTables) {
		return 0, fmt.Errorf("schema base parcial (%d/%d tabelas): restaure ou repare antes de migrar", baseCount, len(baseTables))
	}
	version := 1
	checks := [][]schemaFeature{
		{{"table", "level_configs", ""}, {"table", "battle_pass_season_history", ""}, {"column", "system_config", "season_duration_days"}, {"column", "users", "insurance_vouchers_count"}},
		{{"table", "ranking_season_history", ""}, {"column", "system_config", "level12_maintenance_days"}, {"column", "users", "last_referral_created_at"}},
		{{"column", "system_config", "gateway_provider"}, {"column", "system_config", "paradise_secret_key"}, {"column", "transactions", "gateway_provider"}},
		{{"column", "transactions", "idempotency_key"}, {"column", "transactions", "payout_attempted_at"}, {"column", "users", "version"}, {"table", "financial_events", ""}, {"table", "payout_gateway_events", ""}},
		{{"column", "sms_verifications", "attempts"}},
		{{"table", "user_identities", ""}, {"table", "identity_events", ""}, {"column", "user_identities", "payload_ciphertext"}},
		{{"constraint", "identity_events", "inspected"}},
		{{"table", "deposit_gateway_events", ""}, {"table", "cashin_deposit_intents", ""}, {"table", "cashin_event_queue", ""}},
		{{"table", "payout_submissions", ""}, {"table", "payout_event_queue", ""}},
	}
	missingEarlier := false
	for _, group := range checks {
		found := 0
		for _, feature := range group {
			var exists bool
			var err error
			switch feature.kind {
			case "table":
				exists, err = hasTable(ctx, db, feature.table)
			case "column":
				exists, err = hasColumn(ctx, db, feature.table, feature.column)
			case "constraint":
				exists, err = hasIdentityInspectionConstraint(ctx, db)
			}
			if err != nil {
				return 0, err
			}
			if exists {
				found++
			}
		}
		if found != 0 && found != len(group) {
			return 0, errors.New("schema legado parcialmente migrado: revisão manual necessária")
		}
		if found == 0 {
			missingEarlier = true
			continue
		}
		if missingEarlier {
			return 0, errors.New("schema legado com lacuna de migrações")
		}
		version++
	}
	return version, nil
}

type schemaFeature struct{ kind, table, column string }

func hasTable(ctx context.Context, db rowQuerier, table string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists)
	return exists, err
}

func hasColumn(ctx context.Context, db rowQuerier, table, column string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2)`, table, column).Scan(&exists)
	return exists, err
}

func hasIdentityInspectionConstraint(ctx context.Context, db rowQuerier) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace WHERE n.nspname='public' AND t.relname='identity_events' AND c.conname='identity_events_action_check' AND pg_get_constraintdef(c.oid) LIKE '%inspected%')`).Scan(&exists)
	return exists, err
}
