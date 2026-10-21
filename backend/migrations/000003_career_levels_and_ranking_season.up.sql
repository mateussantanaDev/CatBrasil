-- ============================================================================
-- CATBRASIL - MIGRATION 000003: PERPETUAL CAREER LEVELS & LEVEL 12 MAINTENANCE
-- ============================================================================

-- 1. CONFIGURAÇÕES DE MANUTENÇÃO DO NÍVEL 12 NO ERP
ALTER TABLE system_config
    ADD COLUMN IF NOT EXISTS level12_maintenance_days INTEGER DEFAULT 30 NOT NULL,
    ADD COLUMN IF NOT EXISTS level12_required_referrals INTEGER DEFAULT 1 NOT NULL;

-- 2. CAMPOS DE RASTREABILIDADE DE CONVITES E STATUS NO NÍVEL 12 EM USERS
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_referral_created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS level12_reached_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS level12_bonus_suspended BOOLEAN DEFAULT FALSE NOT NULL;

-- 3. TABELA DEDICADA PARA HISTÓRICO DE TEMPORADAS DE RANKING (75 DIAS)
CREATE TABLE IF NOT EXISTS ranking_season_history (
    id BIGSERIAL PRIMARY KEY,
    season_number INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    rank_position INTEGER NOT NULL,
    tonnage_extracted NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    reward_granted TEXT DEFAULT '' NOT NULL,
    closed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ranking_season_user ON ranking_season_history (user_id, season_number);
CREATE INDEX IF NOT EXISTS idx_ranking_season_pos ON ranking_season_history (season_number, rank_position);

-- 4. AJUSTE SEMÂNTICO NAS LABELS DE NÍVEL (REMOVENDO TERMOS DE TEMPORADA DE PASSE)
UPDATE level_configs 
SET special_reward_text = 'Boas-vindas ao Plano de Carreira Profissional CAT'
WHERE level = 1 AND special_reward_text LIKE '%Temporada%';

