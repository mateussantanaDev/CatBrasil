#!/usr/bin/env python3
"""
scripts/release/reconcile.py

Ferramenta de auditoria PostgreSQL somente leitura para CatBrasil (P0.1).
Suporta alvos explícitos staging e production, com identidade de projeto, banco e volume verificada.

Garantia técnica de imutabilidade:
- Nível de isolamento REPEATABLE READ READ ONLY e SET TRANSACTION READ ONLY bloqueiam qualquer escrita nesta conexão.
- A contagem de linhas pré e pós-auditoria é uma verificação AUXILIAR de estabilidade volumétrica
  (não comprova ausência de UPDATE in-place ou alterações concorrentes compensadas).

Privacidade e Pseudonimização:
- Pseudonimização determinística por execução via HMAC-SHA256 (com sal efêmero) para itens sob investigação.
- Proibida exibição de IDs reais de usuários, transações ou provedores (sem prefixos como CB-, STG-, PIX-).
- Exclusão absoluta de PII e credenciais. Agregações e continuidades realizadas preferencialmente no SQL via LAG().
- Preservação estrita de precisão Decimal (parse_float=Decimal, serialização como string; jamais float).
"""

import argparse
import datetime
import decimal
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import subprocess
import sys


class AuditError(Exception):
    """Erro fatal durante a auditoria."""
    pass


class DecimalEncoder(json.JSONEncoder):
    """Serializador JSON que preserva Decimals como strings exatas sem perda por float."""
    def default(self, obj):
        if isinstance(obj, decimal.Decimal):
            return str(obj)
        return super().default(obj)


ROOT = Path(__file__).resolve().parents[2]
TARGETS = {
    "staging": ("catbrasil-staging", ".staging.env", "compose.staging.yml", "catbrasil_staging", "catbrasil-staging_pgdata"),
    "production": ("catbrasil-prod", ".prod.env", "compose.prod.yml", "catbrasil_prod", "catbrasil_prod_pgdata"),
}
PSQL_CMD = [
    "exec", "-T", "postgres", "sh", "-c",
    'exec psql -XAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
]

# Sal efêmero único gerado por execução para pseudonimização auditável sem vazamento de dados
EXECUTION_SALT = secrets.token_hex(16)


def pseudonymize(entity_type: str, raw_value: str) -> str:
    """Gera pseudônimo determinístico por execução via HMAC-SHA256 com sal efêmero.
    Permite correlacionar itens dentro do mesmo relatório sem expor IDs reais nem prefixos."""
    if not raw_value:
        return "anon_none"
    h = hmac.new(
        EXECUTION_SALT.encode("utf-8"),
        f"{entity_type}:{raw_value}".encode("utf-8"),
        hashlib.sha256
    ).hexdigest()[:12]
    return f"anon_{entity_type}_{h}"


def run_compose_cmd(args, stdin=None, target="staging"):
    """Executa comando via docker compose no contexto do alvo verificado."""
    try:
        project, env_file, override, _, _ = TARGETS[target]
        proc = subprocess.run(
            ["docker", "compose", "-p", project, "--env-file", env_file,
             "-f", "docker-compose.yml", "-f", override] + args,
            cwd=ROOT,
            input=stdin,
            text=True,
            capture_output=True,
            check=True,
            timeout=45
        )
        return proc.stdout.strip()
    except subprocess.CalledProcessError as exc:
        raise AuditError(f"Comando Compose falhou com código {exc.returncode}") from None
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise AuditError(f"Execução do comando Compose falhou ({type(exc).__name__})") from None


def verify_target_safety(target: str, confirm_staging: bool = False, confirm_production: bool = False) -> str:
    """Valida estritamente o alvo e impede qualquer acesso a produção ou contêineres não autorizados."""
    if target not in TARGETS:
        raise AuditError(
            f"Alvo '{target}' recusado. Use staging ou production."
        )
    if not (confirm_staging if target == "staging" else confirm_production):
        raise AuditError(
            "Confirmação explícita ausente para o alvo selecionado."
        )

    project_expected, env_name, _, _, volume_expected = TARGETS[target]
    env_path = ROOT / env_name
    if not env_path.is_file():
        raise AuditError(f"{env_name} ausente no repositório.")

    container_id = run_compose_cmd(["ps", "-q", "postgres"], target=target)
    if not container_id:
        raise AuditError("Contêiner PostgreSQL do alvo não encontrado ou não está em execução.")

    try:
        inspect_proc = subprocess.run(
            ["docker", "inspect", "-f", '{{ index .Config.Labels "com.docker.compose.project" }}|{{ index .Config.Labels "com.docker.compose.service" }}', container_id],
            capture_output=True,
            text=True,
            check=True,
            timeout=10
        )
        labels = inspect_proc.stdout.strip().split("|")
        project, service = labels[0], labels[1] if len(labels) > 1 else ""
    except Exception as exc:
        raise AuditError("Falha ao inspecionar rótulos Docker do contêiner PostgreSQL.") from None

    if project != project_expected or service != "postgres":
        raise AuditError(
            "Rótulos Docker do contêiner não correspondem ao alvo selecionado."
        )

    try:
        mounts_proc = subprocess.run(
            ["docker", "inspect", "-f", "{{json .Mounts}}", container_id],
            capture_output=True, text=True, check=True, timeout=10,
        )
        mounts = json.loads(mounts_proc.stdout)
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired, ValueError):
        raise AuditError("Não foi possível verificar o volume PostgreSQL do alvo.") from None
    if not isinstance(mounts, list) or not any(isinstance(m, dict)
               and m.get("Type") == "volume" and m.get("Name") == volume_expected
               and m.get("Destination") == "/var/lib/postgresql/data" for m in mounts):
        raise AuditError("Volume PostgreSQL não corresponde ao alvo selecionado.")

    return container_id


def verify_database_name(target="staging"):
    """Valida o nome do banco de dados imediatamente antes de qualquer inventário ou contagem."""
    sql = "BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY; SELECT current_database(); ROLLBACK;"
    raw = run_compose_cmd(PSQL_CMD, stdin=sql, target=target)
    db_name = None
    for line in raw.splitlines():
        line = line.strip()
        if line and line not in ("BEGIN", "ROLLBACK", "SET"):
            db_name = line
            break
    expected = TARGETS[target][3]
    if db_name != expected:
        raise AuditError(
            f"Banco de dados '{db_name}' não corresponde ao ambiente esperado '{expected}'. Auditoria abortada antes de qualquer inventário."
        )
    return db_name


def query_table_row_counts(target="staging"):
    """Retorna contagem de linhas de todas as tabelas em formato {tabela: contagem}."""
    sql = """
    SELECT json_object_agg(table_name, cnt) FROM (
      SELECT table_name, (xpath('/row/cnt/text()', xml_count))[1]::text::bigint as cnt
      FROM (
        SELECT table_name,
               query_to_xml(format('SELECT count(*) as cnt FROM %I', table_name), false, true, '') as xml_count
        FROM information_schema.tables
        WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
      ) t
    ) sub;
    """
    raw = run_compose_cmd(PSQL_CMD, stdin=sql, target=target)
    try:
        for line in raw.splitlines():
            line = line.strip()
            if line.startswith("{") and line.endswith("}"):
                return json.loads(line)
        return json.loads(raw)
    except Exception as exc:
        raise AuditError("Falha ao obter contagem de linhas das tabelas: formato inválido") from None


def check_table_row_count_stability(counts_before, counts_after):
    """
    Verificação AUXILIAR de estabilidade volumétrica das tabelas.
    AVISO TÉCNICO: Não constitui prova de ausência de mutação; apenas checa deltas líquidos de linhas.
    A proteção contra escrita é provida exclusivamente pela transação READ ONLY do PostgreSQL.
    """
    deltas = {}
    all_tables = set(counts_before.keys()) | set(counts_after.keys())
    for t in sorted(all_tables):
        b = counts_before.get(t, 0)
        a = counts_after.get(t, 0)
        if b != a:
            deltas[t] = {"before": b, "after": a, "diff": a - b}

    if deltas:
        raise AuditError(
            f"Variação volumétrica detectada nas tabelas durante a auditoria: {deltas}"
        )
    return True


def parse_audit_sections(raw_output: str) -> dict:
    """Faz o parse seguro das seções JSON delimitadas emitidas pelo script SQL."""
    sections = {}
    lines = raw_output.splitlines()
    current_section = None
    section_buffer = []

    for line in lines:
        line_clean = line.strip()
        if line_clean.startswith("SECTION_") and line_clean.endswith("_BEGIN"):
            current_section = line_clean[8:-6]
            section_buffer = []
        elif line_clean.startswith("SECTION_") and line_clean.endswith("_END"):
            if current_section:
                full_json_str = "".join(section_buffer)
                try:
                    sections[current_section] = json.loads(full_json_str, parse_float=decimal.Decimal)
                except Exception as exc:
                    # NUNCA incluir full_json_str, buffers ou PII na mensagem de erro!
                    raise AuditError(
                        f"Falha de desserialização JSON na seção '{current_section}': erro de sintaxe ({type(exc).__name__})"
                    ) from None
            current_section = None
            section_buffer = []
        elif current_section:
            section_buffer.append(line)

    expected_sections = [
        "SCHEMA", "USERS", "TRANSACTIONS", "FINANCIAL_EVENTS",
        "BALANCE_VS_EVENTS", "PENDING_OPS", "NO_EXTERNAL_ID", "TX_SUM_ANALYSIS"
    ]
    missing = [sec for sec in expected_sections if sec not in sections]
    if missing:
        raise AuditError(f"Seções obrigatórias ausentes na saída SQL: {missing}")

    return sections


def execute_read_only_audit(target="staging"):
    """
    Executa a coleta de inventário e dados financeiros inteiramente dentro de uma
    única transação REPEATABLE READ READ ONLY. Agrega dados no SQL via window functions (LAG)
    para evitar despejo de listas brutas e escalar de forma eficiente.
    """
    sql_script = """
    BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
    SET TRANSACTION READ ONLY;

    -- 1. Schema, migrations e índices
    SELECT 'SECTION_SCHEMA_BEGIN';
    SELECT json_build_object(
      'current_db', current_database(),
      'tx_read_only', current_setting('transaction_read_only'),
      'migrations', (
        SELECT COALESCE(json_agg(json_build_object(
          'version', version,
          'name', name,
          'checksum', checksum,
          'applied_at', applied_at::text
        ) ORDER BY version), '[]'::json)
        FROM schema_migrations
      ),
      'tables', (
        SELECT COALESCE(json_agg(json_build_object(
          'table_name', table_name,
          'column_count', cnt
        ) ORDER BY table_name), '[]'::json)
        FROM (
          SELECT table_name, count(column_name) as cnt
          FROM information_schema.columns
          WHERE table_schema = 'public'
          GROUP BY table_name
        ) t
      ),
      'financial_indexes', (
        SELECT COALESCE(json_agg(json_build_object(
          'table', tablename,
          'index', indexname,
          'def', indexdef
        ) ORDER BY tablename, indexname), '[]'::json)
        FROM pg_indexes
        WHERE schemaname = 'public' AND (
          indexname LIKE '%idempotency%' OR
          indexname LIKE '%external_id%' OR
          indexname LIKE '%financial_events%' OR
          indexname LIKE '%payout%'
        )
      )
    );
    SELECT 'SECTION_SCHEMA_END';

    -- 2. Inventário de usuários e saldos agregados (sem listar dados pessoais)
    SELECT 'SECTION_USERS_BEGIN';
    SELECT json_build_object(
      'total_users', count(*),
      'roles', (SELECT json_object_agg(role, cnt) FROM (SELECT role, count(*) as cnt FROM users GROUP BY role) r),
      'aggregates', (
        SELECT json_build_object(
          'sum_balance', COALESCE(sum(balance), 0.00)::text,
          'sum_yield_balance', COALESCE(sum(yield_balance_available), 0.00)::text,
          'min_balance', COALESCE(min(balance), 0.00)::text,
          'max_balance', COALESCE(max(balance), 0.00)::text,
          'users_with_balance_gt_zero', count(*) FILTER (WHERE balance > 0),
          'users_with_balance_zero', count(*) FILTER (WHERE balance = 0)
        ) FROM users
      ),
      'flags', (
        SELECT json_build_object(
          'withdraw_locked_count', count(*) FILTER (WHERE withdraw_locked),
          'withdraw_pin_locked_count', count(*) FILTER (WHERE withdraw_pin_locked_until > now()),
          'multi_account_count', count(*) FILTER (WHERE multi_account_flag),
          'referral_bonus_suspended_count', count(*) FILTER (WHERE referral_bonus_suspended),
          'level12_bonus_suspended_count', count(*) FILTER (WHERE level12_bonus_suspended)
        ) FROM users
      ),
      'demo_seed_suspicions_count', (
        SELECT count(*) FROM users
        WHERE username ILIKE '%demo%' OR username ILIKE '%seed%' OR username IN ('gestor_master', 'admin', 'catadmin', 'teste')
      )
    ) FROM users;
    SELECT 'SECTION_USERS_END';

    -- 3. Transações agregadas por tipo, status e provedor
    SELECT 'SECTION_TRANSACTIONS_BEGIN';
    SELECT json_build_object(
      'total_transactions', (SELECT count(*) FROM transactions),
      'date_range', (
        SELECT json_build_object(
          'oldest', min(created_at)::text,
          'newest', max(created_at)::text
        ) FROM transactions
      ),
      'by_group', (
        SELECT COALESCE(json_agg(json_build_object(
          'tipo', tipo,
          'status', status,
          'provider', COALESCE(gateway_provider, 'none'),
          'count', cnt,
          'sum_valor', sum_val::text,
          'min_created_at', min_d::text,
          'max_created_at', max_d::text
        )), '[]'::json)
        FROM (
          SELECT tipo, status, gateway_provider, count(*) as cnt, sum(valor) as sum_val, min(created_at) as min_d, max(created_at) as max_d
          FROM transactions
          GROUP BY tipo, status, gateway_provider
        ) t
      ),
      'by_status', (
        SELECT COALESCE(json_object_agg(status, cnt), '{}'::json)
        FROM (SELECT status, count(*) as cnt FROM transactions GROUP BY status) s
      ),
      'by_provider', (
        SELECT COALESCE(json_object_agg(COALESCE(gateway_provider, 'none'), cnt), '{}'::json)
        FROM (SELECT gateway_provider, count(*) as cnt FROM transactions GROUP BY gateway_provider) p
      )
    );
    SELECT 'SECTION_TRANSACTIONS_END';

    -- 4. Cobertura e continuidade de financial_events (agregação escalável via LAG particionado)
    SELECT 'SECTION_FINANCIAL_EVENTS_BEGIN';
    WITH event_checks AS (
      SELECT
        id,
        user_id,
        amount,
        balance_before,
        balance_after,
        LAG(balance_after) OVER (PARTITION BY user_id ORDER BY id) AS prev_balance_after
      FROM financial_events
    ),
    anomalies AS (
      SELECT
        id,
        user_id,
        CASE
          WHEN balance_after <> balance_before + amount THEN 'EQUATION_MISMATCH'
          WHEN prev_balance_after IS NOT NULL AND balance_before <> prev_balance_after THEN 'CHAIN_DISCONTINUITY'
        END AS anomaly_kind,
        balance_before,
        balance_after,
        amount,
        prev_balance_after
      FROM event_checks
      WHERE (balance_after <> balance_before + amount)
         OR (prev_balance_after IS NOT NULL AND balance_before <> prev_balance_after)
    )
    SELECT json_build_object(
      'total_financial_events', (SELECT count(*) FROM financial_events),
      'migration_5_applied_at', (SELECT applied_at::text FROM schema_migrations WHERE version = 5),
      'by_kind', (
        SELECT COALESCE(json_agg(json_build_object('kind', kind, 'count', cnt, 'sum_amount', sum_amt::text)), '[]'::json)
        FROM (SELECT kind, count(*) as cnt, sum(amount) as sum_amt FROM financial_events GROUP BY kind) k
      ),
      'tx_before_v5_count', (
        SELECT count(*) FROM transactions t
        WHERE t.created_at < (SELECT COALESCE(min(applied_at), 'epoch'::timestamptz) FROM schema_migrations WHERE version = 5)
      ),
      'tx_after_v5_count', (
        SELECT count(*) FROM transactions t
        WHERE t.created_at >= (SELECT COALESCE(min(applied_at), 'epoch'::timestamptz) FROM schema_migrations WHERE version = 5)
      ),
      'tx_linked_to_events_count', (
        SELECT count(DISTINCT transaction_id) FROM financial_events
      ),
      'anomalies_count', (SELECT count(*) FROM anomalies),
      'anomalies', (
        SELECT COALESCE(json_agg(json_build_object(
          'event_id', id,
          'user_id', user_id,
          'kind', anomaly_kind,
          'balance_before', balance_before::text,
          'balance_after', balance_after::text,
          'amount', amount::text,
          'prev_balance_after', prev_balance_after::text
        )), '[]'::json)
        FROM (
          SELECT id, user_id, anomaly_kind, balance_before, balance_after, amount, prev_balance_after
          FROM anomalies
          ORDER BY id
          LIMIT 100
        ) sub_anomalies
      )
    );
    SELECT 'SECTION_FINANCIAL_EVENTS_END';

    -- 5. Último evento versus users.balance (emite apenas discrepâncias)
    SELECT 'SECTION_BALANCE_VS_EVENTS_BEGIN';
    SELECT json_build_object(
      'comparisons_summary', (
        SELECT json_build_object(
          'balanced_count', count(*) FILTER (WHERE latest.balance_after = u.balance),
          'no_events_zero_balance_count', count(*) FILTER (WHERE latest.id IS NULL AND u.balance = 0.00),
          'mismatched_count', count(*) FILTER (WHERE latest.id IS NOT NULL AND latest.balance_after <> u.balance),
          'no_events_nonzero_balance_count', count(*) FILTER (WHERE latest.id IS NULL AND u.balance <> 0.00)
        )
        FROM users u
        LEFT JOIN LATERAL (
          SELECT id, balance_after
          FROM financial_events
          WHERE user_id = u.id
          ORDER BY id DESC
          LIMIT 1
        ) latest ON TRUE
      ),
      'discrepancies', (
        SELECT COALESCE(json_agg(json_build_object(
          'user_id', u.id,
          'user_balance', u.balance::text,
          'latest_event_balance_after', latest.balance_after::text,
          'diff', (u.balance - COALESCE(latest.balance_after, 0.00))::text,
          'status', CASE
            WHEN latest.id IS NULL AND u.balance <> 0.00 THEN 'no_events_nonzero_balance'
            ELSE 'mismatch'
          END
        )), '[]'::json)
        FROM users u
        LEFT JOIN LATERAL (
          SELECT id, balance_after
          FROM financial_events
          WHERE user_id = u.id
          ORDER BY id DESC
          LIMIT 1
        ) latest ON TRUE
        WHERE (latest.id IS NOT NULL AND latest.balance_after <> u.balance)
           OR (latest.id IS NULL AND u.balance <> 0.00)
      )
    );
    SELECT 'SECTION_BALANCE_VS_EVENTS_END';

    -- 6. Pendências de depósito e saque
    SELECT 'SECTION_PENDING_OPS_BEGIN';
    SELECT json_build_object(
      'pending_deposits', (
        SELECT COALESCE(json_agg(json_build_object(
          'id', id,
          'user_id', user_id,
          'valor', valor::text,
          'gateway_provider', gateway_provider,
          'has_external_id', (external_id IS NOT NULL AND external_id <> ''),
          'external_id', COALESCE(external_id, ''),
          'created_at', created_at::text,
          'age_minutes', round(extract(epoch from (now() - created_at))/60, 1)::text
        ) ORDER BY created_at), '[]'::json)
        FROM transactions
        WHERE tipo = 'Depósito PIX' AND status = 'pendente'
      ),
      'pending_or_processing_withdrawals', (
        SELECT COALESCE(json_agg(json_build_object(
          'id', id,
          'user_id', user_id,
          'valor', valor::text,
          'net_amount', net_amount::text,
          'fee_amount', fee_amount::text,
          'status', status,
          'gateway_provider', gateway_provider,
          'has_external_id', (external_id IS NOT NULL AND external_id <> ''),
          'external_id', COALESCE(external_id, ''),
          'payout_attempted_at', payout_attempted_at::text,
          'created_at', created_at::text,
          'age_minutes', round(extract(epoch from (now() - created_at))/60, 1)::text
        ) ORDER BY created_at), '[]'::json)
        FROM transactions
        WHERE tipo = 'Saque PIX' AND status IN ('pendente', 'processando')
      ),
      'unmatched_payout_gateway_events', (
        SELECT COALESCE(json_agg(json_build_object(
          'external_id', external_id,
          'status', status,
          'amount', amount::text,
          'received_at', received_at::text
        ) ORDER BY received_at), '[]'::json)
        FROM payout_gateway_events e
        WHERE NOT EXISTS (SELECT 1 FROM transactions t WHERE t.external_id = e.external_id)
      )
    );
    SELECT 'SECTION_PENDING_OPS_END';

    -- 7. Casos sem external_id
    SELECT 'SECTION_NO_EXTERNAL_ID_BEGIN';
    SELECT json_build_object(
      'cases', (
        SELECT COALESCE(json_agg(json_build_object(
          'tipo', tipo,
          'status', status,
          'provider', COALESCE(gateway_provider, 'none'),
          'count', cnt,
          'sum_valor', sum_val::text
        )), '[]'::json)
        FROM (
          SELECT tipo, status, gateway_provider, count(*) as cnt, sum(valor) as sum_val
          FROM transactions
          WHERE external_id IS NULL OR external_id = ''
          GROUP BY tipo, status, gateway_provider
        ) s
      )
    );
    SELECT 'SECTION_NO_EXTERNAL_ID_END';

    -- 8. Demonstração e qualificação de soma de transactions vs users.balance
    SELECT 'SECTION_TX_SUM_ANALYSIS_BEGIN';
    SELECT json_build_object(
      'sum_all_users_balance', (SELECT COALESCE(sum(balance), 0.00)::text FROM users),
      'sum_deposits_sucesso', (SELECT COALESCE(sum(valor), 0.00)::text FROM transactions WHERE tipo = 'Depósito PIX' AND status = 'sucesso'),
      'sum_saques_sucesso', (SELECT COALESCE(sum(valor), 0.00)::text FROM transactions WHERE tipo = 'Saque PIX' AND status = 'sucesso'),
      'transactions_by_type_and_status', (
        SELECT COALESCE(json_agg(json_build_object(
          'tipo', tipo,
          'status', status,
          'count', cnt,
          'sum_valor', sum_val::text
        )), '[]'::json)
        FROM (
          SELECT tipo, status, count(*) as cnt, sum(valor) as sum_val
          FROM transactions
          GROUP BY tipo, status
        ) t
      )
    );
    SELECT 'SECTION_TX_SUM_ANALYSIS_END';

    ROLLBACK;
    """

    raw_output = run_compose_cmd(PSQL_CMD, stdin=sql_script, target=target)
    return parse_audit_sections(raw_output)


def process_financial_events_continuity(fe_section: dict) -> dict:
    """
    Processa o resultado da checagem contábil realizada via window function LAG() no PostgreSQL.
    Se total_events == 0, declara formalmente UNTESTED_NO_EVENTS (não testada).
    """
    total_events = fe_section.get("total_financial_events", 0)
    if total_events == 0:
        return {
            "status": "UNTESTED_NO_EVENTS",
            "description": "NÃO TESTADA (zero eventos financeiros no banco auditado)",
            "total_events_checked": 0,
            "is_continuous": None,
            "anomalies_count": 0,
            "anomalies": []
        }

    raw_anomalies = fe_section.get("anomalies", [])
    anomalies_count = fe_section.get("anomalies_count", 0)

    pseudonymized_anomalies = []
    for a in raw_anomalies:
        pseudonymized_anomalies.append({
            "event_pseudonym": pseudonymize("event", str(a.get("event_id", ""))),
            "user_pseudonym": pseudonymize("user", str(a.get("user_id", ""))),
            "kind": a.get("kind", ""),
            "balance_before": str(a.get("balance_before", "")),
            "balance_after": str(a.get("balance_after", "")),
            "amount": str(a.get("amount", "")),
            "prev_balance_after": str(a.get("prev_balance_after", ""))
        })

    is_continuous = (anomalies_count == 0)
    return {
        "status": "TESTED_CONTINUOUS" if is_continuous else "CHAIN_INTEGRITY_BREACH",
        "description": "Encadeamento íntegro" if is_continuous else f"Detectadas {anomalies_count} anomalias no encadeamento",
        "total_events_checked": total_events,
        "is_continuous": is_continuous,
        "anomalies_count": anomalies_count,
        "anomalies": pseudonymized_anomalies
    }


def analyze_balance_comparisons(bal_section: dict) -> dict:
    """Analisa a comparação entre users.balance e o último evento financeiro a partir de agregações SQL."""
    summary = bal_section.get("comparisons_summary", {})
    raw_discrepancies = bal_section.get("discrepancies", [])

    pseudonymized_discrepancies = []
    for d in raw_discrepancies:
        pseudonymized_discrepancies.append({
            "user_pseudonym": pseudonymize("user", d.get("user_id", "")),
            "user_balance": str(d.get("user_balance", "0.00")),
            "latest_event_balance_after": str(d.get("latest_event_balance_after", "none")),
            "diff": str(d.get("diff", "0.00")),
            "status": d.get("status", "unknown")
        })

    return {
        "balanced_count": summary.get("balanced_count", 0),
        "no_events_zero_balance_count": summary.get("no_events_zero_balance_count", 0),
        "mismatched_count": summary.get("mismatched_count", 0),
        "no_events_nonzero_balance_count": summary.get("no_events_nonzero_balance_count", 0),
        "discrepancies_count": len(pseudonymized_discrepancies),
        "discrepancies": pseudonymized_discrepancies
    }


def build_audit_report(container_id: str, counts_before: dict, counts_after: dict, sections: dict, target="staging") -> dict:
    """Monta a estrutura consolidada, sanitizada e pseudonimizada do relatório."""
    schema = sections.get("SCHEMA", {})
    users_sec = sections.get("USERS", {})
    tx_sec = sections.get("TRANSACTIONS", {})
    fe_sec = sections.get("FINANCIAL_EVENTS", {})
    bal_sec = sections.get("BALANCE_VS_EVENTS", {})
    pending_sec = sections.get("PENDING_OPS", {})
    no_ext_sec = sections.get("NO_EXTERNAL_ID", {})
    sum_analysis_sec = sections.get("TX_SUM_ANALYSIS", {})

    current_db = schema.get("current_db")
    if current_db != TARGETS[target][3]:
        raise AuditError(
            "Banco de dados inesperado para o alvo selecionado."
        )

    continuity_result = process_financial_events_continuity(fe_sec)
    balance_analysis = analyze_balance_comparisons(bal_sec)

    pending_deposits_pseudonymized = []
    for d in pending_sec.get("pending_deposits", []):
        pending_deposits_pseudonymized.append({
            "transaction_pseudonym": pseudonymize("tx", d.get("id", "")),
            "user_pseudonym": pseudonymize("user", d.get("user_id", "")),
            "valor": str(d.get("valor", "0.00")),
            "gateway_provider": d.get("gateway_provider", ""),
            "has_external_id": d.get("has_external_id", False),
            "external_id_pseudonym": pseudonymize("ext", d.get("external_id", "")) if d.get("has_external_id") else "none",
            "created_at": d.get("created_at", ""),
            "age_minutes": str(d.get("age_minutes", ""))
        })

    pending_withdrawals_pseudonymized = []
    for w in pending_sec.get("pending_or_processing_withdrawals", []):
        pending_withdrawals_pseudonymized.append({
            "transaction_pseudonym": pseudonymize("tx", w.get("id", "")),
            "user_pseudonym": pseudonymize("user", w.get("user_id", "")),
            "valor": str(w.get("valor", "0.00")),
            "net_amount": str(w.get("net_amount", "0.00")),
            "fee_amount": str(w.get("fee_amount", "0.00")),
            "status": w.get("status", ""),
            "gateway_provider": w.get("gateway_provider", ""),
            "has_external_id": w.get("has_external_id", False),
            "external_id_pseudonym": pseudonymize("ext", w.get("external_id", "")) if w.get("has_external_id") else "none",
            "payout_attempted_at": w.get("payout_attempted_at", ""),
            "created_at": w.get("created_at", ""),
            "age_minutes": str(w.get("age_minutes", ""))
        })

    unmatched_callbacks_pseudonymized = []
    for u in pending_sec.get("unmatched_payout_gateway_events", []):
        unmatched_callbacks_pseudonymized.append({
            "external_id_pseudonym": pseudonymize("ext", u.get("external_id", "")),
            "status": u.get("status", ""),
            "amount": str(u.get("amount", "0.00")),
            "received_at": u.get("received_at", "")
        })

    now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()

    report = {
        "audit_metadata": {
            "title": f"P0.1 — Inventário e Conciliação Financeira Interna ({current_db})",
            "timestamp": now_iso,
            "target": target,
            "database_name": current_db,
            "transaction_isolation": "REPEATABLE READ READ ONLY (ROLLBACK)",
            "safety_model": {
                "technical_guarantee": "Transação PostgreSQL REPEATABLE READ READ ONLY (SET TRANSACTION READ ONLY) bloqueia escritas nesta sessão.",
                "auxiliary_row_count_check": {
                    "description": "Verificação auxiliar de estabilidade volumétrica de linhas (não prova ausência de UPDATE in-place ou alterações compensadas).",
                    "tables_checked_count": len(counts_before),
                    "counts_before": counts_before,
                    "counts_after": counts_after,
                    "deltas": {}
                }
            }
        },
        "schema_inventory": {
            "migrations_applied_count": len(schema.get("migrations", [])),
            "migrations": schema.get("migrations", []),
            "tables_count": len(schema.get("tables", [])),
            "tables": schema.get("tables", []),
            "financial_indexes": schema.get("financial_indexes", [])
        },
        "users_inventory": {
            "total_users": users_sec.get("total_users", 0),
            "roles": users_sec.get("roles", {}),
            "aggregates": users_sec.get("aggregates", {}),
            "security_flags": users_sec.get("flags", {}),
            "demo_seed_suspicions_count": users_sec.get("demo_seed_suspicions_count", 0)
        },
        "transactions_inventory": {
            "total_transactions": tx_sec.get("total_transactions", 0),
            "date_range": tx_sec.get("date_range", {}),
            "by_group": tx_sec.get("by_group", []),
            "by_status": tx_sec.get("by_status", {}),
            "by_provider": tx_sec.get("by_provider", {})
        },
        "financial_events_coverage_and_continuity": {
            "total_events": fe_sec.get("total_events", 0),
            "migration_5_applied_at": fe_sec.get("migration_5_applied_at"),
            "tx_before_v5_count": fe_sec.get("tx_before_v5_count", 0),
            "tx_after_v5_count": fe_sec.get("tx_after_v5_count", 0),
            "tx_linked_to_events_count": fe_sec.get("tx_linked_to_events_count", 0),
            "by_kind": fe_sec.get("by_kind", []),
            "continuity_verification": continuity_result
        },
        "balance_vs_latest_event": balance_analysis,
        "pending_operations": {
            "pending_deposits_count": len(pending_deposits_pseudonymized),
            "pending_deposits": pending_deposits_pseudonymized,
            "pending_or_processing_withdrawals_count": len(pending_withdrawals_pseudonymized),
            "pending_or_processing_withdrawals": pending_withdrawals_pseudonymized,
            "unmatched_payout_callbacks_count": len(unmatched_callbacks_pseudonymized),
            "unmatched_payout_callbacks": unmatched_callbacks_pseudonymized
        },
        "cases_without_external_id": {
            "total_groups": len(no_ext_sec.get("cases", [])),
            "cases": no_ext_sec.get("cases", [])
        },
        "pre_v5_history_and_sum_fallacy_qualification": {
            "explanation": (
                "No modelo legado CatBrasil (anterior à migração 5), não existia o diário financeiro "
                "(financial_events) com invariante balance_after = balance_before + amount. "
                "Operações internas (como coletas de máquinas, giros de roleta, aluguéis e comissões) podiam "
                "atualizar diretamente users.balance sem gerar linhas em transactions, ou geravam transações "
                "sem convenção contábil uniforme. Além disso, sementes iniciais injetavam saldo diretamente "
                "no cadastro do usuário. Portanto, a soma ingênua de transações (SUM(transactions.valor)) é "
                "uma falácia contábil no histórico legado e não pode ser usada para arbitrar ou validar saldos."
            ),
            "data_comparison": sum_analysis_sec
        },
        "evidence_classification": {
            "audited_facts": [
                f"Banco ativo verificado: {current_db}.",
                f"Migrações registradas em schema_migrations: {len(schema.get('migrations', []))}.",
                f"Total de usuários cadastrados: {users_sec.get('total_users', 0)}.",
                f"Saldo operacional total em users.balance: R$ {users_sec.get('aggregates', {}).get('sum_balance', '0.00')}.",
                f"Total de transações: {tx_sec.get('total_transactions', 0)}.",
                f"Total de eventos em financial_events: {fe_sec.get('total_events', 0)}.",
                f"Todas as {len(counts_before)} tabelas auditadas apresentaram estabilidade volumétrica de linhas pré e pós-auditoria."
            ],
            "suspicions_and_hypotheses": [
                f"Existem {len(pending_deposits_pseudonymized)} depósitos pendentes que requerem validação operacional.",
                f"{users_sec.get('demo_seed_suspicions_count', 0)} contas coincidem com padrões conhecidos de nomes de teste/demo."
            ],
            "bank_statement_dependencies": [
                "Determinação de se houve liquidação de valor real para qualquer depósito pendente.",
                "Conciliação de abertura e aferição de passivo financeiro real para decisão de corte em produção.",
                "Aferição do saldo bancário de custódia versus saldo em carteira."
            ]
        },
        "verdict": {
            "status": "INTERNAL_AUDIT_REQUIRES_EXTERNAL_RECONCILIATION",
            "explanation": (
                "Esta auditoria verifica somente a consistência interna do banco. "
                "Sem eventos financeiros a continuidade permanece não testada. "
                "Mesmo com cadeia íntegra, a custódia requer conciliação com extrato oficial do provedor."
            ),
            "p0_status": "P0_NOT_CONCLUDED"
        }
    }

    return report


def render_markdown_report(report: dict) -> str:
    """Renderiza relatório legível em Markdown sem expor PII ou segredos."""
    meta = report["audit_metadata"]
    schema = report["schema_inventory"]
    users = report["users_inventory"]
    tx = report["transactions_inventory"]
    fe = report["financial_events_coverage_and_continuity"]
    bal = report["balance_vs_latest_event"]
    pending = report["pending_operations"]
    no_ext = report["cases_without_external_id"]
    qual = report["pre_v5_history_and_sum_fallacy_qualification"]
    evid = report["evidence_classification"]
    verdict = report["verdict"]

    md = []
    md.append(f"# {meta['title']}\n")
    md.append(f"- **Data / Hora (UTC):** `{meta['timestamp']}`")
    md.append(f"- **Ambiente auditado:** `{meta['target']}` (`{meta['database_name']}`)")
    md.append(f"- **Isolamento de transação:** `{meta['transaction_isolation']}`")
    md.append(f"- **Garantia Técnica:** {meta['safety_model']['technical_guarantee']}")
    md.append(f"- **Verificação Auxiliar de Linhas:** {meta['safety_model']['auxiliary_row_count_check']['description']} ({meta['safety_model']['auxiliary_row_count_check']['tables_checked_count']} tabelas conferidas com delta zero).\n")

    md.append("## 1. Resumo Executivo e Veredito\n")
    md.append("| Métrica | Valor Auditado |")
    md.append("| --- | --- |")
    md.append(f"| **Versão de Migrações Registradas** | `{schema['migrations_applied_count']}` |")
    md.append(f"| **Total de Usuários** | `{users['total_users']}` |")
    md.append(f"| **Saldo Operacional Total (`users.balance`)** | `R$ {users['aggregates'].get('sum_balance', '0.00')}` |")
    md.append(f"| **Total de Transações (`transactions`)** | `{tx['total_transactions']}` |")
    md.append(f"| **Total de Eventos no Diário (`financial_events`)** | `{fe['total_events']}` |")
    md.append(f"| **Status de Continuidade do Diário** | `{fe['continuity_verification']['status']}` ({fe['continuity_verification']['description']}) |")
    md.append(f"| **Usuários com Saldo Exato vs Último Evento** | `{bal['balanced_count']}` |")
    md.append(f"| **Usuários sem Eventos com Saldo R$ 0,00** | `{bal['no_events_zero_balance_count']}` |")
    md.append(f"| **Divergências de Saldo Detectadas** | `{bal['discrepancies_count']}` |")
    md.append(f"| **Depósitos Pendentes (Pseudonimizados)** | `{pending['pending_deposits_count']}` |")
    md.append(f"| **Saques Pendentes / Em Processamento** | `{pending['pending_or_processing_withdrawals_count']}` |\n")

    md.append(f"> **Status do Veredito:** `{verdict['status']}`\n")
    md.append(f"> **Explicação:** {verdict['explanation']}\n")
    md.append(f"> **Status P0:** `{verdict['p0_status']}` (a auditoria interna não substitui o extrato oficial)\n")

    md.append("## 2. Separação de Evidências: Fatos, Suspeitas e Dependências de Extrato\n")
    md.append("### 2.1. Fatos Auditados no Staging")
    for f in evid["audited_facts"]:
        md.append(f"- {f}")
    md.append("")

    md.append("### 2.2. Suspeitas e Hipóteses Operacionais")
    for s in evid["suspicions_and_hypotheses"]:
        md.append(f"- {s}")
    md.append("")

    md.append("### 2.3. Itens Dependentes de Extrato Bancário / Provedor Real")
    for d in evid["bank_statement_dependencies"]:
        md.append(f"- {d}")
    md.append("")

    md.append("## 3. Inventário de Schema e Restrições Críticas\n")
    md.append(f"Identificadas `{schema['tables_count']}` tabelas base no schema `public`.")
    md.append("### Migrações:")
    md.append("| Versão | Nome | Aplicado em (UTC) |")
    md.append("| :---: | :--- | :--- |")
    for m in schema["migrations"]:
        md.append(f"| `{m['version']}` | `{m['name']}` | `{m['applied_at']}` |")
    md.append("")

    md.append("### Índices Financeiros Relevantes:")
    for idx in schema.get("financial_indexes", []):
        md.append(f"- **`{idx['table']}`** (`{idx['index']}`): `{idx['def']}`")
    md.append("")

    md.append("## 4. Agregações de Usuários e Carteiras (Sem PII)\n")
    md.append(f"- **Distribuição por Role:** `{json.dumps(users['roles'])}`")
    md.append(f"- **Saldo Operacional:** `R$ {users['aggregates'].get('sum_balance', '0.00')}`")
    md.append(f"- **Saldo de Rendimentos:** `R$ {users['aggregates'].get('sum_yield_balance', '0.00')}`")
    md.append(f"- **Usuários com Saldo > 0:** `{users['aggregates'].get('users_with_balance_gt_zero', 0)}`")
    md.append(f"- **Usuários com Saldo = 0:** `{users['aggregates'].get('users_with_balance_zero', 0)}`")
    md.append(f"- **Travas de Segurança:** `{json.dumps(users['security_flags'])}`")
    md.append(f"- **Suspeitas de Nomes Demo/Seed:** `{users['demo_seed_suspicions_count']}`")
    md.append("")

    md.append("## 5. Transações por Tipo, Status e Provedor\n")
    if tx["by_group"]:
        md.append("| Tipo | Status | Gateway | Quantidade | Total (R$) | Data Inicial | Data Final |")
        md.append("| :--- | :--- | :--- | :---: | :---: | :--- | :--- |")
        for g in tx["by_group"]:
            md.append(f"| {g['tipo']} | `{g['status']}` | `{g['provider']}` | {g['count']} | R$ {g['sum_valor']} | {g['min_created_at'][:19]} | {g['max_created_at'][:19]} |")
    else:
        md.append("Nenhuma transação registrada no banco.")
    md.append("")

    md.append("## 6. Cobertura e Continuidade do Diário de Saldo (`financial_events`)\n")
    md.append(f"- **Data da Migração 5:** `{fe['migration_5_applied_at']}`")
    md.append(f"- **Transações pré-migração 5:** `{fe['tx_before_v5_count']}`")
    md.append(f"- **Transações pós-migração 5:** `{fe['tx_after_v5_count']}`")
    md.append(f"- **Transações vinculadas ao diário:** `{fe['tx_linked_to_events_count']}`")
    md.append(f"- **Status Contábil:** `{fe['continuity_verification']['status']}` — {fe['continuity_verification']['description']}")
    md.append(f"- **Anomalias de encadeamento:** `{fe['continuity_verification']['anomalies_count']}`\n")

    md.append("## 7. Comparação de Saldo Atual vs Último Evento no Diário\n")
    md.append(f"- **Usuários com saldo em conformidade com o último evento:** `{bal['balanced_count']}`")
    md.append(f"- **Usuários novos zerados sem registros no diário:** `{bal['no_events_zero_balance_count']}`")
    md.append(f"- **Usuários com divergência (`users.balance != latest.balance_after`):** `{bal['mismatched_count']}`")
    md.append(f"- **Usuários com saldo não-nulo sem eventos no diário:** `{bal['no_events_nonzero_balance_count']}`")
    if bal["discrepancies"]:
        md.append("\n### Discrepâncias Detectadas (Pseudonimizadas):")
        for d in bal["discrepancies"]:
            md.append(f"- Usuário `{d['user_pseudonym']}`: Saldo={d['user_balance']} | Diário={d['latest_event_balance_after']} | Diff={d['diff']} ({d['status']})")
    md.append("")

    md.append("## 8. Pendências Operacionais (Pseudonimizadas com HMAC Efêmero)\n")
    md.append(f"### Depósitos Pendentes (`{pending['pending_deposits_count']}`):")
    if pending["pending_deposits"]:
        md.append("| Transação (Pseudônimo) | Usuário (Pseudônimo) | Valor (R$) | Provedor | External ID | Idade (min) |")
        md.append("| :--- | :--- | :---: | :--- | :--- | :---: |")
        for d in pending["pending_deposits"]:
            md.append(f"| `{d['transaction_pseudonym']}` | `{d['user_pseudonym']}` | R$ {d['valor']} | `{d['gateway_provider']}` | `{d['external_id_pseudonym']}` | {d['age_minutes']} |")
    else:
        md.append("Nenhum depósito pendente.")
    md.append("")

    md.append(f"### Saques Pendentes ou em Processamento (`{pending['pending_or_processing_withdrawals_count']}`):")
    if pending["pending_or_processing_withdrawals"]:
        md.append("| Transação | Usuário | Valor | Status | Provedor | External ID | Idade (min) |")
        md.append("| :--- | :--- | :---: | :--- | :--- | :--- | :---: |")
        for s in pending["pending_or_processing_withdrawals"]:
            md.append(f"| `{s['transaction_pseudonym']}` | `{s['user_pseudonym']}` | R$ {s['valor']} | `{s['status']}` | `{s['gateway_provider']}` | `{s['external_id_pseudonym']}` | {s['age_minutes']} |")
    else:
        md.append("Nenhum saque pendente ou em processamento.")
    md.append("")

    md.append("## 9. Qualificação do Histórico Anterior à Migração 5\n")
    md.append(f"{qual['explanation']}\n")
    comp = qual["data_comparison"]
    md.append(f"- **Saldo total em `users.balance`:** `R$ {comp.get('sum_all_users_balance', '0.00')}`")
    md.append(f"- **Soma de depósitos com status `sucesso`:** `R$ {comp.get('sum_deposits_sucesso', '0.00')}`")
    md.append(f"- **Soma de saques com status `sucesso`:** `R$ {comp.get('sum_saques_sucesso', '0.00')}`")
    md.append("\n> **Nota:** Não concluir saldo pela soma de `transactions`. O diário linear `financial_events` a partir da migração 5 é o instrumento contábil de referência.")
    md.append("")

    md.append("## 10. Verificação Auxiliar de Estabilidade Volumétrica de Linhas\n")
    b_counts = meta["safety_model"]["auxiliary_row_count_check"]["counts_before"]
    a_counts = meta["safety_model"]["auxiliary_row_count_check"]["counts_after"]
    md.append("| Tabela | Linhas Pré-Auditoria | Linhas Pós-Auditoria | Delta |")
    md.append("| :--- | :---: | :---: | :---: |")
    for t in sorted(b_counts.keys()):
        md.append(f"| `{t}` | {b_counts[t]} | {a_counts.get(t, 0)} | 0 |")
    md.append("")

    return "\n".join(md)


def write_private_file(file_path, content: str):
    """Escreve arquivo no disco com permissões estritas 0600.
    Segurança:
    - Rejeita symlinks explicitamente (tanto no caminho original quanto no resolvido).
    - Rejeita sobrescrita se o arquivo já existir (O_EXCL).
    - Não altera permissão de diretórios pré-existentes (ex: /tmp); apenas define 0700 em diretórios que o script criar."""
    raw_path = Path(file_path)
    if raw_path.is_symlink():
        raise AuditError(f"Destino de saída '{raw_path}' é um link simbólico, o que é proibido.")

    resolved_path = raw_path.resolve()
    if resolved_path.is_symlink():
        raise AuditError(f"Caminho resolvido '{resolved_path}' é um link simbólico, o que é proibido.")

    if raw_path.exists():
        raise AuditError(f"Arquivo de saída '{raw_path}' já existe. Recusado sobrescrever arquivo existente por segurança.")

    parent_dir = raw_path.parent
    if not parent_dir.exists():
        parent_dir.mkdir(parents=True, mode=0o700, exist_ok=True)
        try:
            os.chmod(parent_dir, 0o700)
        except OSError:
            pass

    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
    try:
        fd = os.open(str(raw_path), flags, 0o600)
    except FileExistsError:
        raise AuditError(f"Arquivo de saída já existe: {raw_path}")
    except OSError as exc:
        raise AuditError(f"Falha ao abrir arquivo de saída de forma segura: {exc}")

    try:
        with open(fd, "w", encoding="utf-8") as f:
            f.write(content)
    except Exception:
        os.close(fd)
        raise

    os.chmod(str(raw_path), 0o600)


def evaluate_check_only_status(report: dict) -> int:
    """
    Avalia o relatório de auditoria para o modo --check-only.

    Retorna:
      0: Nenhuma discrepância de saldo detectada (discrepancies_count == 0) e
         nenhuma quebra de continuidade na cadeia de eventos.
         (Atenção: sob UNTESTED_NO_EVENTS, o código 0 NÃO equivale a aprovação de corte).
      1: Discrepância de saldo detectada (discrepancies_count > 0) OU
         quebra de integridade na cadeia de financial_events
         (status == 'CHAIN_INTEGRITY_BREACH' ou anomalies_count > 0).
    """
    has_balance_discrepancies = report["balance_vs_latest_event"]["discrepancies_count"] > 0
    continuity = report["financial_events_coverage_and_continuity"]["continuity_verification"]
    has_continuity_breach = (
        continuity.get("status") == "CHAIN_INTEGRITY_BREACH"
        or continuity.get("anomalies_count", 0) > 0
    )
    if has_balance_discrepancies or has_continuity_breach:
        return 1
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Auditoria de Conciliação e Inventário Financeiro PostgreSQL (P0.1) — Staging"
    )
    parser.add_argument(
        "--target",
        required=True,
        choices=["staging", "production"],
        help="Alvo obrigatório e explícito da auditoria."
    )
    parser.add_argument(
        "--confirm-staging",
        action="store_true",
        help="Confirmação explícita de execução contra o ambiente staging isolado."
    )
    parser.add_argument("--confirm-production", action="store_true",
                        help="Confirmação explícita de auditoria somente leitura em produção.")
    parser.add_argument(
        "--output-json",
        help="Caminho do arquivo JSON de saída."
    )
    parser.add_argument(
        "--output-md",
        help="Caminho do arquivo Markdown de saída."
    )
    parser.add_argument(
        "--check-only",
        action="store_true",
        help="Falha com código 1 se houver divergência detectada na conciliação ou quebra de continuidade."
    )

    args = parser.parse_args()

    # 1. Validação estrita de alvo e contêiner
    container_id = verify_target_safety(args.target, args.confirm_staging, args.confirm_production)

    # 2. Validação imediata do banco esperado ANTES de qualquer inventário ou contagem
    verify_database_name(args.target)

    # 3. Contagens pré-auditoria (verificação auxiliar de estabilidade)
    counts_before = query_table_row_counts(args.target)

    # 4. Execução da auditoria em transação REPEATABLE READ READ ONLY
    sections = execute_read_only_audit(args.target)

    # 5. Contagens pós-auditoria
    counts_after = query_table_row_counts(args.target)

    # 6. Verificação auxiliar de estabilidade de linhas
    check_table_row_count_stability(counts_before, counts_after)

    # 7. Construção do relatório consolidado com pseudonimização
    report = build_audit_report(container_id, counts_before, counts_after, sections, args.target)
    md_content = render_markdown_report(report)
    json_content = json.dumps(report, indent=2, ensure_ascii=False, cls=DecimalEncoder)

    ts_slug = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%d_%H%M%S")
    reports_dir = ROOT / "scripts" / "release" / "reports"
    json_path = Path(args.output_json) if args.output_json else reports_dir / f"p01_reconcile_{args.target}_{ts_slug}.json"
    md_path = Path(args.output_md) if args.output_md else reports_dir / f"p01_reconcile_{args.target}_{ts_slug}.md"

    write_private_file(json_path, json_content)
    write_private_file(md_path, md_content)

    fe_status = report["financial_events_coverage_and_continuity"]["continuity_verification"]["status"]
    verdict_status = report["verdict"]["status"]

    print(f"=== EXECUÇÃO DA AUDITORIA P0.1 EM {args.target.upper()} ===")
    print(f"Alvo: {report['audit_metadata']['database_name']} (Container: {container_id[:12]})")
    print(f"Isolamento: REPEATABLE READ READ ONLY (ROLLBACK)")
    print(f"Verificação auxiliar de linhas: {len(counts_before)} tabelas estáveis (delta 0)")
    print(f"Usuários: {report['users_inventory']['total_users']} | Saldo Total: R$ {report['users_inventory']['aggregates'].get('sum_balance', '0.00')}")
    print(f"Transações: {report['transactions_inventory']['total_transactions']} (pendências de depósito: {report['pending_operations']['pending_deposits_count']})")
    print(f"Eventos Financeiros: {report['financial_events_coverage_and_continuity']['total_events']} (Status: {fe_status})")
    print(f"Divergências de Saldo: {report['balance_vs_latest_event']['discrepancies_count']}")
    print(f"Veredito Técnico: {verdict_status} ({report['verdict']['p0_status']})")
    if fe_status == "UNTESTED_NO_EVENTS":
        print("Nota de Corte: UNTESTED_NO_EVENTS não equivale a aprovação de corte.")
    print(f"Relatório JSON (0600): {json_path}")
    print(f"Relatório Markdown (0600): {md_path}")
    print("============================================")

    if args.check_only:
        exit_code = evaluate_check_only_status(report)
        if exit_code != 0:
            sys.exit(exit_code)


if __name__ == "__main__":
    try:
        main()
    except AuditError as exc:
        print(f"FALHA NA AUDITORIA P0.1: {exc}", file=sys.stderr)
        sys.exit(2)
