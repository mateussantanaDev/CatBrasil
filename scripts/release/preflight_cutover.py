#!/usr/bin/env python3
"""
scripts/release/preflight_cutover.py

Portão Técnico de Corte (Cutover Preflight Gate) para Abertura em Banco Novo (Greenfield).
Executa verificação fail-closed e estritamente somente leitura (READ ONLY) após
as migrações e ANTES de iniciar ou liberar tráfego na API financeira de produção.

Parâmetros fixos e inegociáveis de produção (Hardcoded Target):
- Projeto Compose: catbrasil-prod
- Serviço: postgres
- Volume de dados: catbrasil_prod_pgdata
- Banco de dados: catbrasil_prod
- Rede externa/isolada: catbrasil_prod_net

Verificações obrigatórias:
1. Inspeção Docker inegociável:
   - Exige label 'com.docker.compose.project' == 'catbrasil-prod'.
   - Exige label 'com.docker.compose.service' == 'postgres'.
   - Exige montagem para /var/lib/postgresql/data com Type == 'volume' e Name == 'catbrasil_prod_pgdata'.
   - Rejeita sumariamente qualquer volume legado ('catbrasil_pgdata' ou 'catbrasil-staging_pgdata').
   - Exige presença da rede 'catbrasil_prod_net'.
2. Identidade Relacional:
   - current_database() DEVE ser 'catbrasil_prod'.
   - Rejeita sumariamente 'catbrasil' (legado) ou 'catbrasil_staging'.
3. Coerência Sequencial de Migrações:
   - schema_migrations DEVE conter exatamente a sequência contínua das migrações da fonte.
4. Integridade de Schema:
   - Todas as tabelas obrigatórias das migrações da fonte devem existir no schema público.
   - Tabelas de configuração e catálogo com seeds podem conter dados.
5. Estado Zero Operacional Estrito (Fail-Closed):
   - Exige exatamente ZERO linhas em TODAS as tabelas operacionais:
     users, transactions, financial_events, payout_gateway_events,
     sms_verifications, user_identities, identity_events, machines,
     referrals, user_promos, battle_pass_season_history, ranking_season_history,
     mining_race_weekly, usage_logs, security_logs, deposit_gateway_events,
     cashin_deposit_intents, cashin_event_queue, payout_submissions, payout_event_queue.
   - Exige saldo total e saldo de rendimentos exatamente R$ 0.00.
   - Qualquer tabela ausente ou falha de leitura é rotulada como UNKNOWN e bloqueia o corte.
   - Catálogo, níveis com bônus, roleta e campanhas fictícios devem estar vazios; configuração
     econômica deve ter saques travados, aprovação automática desligada e valores zerados.
6. Segurança da Informação:
   - Execução sob transação REPEATABLE READ READ ONLY com statement_timeout de 5000ms e ROLLBACK.
   - Jamais imprime dados pessoais (PII), senhas, hashes, PINs ou segredos.

Códigos de saída:
  0: APROVADO — Estado zero comprovado em produção (APPROVED_CLEAN_BASELINE).
  1: BLOQUEADO — Violação de estado zero ou migrações incompletas (VIOLATIONS_DETECTED).
  2: ERRO FATAL / PARADA — Rejeição de infraestrutura, volume/projeto/rede inválidos ou erro de conexão.
"""

import argparse
import decimal
import json
import os
from pathlib import Path
import subprocess
import sys
from typing import Any, Dict, List, Optional

ROOT = Path(__file__).resolve().parents[2]

# Metas fixas e inegociáveis de produção
PROD_PROJECT = "catbrasil-prod"
PROD_SERVICE = "postgres"
PROD_VOLUME = "catbrasil_prod_pgdata"
PROD_DATABASE = "catbrasil_prod"
PROD_NETWORK = "catbrasil_prod_net"

FORBIDDEN_VOLUMES = {"catbrasil_pgdata", "catbrasil-staging_pgdata"}
FORBIDDEN_DATABASES = {"catbrasil", "catbrasil_staging"}
EXPECTED_MIGRATION_SEQUENCE = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14]

# 28 tabelas criadas nas migrações 1..14.
SEEDED_CATALOG_TABLES = [
    "system_config",
    "fleet_catalog",
    "level_configs",
    "roulette_slices",
    "announcements"
]

OPERATIONAL_ZERO_STATE_TABLES = [
    "users",
    "transactions",
    "financial_events",
    "payout_gateway_events",
    "sms_verifications",
    "user_identities",
    "identity_events",
    "machines",
    "referrals",
    "user_promos",
    "battle_pass_season_history",
    "ranking_season_history",
    "mining_race_weekly",
    "usage_logs",
    "security_logs",
    "deposit_gateway_events",
    "cashin_deposit_intents",
    "cashin_event_queue",
    "payout_submissions",
    "payout_event_queue",
    "http_rate_limits",
    "cron_executions"
]

ALL_REQUIRED_TABLES = ["schema_migrations"] + SEEDED_CATALOG_TABLES + OPERATIONAL_ZERO_STATE_TABLES


class PreflightError(Exception):
    """Exceção levantada em caso de erro fatal de infraestrutura ou configuração."""
    pass


class DecimalEncoder(json.JSONEncoder):
    def default(self, o):
        if isinstance(o, decimal.Decimal):
            return str(o)
        return super().default(o)


def inspect_docker_target(container_name_or_id: str) -> Dict[str, Any]:
    """
    Inspeciona o contêiner PostgreSQL de produção via Docker CLI.
    Valida estritamente labels, serviço, volume de dados e rede de produção.
    Não permite parametrização de alvos para a execução real.
    """
    try:
        proc = subprocess.run(
            ["docker", "inspect", container_name_or_id],
            capture_output=True,
            text=True,
            check=True,
            timeout=10
        )
        data = json.loads(proc.stdout)
        if not data or not isinstance(data, list):
            raise PreflightError(f"docker inspect retornou saída vazia ou inválida para {container_name_or_id}")
        info = data[0]
    except subprocess.CalledProcessError as exc:
        raise PreflightError(f"Falha ao inspecionar contêiner '{container_name_or_id}': código {exc.returncode}") from None
    except Exception as exc:
        raise PreflightError(f"Erro ao consultar Docker para contêiner '{container_name_or_id}': {exc}") from None

    # 1. Estado de execução
    state = info.get("State", {})
    if not state.get("Running", False):
        raise PreflightError(f"Contêiner '{container_name_or_id}' não está em execução (status: {state.get('Status')})")

    # 2. Labels Compose obrigatórios
    labels = info.get("Config", {}).get("Labels", {}) or {}
    project = labels.get("com.docker.compose.project")
    service = labels.get("com.docker.compose.service")

    if not project:
        raise PreflightError("LABEL_AUSENTE: Label 'com.docker.compose.project' ausente no contêiner.")
    if project != PROD_PROJECT:
        raise PreflightError(
            f"PROJETO_INVALIDO: Projeto Compose '{project}' difere do alvo obrigatório de produção '{PROD_PROJECT}'."
        )

    if not service:
        raise PreflightError("LABEL_AUSENTE: Label 'com.docker.compose.service' ausente no contêiner.")
    if service != PROD_SERVICE:
        raise PreflightError(
            f"SERVICO_INVALIDO: Serviço Compose '{service}' difere do esperado '{PROD_SERVICE}'."
        )

    # 3. Montagem do volume de dados em /var/lib/postgresql/data
    mounts = info.get("Mounts", [])
    data_mount = None
    for m in mounts:
        if m.get("Destination") == "/var/lib/postgresql/data":
            data_mount = m
            break

    if not data_mount:
        raise PreflightError("MONTAGEM_AUSENTE: Não foi localizada montagem para /var/lib/postgresql/data no contêiner.")

    mount_type = data_mount.get("Type")
    volume_name = data_mount.get("Name", "")

    if mount_type != "volume":
        raise PreflightError(
            f"TIPO_MONTAGEM_INVALIDO: Montagem em /var/lib/postgresql/data deve ser do tipo 'volume', encontrada: '{mount_type}'."
        )

    if volume_name in FORBIDDEN_VOLUMES:
        raise PreflightError(
            f"VOLUME_PROIBIDO: O volume montado é '{volume_name}', pertencente ao histórico legado/staging. "
            f"O corte greenfield proíbe estritamente o reuso de dados antigos."
        )

    if volume_name != PROD_VOLUME:
        raise PreflightError(
            f"VOLUME_DIVERGENTE: Volume montado '{volume_name}' não é o volume de produção fixado '{PROD_VOLUME}'."
        )

    # 4. Rede isolada de produção
    networks = info.get("NetworkSettings", {}).get("Networks", {}) or {}
    if PROD_NETWORK not in networks:
        raise PreflightError(
            f"REDE_AUSENTE: O contêiner não está conectado à rede de produção '{PROD_NETWORK}' (redes: {list(networks.keys())})."
        )

    return {
        "container_id": info.get("Id", "")[:12],
        "container_name": info.get("Name", "").lstrip("/"),
        "project": project,
        "service": service,
        "volume_name": volume_name,
        "mount_type": mount_type,
        "network": PROD_NETWORK,
        "status": state.get("Status", "running")
    }


# Mapeamento estrito de métrica operacional para tabela de origem
OPERATIONAL_METRIC_QUERIES = {
    # P0.10: qualquer catálogo econômico ou campanha fictícia bloqueia o corte.
    ("fleet_catalog", "fleet_catalog_count"): "(SELECT count(*) FROM fleet_catalog)",
    ("level_configs", "level_configs_count"): "(SELECT count(*) FROM level_configs)",
    ("roulette_slices", "roulette_slices_count"): "(SELECT count(*) FROM roulette_slices)",
    ("announcements", "announcements_count"): "(SELECT count(*) FROM announcements)",
    ("system_config", "unsafe_system_config_count"): "(SELECT count(*) FROM system_config WHERE id <> 1 OR global_withdrawals_locked IS NOT TRUE OR auto_approve_enabled IS NOT FALSE OR auto_approve_withdrawals_under <> 0 OR referral_commission_percent <> 0 OR bonus_per_mutirao_amount <> 0 OR deposit_fee_percent <> 0 OR withdraw_fee_fixed <> 0 OR suitpay_enabled IS NOT FALSE)",
    ("system_config", "system_config_missing_count"): "(SELECT CASE WHEN count(*) = 1 THEN 0 ELSE 1 END FROM system_config)",
    # users
    ("users", "users_count"): "(SELECT count(*) FROM users)",
    ("users", "users_balance_sum"): "(SELECT COALESCE(sum(balance), 0.00)::text FROM users)",
    ("users", "users_yield_balance_sum"): "(SELECT COALESCE(sum(yield_balance_available), 0.00)::text FROM users)",
    # transactions
    ("transactions", "transactions_count"): "(SELECT count(*) FROM transactions)",
    ("transactions", "pending_deposits_count"): "(SELECT count(*) FROM transactions WHERE tipo = 'Depósito PIX' AND status = 'pendente')",
    ("transactions", "pending_withdrawals_count"): "(SELECT count(*) FROM transactions WHERE tipo = 'Saque PIX' AND status IN ('pendente', 'processando'))",
    # financial_events
    ("financial_events", "financial_events_count"): "(SELECT count(*) FROM financial_events)",
    # payout_gateway_events
    ("payout_gateway_events", "payout_gateway_events_count"): "(SELECT count(*) FROM payout_gateway_events)",
    # sms_verifications
    ("sms_verifications", "sms_verifications_count"): "(SELECT count(*) FROM sms_verifications)",
    # user_identities
    ("user_identities", "user_identities_count"): "(SELECT count(*) FROM user_identities)",
    # identity_events
    ("identity_events", "identity_events_count"): "(SELECT count(*) FROM identity_events)",
    # machines
    ("machines", "machines_count"): "(SELECT count(*) FROM machines)",
    # referrals
    ("referrals", "referrals_count"): "(SELECT count(*) FROM referrals)",
    # user_promos
    ("user_promos", "user_promos_count"): "(SELECT count(*) FROM user_promos)",
    # battle_pass_season_history
    ("battle_pass_season_history", "battle_pass_season_history_count"): "(SELECT count(*) FROM battle_pass_season_history)",
    # ranking_season_history
    ("ranking_season_history", "ranking_season_history_count"): "(SELECT count(*) FROM ranking_season_history)",
    # mining_race_weekly
    ("mining_race_weekly", "mining_race_weekly_count"): "(SELECT count(*) FROM mining_race_weekly)",
    # usage_logs
    ("usage_logs", "usage_logs_count"): "(SELECT count(*) FROM usage_logs)",
    # security_logs
    ("security_logs", "security_logs_count"): "(SELECT count(*) FROM security_logs)",
    # deposit_gateway_events
    ("deposit_gateway_events", "deposit_gateway_events_count"): "(SELECT count(*) FROM deposit_gateway_events)",
    # cashin_deposit_intents
    ("cashin_deposit_intents", "cashin_deposit_intents_count"): "(SELECT count(*) FROM cashin_deposit_intents)",
    # cashin_event_queue
    ("cashin_event_queue", "cashin_event_queue_count"): "(SELECT count(*) FROM cashin_event_queue)",
    # payout_submissions
    ("payout_submissions", "payout_submissions_count"): "(SELECT count(*) FROM payout_submissions)",
    ("payout_submissions", "unlinked_payout_submissions_count"): "(SELECT count(*) FROM payout_submissions WHERE external_id IS NULL)",
    # payout_event_queue
    ("payout_event_queue", "payout_event_queue_count"): "(SELECT count(*) FROM payout_event_queue)",
    ("payout_event_queue", "pending_payout_events_count"): "(SELECT count(*) FROM payout_event_queue WHERE queue_status = 'pending')",
    # http_rate_limits
    ("http_rate_limits", "http_rate_limits_count"): "(SELECT count(*) FROM http_rate_limits)",
    # cron_executions
    ("cron_executions", "cron_executions_count"): "(SELECT count(*) FROM cron_executions)"
}


def build_schema_inventory_sql(timeout_ms: int = 5000) -> str:
    """
    Gera a query SQL somente leitura para inventário de metadados e existência de tabelas.
    Utiliza to_regclass, garantindo que o PostgreSQL NUNCA falhe no parse/planejamento
    caso qualquer tabela de catálogo, operacional ou schema_migrations esteja ausente.
    """
    tables_array = ", ".join(f"'{t}'" for t in ALL_REQUIRED_TABLES)

    return f"""
    BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
    SET TRANSACTION READ ONLY;
    SET LOCAL statement_timeout = '{timeout_ms}ms';

    -- 1. Metadados do banco de dados
    SELECT 'SECTION_METADATA_BEGIN';
    SELECT json_build_object(
      'current_database', current_database(),
      'current_user', current_user,
      'server_version', version()
    );
    SELECT 'SECTION_METADATA_END';

    -- 2. Existência de todas as tabelas obrigatórias + schema_migrations
    SELECT 'SECTION_TABLES_BEGIN';
    SELECT COALESCE(json_object_agg(tbl, (to_regclass('public.' || tbl) IS NOT NULL)), '{{}}'::json)
    FROM unnest(ARRAY[{tables_array}]) AS tbl;
    SELECT 'SECTION_TABLES_END';

    ROLLBACK;
    """


def build_state_audit_sql(tables: Dict[str, bool], timeout_ms: int = 5000) -> Optional[str]:
    """
    Gera a query SQL somente leitura para leitura de migrações e métricas de estado zero,
    referenciando EXCLUSIVAMENTE tabelas cuja existência foi comprovada na fase de inventário.
    Desta forma, evita qualquer erro de parse no PostgreSQL (relation does not exist).
    """
    has_migrations = bool(tables.get("schema_migrations"))

    query_fields = []
    for (tbl, metric_name), sql_expr in OPERATIONAL_METRIC_QUERIES.items():
        if tables.get(tbl) is True:
            query_fields.append(f"'{metric_name}', {sql_expr}")

    if not has_migrations and not query_fields:
        return None

    sql_parts = [
        "BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;",
        "SET TRANSACTION READ ONLY;",
        f"SET LOCAL statement_timeout = '{timeout_ms}ms';"
    ]

    if has_migrations:
        sql_parts.append("""
    -- 1. Migrações e coerência sequencial de versões (tabela comprovada)
    SELECT 'SECTION_MIGRATIONS_BEGIN';
    SELECT json_build_object(
      'table_exists', true,
      'versions_applied', COALESCE(array_to_json(array_agg(version ORDER BY version)), '[]'::json),
      'max_version', COALESCE(MAX(version), 0),
      'migrations_count', COUNT(*)
    ) FROM schema_migrations;
    SELECT 'SECTION_MIGRATIONS_END';
        """)

    if query_fields:
        fields_str = ",\n      ".join(query_fields)
        sql_parts.append(f"""
    -- 2. Métricas de estado zero estrito (somente tabelas operacionais comprovadas)
    SELECT 'SECTION_ZERO_STATE_BEGIN';
    SELECT json_build_object(
      {fields_str}
    );
    SELECT 'SECTION_ZERO_STATE_END';
        """)

    sql_parts.append("ROLLBACK;")
    return "\n".join(sql_parts)


def build_preflight_sql(timeout_ms: int = 5000) -> str:
    """Compatibilidade retroativa: retorna SQL da fase de inventário seguro."""
    return build_schema_inventory_sql(timeout_ms=timeout_ms)


def parse_preflight_sections(
    raw_output: str,
    required_tokens: Optional[List[str]] = None
) -> Dict[str, Any]:
    """Extrai e desserializa as seções JSON emitidas pelos blocos SQL."""
    sections = {}
    tokens = required_tokens if required_tokens is not None else ["METADATA", "MIGRATIONS", "TABLES", "ZERO_STATE"]

    for tok in tokens:
        begin_tag = f"SECTION_{tok}_BEGIN"
        end_tag = f"SECTION_{tok}_END"
        if begin_tag not in raw_output or end_tag not in raw_output:
            raise PreflightError(f"Seção SQL '{tok}' ausente na resposta do banco de dados (fail-closed)")

        part = raw_output.split(begin_tag, 1)[1].split(end_tag, 1)[0].strip()
        try:
            sections[tok] = json.loads(part, parse_float=decimal.Decimal)
        except Exception:
            raise PreflightError(f"Falha ao decodificar JSON na seção '{tok}' (fail-closed)") from None

    return sections


def execute_sql_preflight_audit(container_name_or_id: str, timeout_ms: int = 5000) -> Dict[str, Any]:
    """
    Executa a auditoria SQL segregada em duas fases fail-closed:
    Fase 1: Inventário de banco, metadados e existência de tabelas via to_regclass (sem tocar relações diretamente).
    Fase 2: Leitura de versões de schema_migrations e métricas zero-state APENAS para tabelas existentes.
    Tabelas ausentes têm suas métricas marcadas como None e geram VIOLATIONS_DETECTED (exit 1) sem erro de parse SQL.
    """
    # Fase 1: Inventário de schema e tabelas
    inv_sql = build_schema_inventory_sql(timeout_ms=timeout_ms)
    inv_raw = run_psql_in_docker(container_name_or_id, inv_sql)
    inv_sections = parse_preflight_sections(inv_raw, required_tokens=["METADATA", "TABLES"])

    meta = inv_sections.get("METADATA", {})
    tables = inv_sections.get("TABLES", {})

    has_migrations = bool(tables.get("schema_migrations"))
    state_sql = build_state_audit_sql(tables, timeout_ms=timeout_ms)

    migrations: Dict[str, Any] = {
        "table_exists": has_migrations,
        "versions_applied": [],
        "max_version": None,
        "migrations_count": 0
    }
    zero_state: Dict[str, Any] = {}

    if state_sql:
        expected_tokens = []
        if has_migrations:
            expected_tokens.append("MIGRATIONS")
        has_metrics_to_query = any(tables.get(tbl) is True for (tbl, _) in OPERATIONAL_METRIC_QUERIES.keys())
        if has_metrics_to_query:
            expected_tokens.append("ZERO_STATE")

        state_raw = run_psql_in_docker(container_name_or_id, state_sql)
        state_sections = parse_preflight_sections(state_raw, required_tokens=expected_tokens)

        if has_migrations and "MIGRATIONS" in state_sections:
            migrations = state_sections["MIGRATIONS"]
        if "ZERO_STATE" in state_sections:
            zero_state = state_sections["ZERO_STATE"]

    # Garante que todas as métricas operacionais esperadas estão no dicionário zero_state;
    # se a tabela de origem não existia, a métrica fica como None (status: UNKNOWN fail-closed).
    for (tbl, metric_name) in OPERATIONAL_METRIC_QUERIES.keys():
        if metric_name not in zero_state:
            zero_state[metric_name] = None

    return {
        "METADATA": meta,
        "MIGRATIONS": migrations,
        "TABLES": tables,
        "ZERO_STATE": zero_state
    }


def evaluate_preflight_results(
    sections: Dict[str, Any],
    is_proof_mode: bool = False
) -> Dict[str, Any]:
    """
    Avalia estritamente os resultados do preflight.
    Se for proof mode, valida a lógica de dados, mas NUNCA autoriza abertura de produção.
    """
    meta = sections.get("METADATA", {})
    migrations = sections.get("MIGRATIONS", {})
    tables = sections.get("TABLES", {})
    zero_state = sections.get("ZERO_STATE", {})

    violations: List[str] = []
    fatal_errors: List[str] = []

    # 1. Banco de dados
    curr_db = meta.get("current_database", "")
    if curr_db in FORBIDDEN_DATABASES:
        fatal_errors.append(
            f"BANCO_PROIBIDO: Banco ativo '{curr_db}' é legado ou staging! "
            f"O corte greenfield exige banco de produção fixado '{PROD_DATABASE}'."
        )
    elif curr_db != PROD_DATABASE:
        fatal_errors.append(
            f"BANCO_DIVERGENTE: Banco ativo '{curr_db}' difere do banco de produção fixado '{PROD_DATABASE}'."
        )

    # 2. Coerência sequencial de migrações (deve ser exatamente [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11])
    has_mig_table = migrations.get("table_exists", False)
    if not has_mig_table:
        violations.append("METRICA_UNKNOWN: Tabela 'schema_migrations' não existe no banco de dados.")
    else:
        applied_versions = migrations.get("versions_applied", [])
        if applied_versions != EXPECTED_MIGRATION_SEQUENCE:
            violations.append(
                f"MIGRACOES_INCOERENTES: Versões aplicadas {applied_versions} diferem da sequência obrigatória {EXPECTED_MIGRATION_SEQUENCE}."
            )

    # 3. Existência de todas as tabelas requeridas
    for t in ALL_REQUIRED_TABLES:
        exists = tables.get(t)
        if exists is not True:
            violations.append(f"METRICA_UNKNOWN: Tabela requerida '{t}' ausente no schema público (status: UNKNOWN).")

    # 4. Estado zero estrito nas tabelas operacionais
    for metric_name in {metric for _, metric in OPERATIONAL_METRIC_QUERIES}:
        if metric_name not in zero_state:
            violations.append(f"METRICA_UNKNOWN: Métrica operacional '{metric_name}' ausente (status: UNKNOWN).")
    for metric_name, val in zero_state.items():
        if val is None:
            violations.append(f"METRICA_UNKNOWN: Métrica operacional '{metric_name}' retornou nula ou inacessível (status: UNKNOWN).")
            continue

        if "balance_sum" in metric_name:
            dec_val = decimal.Decimal(str(val))
            if dec_val != decimal.Decimal("0.00"):
                violations.append(
                    f"ESTADO_NAO_ZERO: Métrica '{metric_name}' possui saldo acumulado de R$ {dec_val} (esperado 0.00)."
                )
        else:
            int_val = int(val)
            if int_val != 0:
                risk_note = ""
                if metric_name == "unlinked_payout_submissions_count":
                    risk_note = " [RISCO_OPERACIONAL: Submissões de saque sem retorno do provedor vinculado]"
                elif metric_name == "pending_payout_events_count":
                    risk_note = " [RISCO_OPERACIONAL: Eventos de saque pendentes de reconciliação manual]"
                violations.append(
                    f"ESTADO_NAO_ZERO: Métrica '{metric_name}' possui contagem {int_val} (esperado 0){risk_note}."
                )

    if fatal_errors:
        gate_status = "FATAL_CONFIG_ERROR"
        exit_code = 2
        gate_auth = False
    elif violations:
        gate_status = "VIOLATIONS_DETECTED"
        exit_code = 1
        gate_auth = False
    else:
        if is_proof_mode:
            gate_status = "PROOF_MODE_ZERO_STATE_VERIFIED"
            exit_code = 0
            gate_auth = False  # MODO PROVA JAMAIS AUTORIZA ABERTURA DE PRODUÇÃO!
        else:
            gate_status = "APPROVED_CLEAN_BASELINE"
            exit_code = 0
            gate_auth = True

    return {
        "status": gate_status,
        "gate_authorized": gate_auth,
        "is_proof_mode": is_proof_mode,
        "exit_code": exit_code,
        "current_database": curr_db,
        "expected_database": PROD_DATABASE,
        "server_version": meta.get("server_version", ""),
        "migrations": migrations,
        "tables_checked_count": len(tables),
        "zero_state_metrics": zero_state,
        "fatal_errors": fatal_errors,
        "violations": violations
    }


def run_psql_in_docker(container_name_or_id: str, sql_query: str) -> str:
    """Executa script SQL via docker exec dentro do contêiner alvo."""
    cmd = [
        "docker", "exec", "-i", container_name_or_id,
        "sh", "-c",
        'exec psql -XAt -v ON_ERROR_STOP=1 -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-catbrasil_prod}"'
    ]
    try:
        proc = subprocess.run(
            cmd,
            input=sql_query,
            text=True,
            capture_output=True,
            check=True,
            timeout=15
        )
        return proc.stdout
    except subprocess.CalledProcessError as exc:
        raise PreflightError(f"Execução SQL no contêiner falhou com código {exc.returncode}") from None
    except Exception as exc:
        raise PreflightError(f"Falha ao comunicar com contêiner '{container_name_or_id}': {exc}") from None


def main():
    parser = argparse.ArgumentParser(
        description="Portão Técnico de Corte (Cutover Preflight Gate) para Produção Greenfield"
    )
    parser.add_argument(
        "--container",
        required=True,
        help="Nome ou ID do contêiner PostgreSQL de produção (ex: catbrasil-prod-postgres-1)."
    )
    parser.add_argument(
        "--statement-timeout-ms",
        type=int,
        default=5000,
        help="Timeout de comando SQL em milissegundos (padrão: 5000ms)."
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emite saída estruturada em JSON."
    )
    parser.add_argument(
        "--proof-mode-evaluation-only",
        action="store_true",
        help=(
            "Modo restrito de teste para validação de lógica em contêineres efêmeros sem labels Compose. "
            "AVISO DE SEGURANÇA: Este modo JAMAIS emite aprovação de corte de produção (gate_authorized=False)."
        )
    )

    args = parser.parse_args()

    # 1. Inspeção inegociável de infraestrutura Docker (exceto no modo explícito de prova efêmera)
    volume_info = None
    if not args.proof_mode_evaluation_only:
        try:
            volume_info = inspect_docker_target(args.container)
        except PreflightError as exc:
            print(f"BLOQUEIO FATAL DE INFRAESTRUTURA: {exc}", file=sys.stderr)
            sys.exit(2)

    # 2. Execução da auditoria SQL em modo READ ONLY (fases segregadas fail-closed)
    try:
        sections = execute_sql_preflight_audit(args.container, timeout_ms=args.statement_timeout_ms)
    except PreflightError as exc:
        print(f"BLOQUEIO FATAL DE CONSULTA SQL: {exc}", file=sys.stderr)
        sys.exit(2)

    # 3. Avaliação de conformidade
    evaluation = evaluate_preflight_results(
        sections,
        is_proof_mode=args.proof_mode_evaluation_only
    )
    if volume_info:
        evaluation["infrastructure"] = volume_info

    if args.json:
        print(json.dumps(evaluation, indent=2, ensure_ascii=False, cls=DecimalEncoder))
    else:
        print("=================================================================")
        print("  PORTÃO TÉCNICO DE CORTE (CUTOVER PREFLIGHT GATE) — GREENFIELD")
        print("=================================================================")
        if volume_info:
            print(f"Contêiner: {volume_info['container_name']} ({volume_info['container_id']})")
            print(f"Projeto Compose: {volume_info['project']} | Serviço: {volume_info['service']}")
            print(f"Volume Montado: {volume_info['volume_name']} (Tipo: {volume_info['mount_type']})")
            print(f"Rede Isolada: {volume_info['network']}")
        else:
            print(f"Contêiner: {args.container} (MODO DE PROVA EFÊMERA — SEM AUTORIZAÇÃO DE CORTE)")

        print(f"Banco Ativo: {evaluation['current_database']} (fixado: {PROD_DATABASE})")
        print(f"Transação: REPEATABLE READ READ ONLY (ROLLBACK)")
        mig = evaluation["migrations"]
        print(f"Migrações: versão {mig.get('max_version')} (sequência: {mig.get('versions_applied')})")
        print(f"Tabelas Verificadas: {evaluation['tables_checked_count']}")
        print("-----------------------------------------------------------------")
        print("Métricas de Estado Zero (Tabelas Operacionais):")
        for k, v in evaluation["zero_state_metrics"].items():
            print(f"  - {k}: {v}")
        print("-----------------------------------------------------------------")

        if evaluation["gate_authorized"]:
            print("STATUS DO PORTÃO: APROVADO (APPROVED_CLEAN_BASELINE)")
            print("Resultado: Banco novo 100% limpo, estruturado e autorizado para API financeira no caminho suportado de deploy.")
        elif evaluation["is_proof_mode"]:
            if evaluation["exit_code"] == 0:
                print("STATUS DO PORTÃO: PROVA CONCLUÍDA (PROOF_MODE_ZERO_STATE_VERIFIED)")
                print("Nota: Zero-state validado em modo de prova. Não autoriza abertura de produção.")
            else:
                print(f"STATUS DO PORTÃO: BLOQUEADO EM PROVA ({evaluation['status']})")
                for viol in evaluation["violations"]:
                    print(f"  [VIOLACAO] {viol}")
        else:
            print(f"STATUS DO PORTÃO: BLOQUEADO ({evaluation['status']})")
            if evaluation["fatal_errors"]:
                print("Erros Fatais:")
                for err in evaluation["fatal_errors"]:
                    print(f"  [FATAL] {err}")
            if evaluation["violations"]:
                print("Violações de Estado Zero / Migrações:")
                for viol in evaluation["violations"]:
                    print(f"  [VIOLACAO] {viol}")
        print("=================================================================")

    sys.exit(evaluation["exit_code"])


if __name__ == "__main__":
    main()
