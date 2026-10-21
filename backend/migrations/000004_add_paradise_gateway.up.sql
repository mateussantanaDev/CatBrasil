-- ============================================================================
-- CATBRASIL - MIGRATION 000004: PARADISE PAGS GATEWAY INTEGRATION
-- ============================================================================

-- 1. ADICIONAR CAMPOS DO GATEWAY PARADISE PAGS EM SYSTEM_CONFIG
ALTER TABLE system_config
    ADD COLUMN IF NOT EXISTS gateway_provider VARCHAR(32) DEFAULT 'simulated' NOT NULL,
    ADD COLUMN IF NOT EXISTS paradise_secret_key VARCHAR(255) DEFAULT '' NOT NULL,
    ADD COLUMN IF NOT EXISTS paradise_product_hash VARCHAR(255) DEFAULT '' NOT NULL,
    ADD COLUMN IF NOT EXISTS paradise_postback_url TEXT DEFAULT '' NOT NULL,
    ADD COLUMN IF NOT EXISTS paradise_enabled BOOLEAN DEFAULT false NOT NULL;

-- 2. ADICIONAR CAMPO DE PROVEDOR DE GATEWAY NAS TRANSAÇÕES
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS gateway_provider VARCHAR(32) DEFAULT 'simulated' NOT NULL;

-- 3. CRIAR ÍNDICE PARA OTIMIZAR CONSULTAS DE TRANSAÇÕES POR GATEWAY
CREATE INDEX IF NOT EXISTS idx_transactions_gateway_provider ON transactions (gateway_provider);
