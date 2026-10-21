-- ============================================================================
-- CATBRASIL - MIGRATION 000001: INITIAL RELATIONAL SCHEMA & SEED DATA
-- PostgreSQL 16 Enterprise Grade Architecture
-- ============================================================================

-- 1. EXTENSÕES
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 2. TABELA DE USUÁRIOS & OPERADORES
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    phone VARCHAR(20) DEFAULT '',
    password_hash TEXT NOT NULL,
    security_password VARCHAR(10) DEFAULT '',
    role VARCHAR(20) DEFAULT 'user' NOT NULL, -- 'user', 'admin', 'gestor', 'diretoria'
    balance NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (balance >= 0),
    yield_balance_available NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (yield_balance_available >= 0),
    xp INTEGER DEFAULT 0 NOT NULL CHECK (xp >= 0),
    nivel INTEGER DEFAULT 1 NOT NULL CHECK (nivel >= 1),
    coletas_completed INTEGER DEFAULT 0 NOT NULL CHECK (coletas_completed >= 0),
    trator_rented BOOLEAN DEFAULT FALSE NOT NULL,
    already_spun_today BOOLEAN DEFAULT FALSE NOT NULL,
    promo_unlocked BOOLEAN DEFAULT FALSE NOT NULL,
    free_withdraw_active BOOLEAN DEFAULT FALSE NOT NULL,
    withdraw_locked BOOLEAN DEFAULT FALSE NOT NULL,
    referral_bonus_suspended BOOLEAN DEFAULT FALSE NOT NULL,
    multi_account_flag BOOLEAN DEFAULT FALSE NOT NULL,
    required_rollover NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    current_rollover NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    ip_address VARCHAR(45) DEFAULT '',
    device_fingerprint VARCHAR(255) DEFAULT '',
    two_factor_secret VARCHAR(255) DEFAULT '',
    two_factor_enabled BOOLEAN DEFAULT FALSE NOT NULL,
    
    -- Inventário de Peças Caterpillar (Oficina de Montagem)
    motor_c15_count INTEGER DEFAULT 0 NOT NULL,
    esteira_reforcada_count INTEGER DEFAULT 0 NOT NULL,
    filtro_oleo_count INTEGER DEFAULT 0 NOT NULL,
    turbo_garrett_count INTEGER DEFAULT 0 NOT NULL,
    
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 3. TABELA DO CATÁLOGO DE FROTA (STORE & REINVEST)
CREATE TABLE IF NOT EXISTS fleet_catalog (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(150) NOT NULL,
    category VARCHAR(50) DEFAULT 'PESADA' NOT NULL, -- 'PESADA', 'ESCAVAÇÃO', 'REINVEST'
    price NUMERIC(15, 2) NOT NULL CHECK (price >= 0),
    daily_yield NUMERIC(15, 2) NOT NULL CHECK (daily_yield >= 0),
    return_percent_per_day NUMERIC(5, 2) DEFAULT 0.00 NOT NULL CHECK (return_percent_per_day >= 0),
    duration_days INTEGER DEFAULT 30 NOT NULL CHECK (duration_days > 0),
    total_contract_return NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (total_contract_return >= 0),
    stock_quantity INTEGER DEFAULT 9999 NOT NULL CHECK (stock_quantity >= 0),
    max_purchases_per_user INTEGER DEFAULT 10 NOT NULL CHECK (max_purchases_per_user > 0),
    reinvest_only BOOLEAN DEFAULT FALSE NOT NULL,
    image_url TEXT DEFAULT '',
    active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 4. TABELA DE MÁQUINAS NA GARAGEM (FROTA ATIVA DO OPERADOR)
CREATE TABLE IF NOT EXISTS machines (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    cost NUMERIC(15, 2) NOT NULL CHECK (cost >= 0),
    yield_amount NUMERIC(15, 2) NOT NULL CHECK (yield_amount >= 0),
    last_collected_at TIMESTAMPTZ NOT NULL,
    state VARCHAR(20) DEFAULT 'aguardando' NOT NULL, -- 'aguardando', 'disponivel', 'expirado'
    override_state VARCHAR(20),
    override_seconds_remaining INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 5. TABELA DE REDE DE AFILIADOS / CONVIDADOS
CREATE TABLE IF NOT EXISTS referrals (
    id VARCHAR(64) PRIMARY KEY,
    referrer_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referred_user_id VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL,
    nome VARCHAR(100) NOT NULL,
    status VARCHAR(20) DEFAULT 'inativo' NOT NULL, -- 'ativo', 'inativo'
    deposito NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (deposito >= 0),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 6. TABELA DE TRANSAÇÕES FINANCEIRAS
CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tipo VARCHAR(100) NOT NULL, -- 'Depósito PIX', 'Saque PIX', 'Saque', 'Rendimento [Nome]', 'Aluguel [Nome]', etc.
    valor NUMERIC(15, 2) NOT NULL,
    status VARCHAR(30) DEFAULT 'pendente' NOT NULL, -- 'pendente', 'sucesso', 'rejeitado', 'cancelado'
    external_id VARCHAR(255),
    pix_key VARCHAR(255),
    pix_code TEXT,
    qr_code_url TEXT,
    net_amount NUMERIC(15, 2),
    fee_amount NUMERIC(15, 2),
    valor_efetivo_pago NUMERIC(15, 2),
    paid_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 7. TABELA DE CONFIGURAÇÃO DA ROLETA & PROBABILIDADES
CREATE TABLE IF NOT EXISTS roulette_slices (
    id INTEGER PRIMARY KEY,
    label VARCHAR(100) NOT NULL,
    amount NUMERIC(15, 2) DEFAULT 0.00 NOT NULL CHECK (amount >= 0),
    probability NUMERIC(6, 3) NOT NULL CHECK (probability >= 0 AND probability <= 100),
    color VARCHAR(30) NOT NULL,
    text_color VARCHAR(30) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 8. TABELA DE COMUNICADOS RICH MEDIA (MURAL & MODAIS)
CREATE TABLE IF NOT EXISTS announcements (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    category VARCHAR(100) DEFAULT 'CAMPANHA DA GESTÃO' NOT NULL,
    content TEXT NOT NULL,
    image_url TEXT DEFAULT '',
    cta_link TEXT DEFAULT '',
    cta_text VARCHAR(100) DEFAULT '',
    show_in_welcome_modal BOOLEAN DEFAULT TRUE NOT NULL,
    badge_color VARCHAR(30) DEFAULT '#ffb200' NOT NULL,
    active BOOLEAN DEFAULT TRUE NOT NULL,
    views_count INTEGER DEFAULT 0 NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 9. TABELA DE CONFIGURAÇÃO GLOBAL DO SISTEMA (ERP / GESTÃO)
CREATE TABLE IF NOT EXISTS system_config (
    id INTEGER PRIMARY KEY DEFAULT 1,
    global_withdrawals_locked BOOLEAN DEFAULT FALSE NOT NULL,
    auto_approve_enabled BOOLEAN DEFAULT FALSE NOT NULL,
    auto_approve_withdrawals_under NUMERIC(15, 2) DEFAULT 200.00 NOT NULL,
    referral_commission_percent NUMERIC(5, 2) DEFAULT 5.00 NOT NULL,
    required_referrals_count INTEGER DEFAULT 3 NOT NULL,
    require_referrals_for_withdrawal BOOLEAN DEFAULT TRUE NOT NULL,
    anti_self_referral_lock BOOLEAN DEFAULT TRUE NOT NULL,
    bonus_per_mutirao_amount NUMERIC(15, 2) DEFAULT 50.00 NOT NULL,
    collection_grace_minutes INTEGER DEFAULT 10 NOT NULL,
    gateway_name VARCHAR(100) DEFAULT 'Gateway PIX Padrão' NOT NULL,
    deposit_fee_percent NUMERIC(5, 2) DEFAULT 1.50 NOT NULL,
    withdraw_fee_fixed NUMERIC(15, 2) DEFAULT 1.00 NOT NULL,
    pix_expiration_minutes INTEGER DEFAULT 15 NOT NULL,
    suitpay_client_id VARCHAR(255) DEFAULT '' NOT NULL,
    suitpay_client_secret VARCHAR(255) DEFAULT '' NOT NULL,
    suitpay_sandbox BOOLEAN DEFAULT FALSE NOT NULL,
    suitpay_callback_url TEXT DEFAULT '' NOT NULL,
    suitpay_enabled BOOLEAN DEFAULT FALSE NOT NULL,
    strict_modal_mode BOOLEAN DEFAULT TRUE NOT NULL,
    require_deposit_to_spin BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT single_row CHECK (id = 1)
);

-- 10. TABELA DE OFERTAS PROMO CUSTOMIZADAS POR USUÁRIO OU REDE
CREATE TABLE IF NOT EXISTS user_promos (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    target_type VARCHAR(20) DEFAULT 'user' NOT NULL, -- 'user', 'network', 'all'
    target_id VARCHAR(64) DEFAULT '',
    machine_id VARCHAR(64) NOT NULL REFERENCES fleet_catalog(id) ON DELETE CASCADE,
    custom_price NUMERIC(15, 2) NOT NULL,
    custom_return_percent_per_day NUMERIC(5, 2) NOT NULL,
    custom_max_purchases INTEGER DEFAULT 1 NOT NULL,
    active BOOLEAN DEFAULT TRUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 11. TABELA DE RANKING SEMANAL (CORRIDA DE MINERAÇÃO)
CREATE TABLE IF NOT EXISTS mining_race_weekly (
    user_id VARCHAR(64) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    username VARCHAR(100) NOT NULL,
    weekly_tonnage NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 12. TABELA DE LOGS DE USO & EVENTOS OPERACIONAIS
CREATE TABLE IF NOT EXISTS usage_logs (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    amount NUMERIC(15, 2) DEFAULT 0.00 NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 13. TABELA DE LOGS DE SEGURANÇA & AUDITORIA
CREATE TABLE IF NOT EXISTS security_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL,
    event VARCHAR(100) NOT NULL,
    ip_address VARCHAR(45) DEFAULT '',
    fingerprint VARCHAR(255) DEFAULT '',
    details TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- 14. TABELA DE VERIFICAÇÃO SMS / 2FA TOKENS
CREATE TABLE IF NOT EXISTS sms_verifications (
    id BIGSERIAL PRIMARY KEY,
    phone VARCHAR(20) NOT NULL,
    code VARCHAR(10) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN DEFAULT FALSE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- ============================================================================
-- ÍNDICES DE ALTA PERFORMANCE (B-TREE & COVERING)
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_users_username_lower ON users (LOWER(username));
CREATE INDEX IF NOT EXISTS idx_users_phone ON users (phone);
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
CREATE INDEX IF NOT EXISTS idx_users_device_fingerprint ON users (device_fingerprint) WHERE device_fingerprint != '';
CREATE INDEX IF NOT EXISTS idx_users_ip_address ON users (ip_address) WHERE ip_address != '';

CREATE INDEX IF NOT EXISTS idx_machines_user_id ON machines (user_id);
CREATE INDEX IF NOT EXISTS idx_machines_state ON machines (state);
CREATE INDEX IF NOT EXISTS idx_machines_last_collected ON machines (last_collected_at);

CREATE INDEX IF NOT EXISTS idx_referrals_referrer_id ON referrals (referrer_id);
CREATE INDEX IF NOT EXISTS idx_referrals_status ON referrals (status);
CREATE INDEX IF NOT EXISTS idx_referrals_referred_user_id ON referrals (referred_user_id);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions (status);
CREATE INDEX IF NOT EXISTS idx_transactions_tipo ON transactions (tipo);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at_desc ON transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions (external_id) WHERE external_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_mining_race_tonnage_desc ON mining_race_weekly (weekly_tonnage DESC);
CREATE INDEX IF NOT EXISTS idx_security_logs_user_id ON security_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_security_logs_created_desc ON security_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_announcements_active ON announcements (active);
CREATE INDEX IF NOT EXISTS idx_user_promos_target ON user_promos (target_type, target_id) WHERE active = TRUE;
CREATE INDEX IF NOT EXISTS idx_sms_verifications_phone ON sms_verifications (phone, expires_at);

-- ============================================================================
-- SEED DATA INICIAL
-- ============================================================================

-- 1. Configuração Global Padrão
INSERT INTO system_config (
    id, global_withdrawals_locked, auto_approve_enabled, auto_approve_withdrawals_under,
    referral_commission_percent, required_referrals_count, require_referrals_for_withdrawal,
    anti_self_referral_lock, bonus_per_mutirao_amount, collection_grace_minutes,
    gateway_name, deposit_fee_percent, withdraw_fee_fixed, pix_expiration_minutes,
    strict_modal_mode, require_deposit_to_spin
) VALUES (
    1, FALSE, FALSE, 200.00,
    5.00, 3, TRUE,
    TRUE, 50.00, 10,
    'Gateway PIX Padrão', 1.50, 1.00, 15,
    TRUE, FALSE
) ON CONFLICT (id) DO NOTHING;

-- 2. Catálogo Oficial de Frota
INSERT INTO fleet_catalog (
    id, name, category, price, daily_yield, return_percent_per_day, duration_days, total_contract_return, stock_quantity, max_purchases_per_user, reinvest_only, image_url, active
) VALUES
('trator_d8t', 'Trator de Esteira CAT D8T', 'PESADA', 1500.00, 75.00, 5.00, 30, 2250.00, 50, 1, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_320', 'Escavadeira CAT 320', 'ESCAVAÇÃO', 2500.00, 110.00, 4.40, 30, 3300.00, 30, 2, FALSE, 'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80', TRUE),
('cat_966m', 'Pá Carregadeira CAT 966M', 'PESADA', 350.00, 60.00, 17.14, 30, 1800.00, 100, 5, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_140m', 'Motoniveladora CAT 140M', 'PESADA', 800.00, 144.00, 18.00, 30, 4320.00, 50, 3, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_745', 'Caminhão Articulado CAT 745', 'TRANSPORTE', 2000.00, 380.00, 19.00, 30, 11400.00, 20, 2, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_cs56b', 'Rolo Compactador CAT CS56B', 'PESADA', 5000.00, 1000.00, 20.00, 30, 30000.00, 10, 1, FALSE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
-- Linha Reinvest Turbinado (Exclusiva com saldo gerado de coletas)
('cat_320_reinvest', 'Escavadeira CAT 320 (Reinvest)', 'REINVEST', 150.00, 102.85, 68.56, 7, 720.00, 999, 10, TRUE, 'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80', TRUE),
('cat_966m_reinvest', 'Pá Carregadeira CAT 966M (Reinvest)', 'REINVEST', 350.00, 257.14, 73.46, 7, 1800.00, 999, 10, TRUE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE),
('cat_140m_reinvest', 'Motoniveladora CAT 140M (Reinvest)', 'REINVEST', 800.00, 617.14, 77.14, 7, 4320.00, 999, 10, TRUE, 'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80', TRUE)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    category = EXCLUDED.category,
    price = EXCLUDED.price,
    daily_yield = EXCLUDED.daily_yield,
    return_percent_per_day = EXCLUDED.return_percent_per_day,
    duration_days = EXCLUDED.duration_days,
    total_contract_return = EXCLUDED.total_contract_return,
    reinvest_only = EXCLUDED.reinvest_only,
    image_url = EXCLUDED.image_url,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- 3. Fatias da Roleta Diária
INSERT INTO roulette_slices (id, label, amount, probability, color, text_color) VALUES
(1, '🧩 Motor C15', 0.00, 20.000, '#ffb200', '#111111'),
(2, '🧩 Esteira CAT', 0.00, 20.000, '#111111', '#ffffff'),
(3, '🧩 Filtro Óleo', 0.00, 25.000, '#ffb200', '#111111'),
(4, '🧩 Turbo Garrett', 0.00, 10.000, '#111111', '#ffffff'),
(5, '📦 Caixa CAT', 0.00, 15.000, '#ffb200', '#111111'),
(6, '💰 R$ 5,00', 5.00, 10.000, '#2e7d32', '#ffffff')
ON CONFLICT (id) DO UPDATE SET
    label = EXCLUDED.label,
    amount = EXCLUDED.amount,
    probability = EXCLUDED.probability,
    color = EXCLUDED.color,
    text_color = EXCLUDED.text_color,
    updated_at = CURRENT_TIMESTAMP;

-- Contas e transações de demonstração não são criadas automaticamente.

-- 9. Comunicados Iniciais
INSERT INTO announcements (
    id, title, category, content, image_url, cta_link, cta_text, show_in_welcome_modal, badge_color, active, views_count
) VALUES
(
    'evt-1770000000001', 'Verão CAT: +20% Rendimento', 'CAMPANHA DA GESTÃO',
    'Todas as coletas realizadas na Garagem garantem bônus de produção automático para operadores ativos.',
    'https://images.unsplash.com/photo-1504307651254-35680f356dfd?w=500&q=80',
    'https://catbrasil.app/loja', 'VER MÁQUINAS EM OFERTA', TRUE, '#ffb200', TRUE, 1420
),
(
    'evt-1770000000002', 'Oficina de Peças Liberada', 'ATUALIZAÇÃO DO SISTEMA',
    'Gire a Roleta da Sorte diariamente e junte Motor C15, Esteira, Filtro e Turbo para montar sua Escavadeira CAT 320 a custo ZERO!',
    'https://images.unsplash.com/photo-1579912440868-21d3f6d713c7?w=500&q=80',
    'https://catbrasil.app/pecas', 'IR PARA OFICINA', FALSE, '#2e7d32', TRUE, 890
)
ON CONFLICT (id) DO NOTHING;
