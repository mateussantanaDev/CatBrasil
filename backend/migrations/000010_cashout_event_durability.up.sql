-- ============================================================================
-- CATBRASIL - MIGRATION 000010: CASH-OUT EVENT DURABILITY & RECONCILIATION
-- Persistência P0.3: trilha durável da submissão de saque (ID interno estável
-- enviado ao provedor associado ao idTransaction devolvido) e fila de
-- reconciliação para callbacks autenticados fora de ordem, com valor conflitante
-- ou sem vínculo local. Nenhum desses caminhos move saldo automaticamente.
-- ============================================================================

-- 1. Registro durável da intenção de transferência enviada ao provedor.
--    internal_ref é o ID interno estável (transactions.id) entregue ao gateway
--    como ExternalID; external_id é o idTransaction devolvido na resposta.
--    Uma linha sem external_id identifica, para triagem, uma submissão enviada
--    com retorno ambíguo/ausente — nunca inferida a partir do retorno.
CREATE TABLE payout_submissions (
    transaction_id VARCHAR(64) PRIMARY KEY REFERENCES transactions(id) ON DELETE RESTRICT,
    provider VARCHAR(32) NOT NULL,
    internal_ref VARCHAR(64) NOT NULL,
    external_id VARCHAR(255),
    sent_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    linked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- O retorno do provedor só pode pertencer a um único saque local.
CREATE UNIQUE INDEX payout_submissions_external_idx
    ON payout_submissions (external_id)
    WHERE external_id IS NOT NULL;

-- Submissões enviadas sem retorno associado, ordenadas para triagem.
CREATE INDEX payout_submissions_unlinked_idx
    ON payout_submissions (sent_at)
    WHERE external_id IS NULL;

-- 2. Fila persistente para eventos autenticados de saque que não puderam ser
--    aplicados: valor conflitante (nunca altera saldo), PAID_OUT após
--    cancelamento local (evidência para revisão manual; sem reenvio e sem
--    movimentação), status não suportado, retorno sem external_id e vínculos
--    divergentes. Deduplicação por chave estável; resolução manual exige
--    operador, evidência objetiva e razão (resolved_by/evidence_ref).
CREATE TABLE payout_event_queue (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dedup_key TEXT NOT NULL UNIQUE,
    kind VARCHAR(48) NOT NULL,
    transaction_id VARCHAR(64),
    external_id VARCHAR(255),
    gateway_status VARCHAR(32),
    amount NUMERIC(15, 2) CHECK (amount IS NULL OR amount > 0),
    reason TEXT NOT NULL,
    queue_status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (queue_status IN ('pending', 'resolved')),
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by VARCHAR(128),
    evidence_ref TEXT,
    resolution_note TEXT,
    CONSTRAINT payout_event_queue_resolution_evidence CHECK (
        queue_status = 'pending'
        OR (resolved_by IS NOT NULL AND evidence_ref IS NOT NULL AND resolution_note IS NOT NULL)
    )
);

CREATE INDEX payout_event_queue_pending_idx
    ON payout_event_queue (created_at)
    WHERE queue_status = 'pending';
CREATE INDEX payout_event_queue_transaction_idx
    ON payout_event_queue (transaction_id)
    WHERE transaction_id IS NOT NULL;
CREATE INDEX payout_event_queue_external_id_idx
    ON payout_event_queue (external_id)
    WHERE external_id IS NOT NULL;
