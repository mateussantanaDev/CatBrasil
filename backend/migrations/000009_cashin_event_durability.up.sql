-- ============================================================================
-- CATBRASIL - MIGRATION 000009: CASH-IN EVENT DURABILITY & RECONCILIATION
-- Persistência P0.2: callbacks de depósito PIX (PAID_OUT/CHARGEBACK e status
-- autenticados não finais), registro durável de intenções enviadas, resultado
-- ambíguo/QR ausente e fila de reconciliação com deduplicação. Expiração local
-- NÃO comprova cancelamento no provedor; nenhum estado é alterado por tempo.
-- ============================================================================

-- 1. Registro durável de eventos verificados do gateway de depósito.
--    Uma linha por (external_id, status). Duplicatas são reconhecidas pela PK;
--    o callback CHARGEBACK é um evento distinto do PAID_OUT original. Status
--    não é restrito a enum para nunca descartar um callback autenticado sem
--    trilha; o comprimento é limitado. Amount aceita 0 para status não finais.
CREATE TABLE deposit_gateway_events (
    external_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (char_length(status) BETWEEN 1 AND 32),
    amount NUMERIC(15, 2) NOT NULL CHECK (amount >= 0),
    request_number VARCHAR(64),
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (external_id, status)
);

CREATE INDEX deposit_gateway_events_request_number_idx
    ON deposit_gateway_events (request_number)
    WHERE request_number IS NOT NULL;

-- 2. Registro durável da intenção enviada ao provedor: cobre intenção enviada
--    com resposta ambígua e QR code ausente, permitindo reconciliação posterior.
--    FK RESTRICT preserva a trilha financeira: não permite apagar a transação
--    enquanto a intenção existir.
CREATE TABLE cashin_deposit_intents (
    request_number VARCHAR(64) PRIMARY KEY REFERENCES transactions(id) ON DELETE RESTRICT,
    provider VARCHAR(32) NOT NULL,
    external_id VARCHAR(255),
    intent_sent_at TIMESTAMPTZ,
    qr_attached_at TIMESTAMPTZ,
    result_ambiguous BOOLEAN NOT NULL DEFAULT FALSE,
    ambiguity_reason TEXT,
    reconciled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- O external_id de depósito é único por provedor; impede reuso entre intenções.
CREATE UNIQUE INDEX cashin_deposit_intents_external_idx
    ON cashin_deposit_intents (provider, external_id)
    WHERE external_id IS NOT NULL;

CREATE INDEX cashin_deposit_intents_ambiguous_idx
    ON cashin_deposit_intents (created_at)
    WHERE result_ambiguous = TRUE;

-- 3. Fila persistente para eventos que não puderam ser aplicados: sem intenção,
--    externos/valores/requests conflitantes, estados incompatíveis, status não
--    finais, callbacks antecipados e chargeback sem saldo. Deduplicação por
--    chave estável; nunca silencia. resolução manual exige operador, evidência
--    objetiva e razão (colunas resolved_by/evidence_ref).
CREATE TABLE cashin_event_queue (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dedup_key TEXT NOT NULL UNIQUE,
    kind VARCHAR(48) NOT NULL,
    request_number VARCHAR(64),
    external_id VARCHAR(255),
    gateway_status VARCHAR(32),
    amount NUMERIC(15, 2) CHECK (amount IS NULL OR amount >= 0),
    reason TEXT NOT NULL,
    queue_status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (queue_status IN ('pending', 'resolved')),
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by VARCHAR(128),
    evidence_ref TEXT,
    resolution_note TEXT,
    CONSTRAINT cashin_event_queue_resolution_evidence CHECK (
        queue_status = 'pending'
        OR (resolved_by IS NOT NULL AND evidence_ref IS NOT NULL AND resolution_note IS NOT NULL)
    )
);

CREATE INDEX cashin_event_queue_pending_idx
    ON cashin_event_queue (created_at)
    WHERE queue_status = 'pending';
CREATE INDEX cashin_event_queue_request_number_idx
    ON cashin_event_queue (request_number)
    WHERE request_number IS NOT NULL;
CREATE INDEX cashin_event_queue_external_id_idx
    ON cashin_event_queue (external_id)
    WHERE external_id IS NOT NULL;
