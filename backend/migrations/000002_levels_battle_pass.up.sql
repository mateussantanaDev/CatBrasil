-- ============================================================================
-- CATBRASIL - MIGRATION 000002: BATTLE PASS SEASONS & CONFIGURABLE 12 LEVELS
-- ============================================================================

-- 1. NOVAS MÁQUINAS RÁPIDAS E RARAS NO CATÁLOGO
INSERT INTO fleet_catalog (
    id, name, category, price, daily_yield, return_percent_per_day, duration_days, total_contract_return, stock_quantity, max_purchases_per_user, reinvest_only, image_url, active
) VALUES
('cat_301_mini', 'Mini Escavadeira CAT 301.7 (Ciclo Rápido)', 'COMPACTA', 80.00, 20.00, 25.00, 5, 100.00, 999, 5, FALSE, 'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80', TRUE),
('cat_416f2', 'Retroescavadeira CAT 416F2 (Ciclo Médio)', 'ESCAVAÇÃO', 250.00, 45.00, 18.00, 7, 315.00, 999, 5, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_924k', 'Pá Carregadeira CAT 924K Rápida', 'PESADA', 400.00, 65.00, 16.25, 8, 520.00, 999, 5, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_d6r_titan', 'Trator de Esteiras CAT D6R Titan Raro', 'RARA', 1200.00, 130.00, 10.83, 15, 1950.00, 50, 2, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_770g_gold', 'Caminhão Fora de Estrada CAT 770G Gold', 'RARA', 2500.00, 280.00, 11.20, 20, 5600.00, 30, 2, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_349_ultra', 'Escavadeira CAT 349 Ultra Demolição', 'RARA', 4000.00, 500.00, 12.50, 20, 10000.00, 20, 1, FALSE, 'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80', TRUE),
('cat_988k_mining', 'Carregadeira Gigante CAT 988K Mining Edition', 'RARA', 8000.00, 1100.00, 13.75, 30, 33000.00, 10, 1, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    price = EXCLUDED.price,
    daily_yield = EXCLUDED.daily_yield,
    return_percent_per_day = EXCLUDED.return_percent_per_day,
    duration_days = EXCLUDED.duration_days,
    total_contract_return = EXCLUDED.total_contract_return,
    image_url = EXCLUDED.image_url,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- 2. TABELA DE CONFIGURAÇÃO DOS 12 NÍVEIS (EDITÁVEL NO ERP)
CREATE TABLE IF NOT EXISTS level_configs (
    level INTEGER PRIMARY KEY CHECK (level >= 1 AND level <= 12),
    title VARCHAR(150) NOT NULL,
    tier_name VARCHAR(50) NOT NULL, -- 'TIER 1 - OPERADOR', 'TIER 2 - ESPECIALISTA', 'TIER 3 - ELITE'
    required_referrals INTEGER DEFAULT 0 NOT NULL CHECK (required_referrals >= 0),
    required_machines INTEGER DEFAULT 0 NOT NULL CHECK (required_machines >= 0),
    required_coletas INTEGER DEFAULT 0 NOT NULL CHECK (required_coletas >= 0),
    required_tonnage NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (required_tonnage >= 0),
    unlocked_machine_id VARCHAR(64) REFERENCES fleet_catalog(id) ON DELETE SET NULL,
    commission_percent NUMERIC(5, 2) DEFAULT 5.00 NOT NULL CHECK (commission_percent >= 0 AND commission_percent <= 100),
    bonus_yield_percent NUMERIC(5, 2) DEFAULT 0.00 NOT NULL CHECK (bonus_yield_percent >= 0),
    has_insurance_voucher BOOLEAN DEFAULT FALSE NOT NULL,
    insurance_days_recovered INTEGER DEFAULT 0 NOT NULL CHECK (insurance_days_recovered >= 0),
    has_extra_roulette_spin BOOLEAN DEFAULT FALSE NOT NULL,
    has_vip_roulette BOOLEAN DEFAULT FALSE NOT NULL,
    max_machines_allowed INTEGER DEFAULT 15 NOT NULL CHECK (max_machines_allowed >= 1),
    special_reward_text TEXT DEFAULT '' NOT NULL,
    badge_name VARCHAR(100) DEFAULT '' NOT NULL,
    badge_icon VARCHAR(50) DEFAULT '' NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 3. POPULAR OS 12 NÍVEIS INICIAIS BASEADOS NAS REGRAS
INSERT INTO level_configs (
    level, title, tier_name, required_referrals, required_machines, required_coletas, required_tonnage,
    unlocked_machine_id, commission_percent, bonus_yield_percent, has_insurance_voucher, insurance_days_recovered,
    has_extra_roulette_spin, has_vip_roulette, max_machines_allowed, special_reward_text, badge_name, badge_icon
) VALUES
(1, 'Operador Auxiliar', 'TIER 1 - OPERADOR', 0, 0, 0, 0.00, NULL, 5.00, 0.00, FALSE, 0, FALSE, FALSE, 15, 'Boas-vindas à Temporada', 'Novato CAT', '🚜'),
(2, 'Operador de Pátio', 'TIER 1 - OPERADOR', 2, 1, 3, 500.00, 'cat_301_mini', 5.00, 0.50, FALSE, 0, FALSE, FALSE, 15, 'Liberação da Mini Escavadeira CAT 301.7 (Ciclo Rápido de 5 dias)', 'Operador de Pátio', '🛠️'),
(3, 'Operador Técnico', 'TIER 1 - OPERADOR', 4, 2, 6, 1500.00, 'cat_416f2', 5.00, 1.00, FALSE, 0, FALSE, FALSE, 15, 'Liberação da Retroescavadeira CAT 416F2 + 1x Peça Misteriosa de Oficina', 'Técnico de Frota', '⚙️'),
(4, 'Operador Sênior de Frente', 'TIER 1 - OPERADOR', 6, 3, 10, 3000.00, 'cat_924k', 5.00, 1.50, FALSE, 0, FALSE, FALSE, 15, 'Liberação da Pá Carregadeira CAT 924K Rápida + 500 XP', 'Sênior de Frente', '🏗️'),
(5, 'Mestre de Turno', 'TIER 1 - OPERADOR', 8, 5, 15, 5000.00, NULL, 5.00, 2.00, TRUE, 2, FALSE, FALSE, 15, '🛡️ VALE SEGURO CAT: Recupera 100% de rendimentos de até 2 dias de coletas perdidas!', 'Mestre de Turno', '🛡️'),
(6, 'Supervisor de Mina', 'TIER 2 - ESPECIALISTA', 9, 5, 20, 8000.00, 'cat_d6r_titan', 6.00, 3.00, FALSE, 0, TRUE, FALSE, 15, '🎰 1x Giro de Roleta Extra + Comissão da Rede sobe para 6% + Trator CAT D6R Titan Raro', 'Supervisor de Mina', '👷'),
(7, 'Inspetor Técnico de Mina', 'TIER 2 - ESPECIALISTA', 10, 6, 25, 12000.00, 'cat_770g_gold', 7.00, 3.50, FALSE, 0, TRUE, FALSE, 15, '🎰 1x Giro de Roleta Extra + Comissão em 7% + Caminhão CAT 770G Gold Raro', 'Inspetor Técnico', '🔍'),
(8, 'Engenheiro Chefe de Frota', 'TIER 2 - ESPECIALISTA', 11, 7, 30, 18000.00, 'cat_349_ultra', 7.00, 4.00, FALSE, 0, TRUE, FALSE, 15, '🎰 1x Giro de Roleta Extra + Escavadeira CAT 349 Ultra Demolição Rara', 'Engenheiro Chefe', '📐'),
(9, 'Gerente Geral de Operações', 'TIER 2 - ESPECIALISTA', 12, 8, 35, 25000.00, 'cat_988k_mining', 7.50, 4.50, FALSE, 0, TRUE, FALSE, 15, '🎰 1x Giro de Roleta Extra + Carregadeira Gigante CAT 988K Mining Edition', 'Gerente Geral', '💼'),
(10, 'Comandante de Distrito', 'TIER 2 - ESPECIALISTA', 12, 8, 40, 35000.00, NULL, 8.00, 5.00, FALSE, 0, TRUE, FALSE, 15, '🎰 1x Giro de Roleta Extra + 💰 COMISSÃO OFICIAL DE 8% ATIVA EM TODAS AS RECARGAS DA REDE', 'Comandante de Distrito', '🎖️'),
(11, 'Diretor Geral de Mineração', 'TIER 3 - ELITE EXECUTIVA', 14, 10, 50, 50000.00, NULL, 9.00, 6.00, FALSE, 0, FALSE, TRUE, 20, '🎰 Roleta Especial VIP + Garagem expandida para 20 Máquinas + Comissão sobe para 9%', 'Diretor Geral', '🏅'),
(12, 'Comandante Supremo da Frota', 'TIER 3 - ELITE EXECUTIVA', 15, 12, 65, 75000.00, NULL, 10.00, 7.50, FALSE, 0, FALSE, TRUE, 25, '📱 SMARTPHONE DE ÚLTIMA GERAÇÃO (OU R$ 3.500 PIX) + 👑 PLACA DE NÍVEL MÁXIMO + COMISSÃO SUPREMA DE 10% + 25 MÁQUINAS', 'Comandante Supremo (Hall da Fama)', '👑')
ON CONFLICT (level) DO UPDATE SET
    title = EXCLUDED.title,
    tier_name = EXCLUDED.tier_name,
    required_referrals = EXCLUDED.required_referrals,
    required_machines = EXCLUDED.required_machines,
    required_coletas = EXCLUDED.required_coletas,
    required_tonnage = EXCLUDED.required_tonnage,
    unlocked_machine_id = EXCLUDED.unlocked_machine_id,
    commission_percent = EXCLUDED.commission_percent,
    bonus_yield_percent = EXCLUDED.bonus_yield_percent,
    has_insurance_voucher = EXCLUDED.has_insurance_voucher,
    insurance_days_recovered = EXCLUDED.insurance_days_recovered,
    has_extra_roulette_spin = EXCLUDED.has_extra_roulette_spin,
    has_vip_roulette = EXCLUDED.has_vip_roulette,
    max_machines_allowed = EXCLUDED.max_machines_allowed,
    special_reward_text = EXCLUDED.special_reward_text,
    badge_name = EXCLUDED.badge_name,
    badge_icon = EXCLUDED.badge_icon,
    updated_at = CURRENT_TIMESTAMP;

-- 4. ADICIONAR COLUNAS DE TEMPORADA NA TABELA SYSTEM_CONFIG
ALTER TABLE system_config 
    ADD COLUMN IF NOT EXISTS season_duration_days INTEGER DEFAULT 75 NOT NULL,
    ADD COLUMN IF NOT EXISTS current_season_number INTEGER DEFAULT 1 NOT NULL,
    ADD COLUMN IF NOT EXISTS season_started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL;

-- 5. ADICIONAR COLUNAS DE PASSE E VOUCHER NA TABELA USERS
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS insurance_vouchers_count INTEGER DEFAULT 0 NOT NULL,
    ADD COLUMN IF NOT EXISTS max_machines_allowed INTEGER DEFAULT 15 NOT NULL,
    ADD COLUMN IF NOT EXISTS has_max_level_plaque BOOLEAN DEFAULT FALSE NOT NULL,
    ADD COLUMN IF NOT EXISTS season_number INTEGER DEFAULT 1 NOT NULL,
    ADD COLUMN IF NOT EXISTS custom_referral_commission_percent NUMERIC(5, 2) DEFAULT 5.00 NOT NULL;

-- 6. TABELA DE HISTÓRICO DE TEMPORADAS / HALL DA FAMA
CREATE TABLE IF NOT EXISTS battle_pass_season_history (
    id BIGSERIAL PRIMARY KEY,
    season_number INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    final_level INTEGER NOT NULL,
    final_tonnage NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    achieved_max_level BOOLEAN DEFAULT FALSE NOT NULL,
    claimed_physical_reward BOOLEAN DEFAULT FALSE NOT NULL,
    closed_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_season_history_user ON battle_pass_season_history (user_id, season_number);
CREATE INDEX IF NOT EXISTS idx_level_configs_level ON level_configs (level);
