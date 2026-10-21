-- P0.10: a trilha greenfield não publica ofertas, prêmios ou campanhas fictícios.
-- Migrações anteriores são imutáveis por checksum; o saneamento é versionado aqui.
-- Bancos que já possuem usuários não são alterados por este saneamento: exigem
-- avaliação operacional explícita e nunca passam pelo preflight greenfield.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM users)
       AND NOT EXISTS (SELECT 1 FROM machines)
       AND NOT EXISTS (SELECT 1 FROM transactions) THEN
        DELETE FROM level_configs;
        DELETE FROM fleet_catalog;
        DELETE FROM roulette_slices;
        DELETE FROM announcements;

        UPDATE system_config SET
            global_withdrawals_locked = TRUE,
            auto_approve_enabled = FALSE,
            auto_approve_withdrawals_under = 0,
            referral_commission_percent = 0,
            bonus_per_mutirao_amount = 0,
            deposit_fee_percent = 0,
            withdraw_fee_fixed = 0,
            suitpay_enabled = FALSE,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = 1;
    END IF;
END $$;

-- Novos registros não herdam percentuais financeiros do protótipo.
ALTER TABLE users ALTER COLUMN custom_referral_commission_percent SET DEFAULT 0;
ALTER TABLE level_configs ALTER COLUMN commission_percent SET DEFAULT 0;
ALTER TABLE fleet_catalog ALTER COLUMN active SET DEFAULT FALSE;
ALTER TABLE announcements ALTER COLUMN active SET DEFAULT FALSE;
ALTER TABLE system_config ALTER COLUMN global_withdrawals_locked SET DEFAULT TRUE;
ALTER TABLE system_config ALTER COLUMN auto_approve_withdrawals_under SET DEFAULT 0;
ALTER TABLE system_config ALTER COLUMN referral_commission_percent SET DEFAULT 0;
ALTER TABLE system_config ALTER COLUMN bonus_per_mutirao_amount SET DEFAULT 0;
ALTER TABLE system_config ALTER COLUMN deposit_fee_percent SET DEFAULT 0;
ALTER TABLE system_config ALTER COLUMN withdraw_fee_fixed SET DEFAULT 0;
