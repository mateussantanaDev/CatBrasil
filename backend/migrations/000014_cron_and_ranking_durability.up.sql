-- ============================================================================
-- CATBRASIL - MIGRATION 000014: CRON EXECUTIONS & RANKING PERIOD COMPOSITE KEY
-- ============================================================================

-- 1. TABELA DE EXECUÇÕES PERSISTIDAS DO CRON COM EXCLUSÃO MÚTUA POR PERÍODO
CREATE TABLE IF NOT EXISTS cron_executions (
    id BIGSERIAL PRIMARY KEY,
    job_type VARCHAR(64) NOT NULL,
    period_key VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    attempt_count INTEGER NOT NULL DEFAULT 1,
    last_error TEXT DEFAULT '',
    metadata JSONB DEFAULT '{}'::jsonb,
    CONSTRAINT uq_cron_executions_job_period UNIQUE (job_type, period_key)
);

CREATE INDEX IF NOT EXISTS idx_cron_executions_type_period ON cron_executions (job_type, period_key);
CREATE INDEX IF NOT EXISTS idx_cron_executions_status ON cron_executions (status);

-- 2. EVOLUÇÃO DA TABELA MINING_RACE_WEEKLY COM PERIOD_START E CHAVE COMPOSTA (AMERICA/RECIFE)
ALTER TABLE mining_race_weekly
    ADD COLUMN IF NOT EXISTS period_start TIMESTAMPTZ;

-- Backfill real em todas as linhas existentes usando a data de updated_at
UPDATE mining_race_weekly
SET period_start = (date_trunc('week', COALESCE(updated_at, CURRENT_TIMESTAMP) AT TIME ZONE 'America/Recife') AT TIME ZONE 'America/Recife');

ALTER TABLE mining_race_weekly
    ALTER COLUMN period_start SET NOT NULL,
    ALTER COLUMN period_start SET DEFAULT (date_trunc('week', CURRENT_TIMESTAMP AT TIME ZONE 'America/Recife') AT TIME ZONE 'America/Recife');

-- Remove chave primária simples legada e aplica chave composta (period_start, user_id)
ALTER TABLE mining_race_weekly DROP CONSTRAINT IF EXISTS mining_race_weekly_pkey;
ALTER TABLE mining_race_weekly ADD PRIMARY KEY (period_start, user_id);

CREATE INDEX IF NOT EXISTS idx_mining_race_period_tonnage ON mining_race_weekly (period_start, weekly_tonnage DESC, user_id ASC);

-- 3. RASTREABILIDADE DE GIRO DIÁRIO EM USERS PARA PREVENIR DUPLO GIRO EM RESTARTS DO CRON
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_spin_date VARCHAR(10),
    ADD COLUMN IF NOT EXISTS last_spin_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_last_spin_date ON users (last_spin_date);
