#!/usr/bin/env python3
"""
tests/test_preflight_cutover.py

Testes automatizados unitários para o portão técnico de corte (preflight_cutover.py).
Cobre:
1. Rejeição incondicional de volumes proibidos (catbrasil_pgdata legado e catbrasil-staging_pgdata).
2. Rejeição de projetos divergentes de 'catbrasil-prod' ou labels ausentes.
3. Rejeição de serviços divergentes de 'postgres' ou labels ausentes.
4. Rejeição de tipo de montagem diferente de 'volume' (ex: bind mount).
5. Rejeição de contêiner desconectado da rede 'catbrasil_prod_net'.
6. Rejeição de bancos proibidos ('catbrasil' legado e 'catbrasil_staging') ou divergentes de 'catbrasil_prod'.
7. Rejeição por incoerência sequencial de migrações (deve ser exatamente [1..11]).
8. Bloqueio fail-closed por métrica UNKNOWN (tabelas críticas ou operacionais ausentes).
9. Bloqueio fail-closed por violação de estado zero nas 20 tabelas operacionais.
10. Comprovação de que o modo de prova efêmera JAMAIS autoriza corte de produção (gate_authorized=False).
11. Aprovação com exit code 0 e gate_authorized=True quando todos os requisitos fixados de produção são satisfeitos.
"""

import decimal
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch, MagicMock

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from scripts.release.preflight_cutover import (
    PreflightError,
    inspect_docker_target,
    evaluate_preflight_results,
    ALL_REQUIRED_TABLES,
    EXPECTED_MIGRATION_SEQUENCE,
    OPERATIONAL_ZERO_STATE_TABLES,
    PROD_PROJECT,
    PROD_SERVICE,
    PROD_VOLUME,
    PROD_DATABASE,
    PROD_NETWORK
)


class TestDockerTargetInspection(unittest.TestCase):
    """Testa validação estrita e inegociável de labels, serviço, volume e rede."""

    def _make_docker_inspect_output(
        self,
        project=PROD_PROJECT,
        service=PROD_SERVICE,
        volume_name=PROD_VOLUME,
        mount_type="volume",
        network=PROD_NETWORK,
        running=True
    ):
        labels = {}
        if project is not None:
            labels["com.docker.compose.project"] = project
        if service is not None:
            labels["com.docker.compose.service"] = service

        networks = {}
        if network is not None:
            networks[network] = {"NetworkID": "net123"}

        return json.dumps([{
            "Id": "abc123456789",
            "Name": "/catbrasil-prod-postgres-1",
            "State": {"Running": running, "Status": "running" if running else "exited"},
            "Config": {"Labels": labels},
            "Mounts": [
                {
                    "Type": mount_type,
                    "Name": volume_name,
                    "Destination": "/var/lib/postgresql/data"
                }
            ],
            "NetworkSettings": {"Networks": networks}
        }])

    @patch("subprocess.run")
    def test_reject_legacy_volume_catbrasil_pgdata(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(volume_name="catbrasil_pgdata"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("VOLUME_PROIBIDO", str(ctx.exception))
        self.assertIn("catbrasil_pgdata", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_staging_volume(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(volume_name="catbrasil-staging_pgdata"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("VOLUME_PROIBIDO", str(ctx.exception))
        self.assertIn("catbrasil-staging_pgdata", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_legacy_project_catbrasil(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(project="catbrasil"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("PROJETO_INVALIDO", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_missing_project_label(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(project=None))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("LABEL_AUSENTE", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_invalid_service_label(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(service="backend-api"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("SERVICO_INVALIDO", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_invalid_mount_type(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(mount_type="bind"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("TIPO_MONTAGEM_INVALIDO", str(ctx.exception))

    @patch("subprocess.run")
    def test_reject_missing_prod_network(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output(network="bridge"))
        with self.assertRaises(PreflightError) as ctx:
            inspect_docker_target("fake_container")
        self.assertIn("REDE_AUSENTE", str(ctx.exception))
        self.assertIn(PROD_NETWORK, str(ctx.exception))

    @patch("subprocess.run")
    def test_accept_valid_prod_container(self, mock_run):
        mock_run.return_value = MagicMock(stdout=self._make_docker_inspect_output())
        res = inspect_docker_target("fake_container")
        self.assertEqual(res["project"], PROD_PROJECT)
        self.assertEqual(res["service"], PROD_SERVICE)
        self.assertEqual(res["volume_name"], PROD_VOLUME)
        self.assertEqual(res["mount_type"], "volume")
        self.assertEqual(res["network"], PROD_NETWORK)


class TestPreflightEvaluationLogic(unittest.TestCase):
    """Testa a avaliação lógica estrita do portão de corte."""

    def _make_valid_sections(self, db=PROD_DATABASE):
        tables_dict = {t: True for t in ALL_REQUIRED_TABLES}
        zero_state_dict = {
            "fleet_catalog_count": 0,
            "level_configs_count": 0,
            "roulette_slices_count": 0,
            "announcements_count": 0,
            "unsafe_system_config_count": 0,
            "system_config_missing_count": 0,
            "users_count": 0,
            "users_balance_sum": "0.00",
            "users_yield_balance_sum": "0.00",
            "transactions_count": 0,
            "financial_events_count": 0,
            "pending_deposits_count": 0,
            "pending_withdrawals_count": 0,
            "payout_gateway_events_count": 0,
            "sms_verifications_count": 0,
            "user_identities_count": 0,
            "identity_events_count": 0,
            "machines_count": 0,
            "referrals_count": 0,
            "user_promos_count": 0,
            "battle_pass_season_history_count": 0,
            "ranking_season_history_count": 0,
            "mining_race_weekly_count": 0,
            "usage_logs_count": 0,
            "security_logs_count": 0,
            "deposit_gateway_events_count": 0,
            "cashin_deposit_intents_count": 0,
            "cashin_event_queue_count": 0,
            "payout_submissions_count": 0,
            "unlinked_payout_submissions_count": 0,
            "payout_event_queue_count": 0,
            "pending_payout_events_count": 0,
            "http_rate_limits_count": 0,
            "cron_executions_count": 0
        }
        return {
            "METADATA": {
                "current_database": db,
                "current_user": "catadmin",
                "server_version": "PostgreSQL 16.2"
            },
            "MIGRATIONS": {
                "table_exists": True,
                "versions_applied": EXPECTED_MIGRATION_SEQUENCE,
                "max_version": EXPECTED_MIGRATION_SEQUENCE[-1],
                "migrations_count": len(EXPECTED_MIGRATION_SEQUENCE)
            },
            "TABLES": tables_dict,
            "ZERO_STATE": zero_state_dict
        }

    def test_approved_when_all_zero_and_schema_complete(self):
        sec = self._make_valid_sections()
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "APPROVED_CLEAN_BASELINE")
        self.assertTrue(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 0)
        self.assertEqual(len(eval_res["violations"]), 0)
        self.assertEqual(len(eval_res["fatal_errors"]), 0)

    def test_demo_catalog_and_missing_metric_block_cutover(self):
        sec = self._make_valid_sections()
        sec["ZERO_STATE"]["fleet_catalog_count"] = 1
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertFalse(eval_res["gate_authorized"])
        self.assertTrue(any("fleet_catalog_count" in violation for violation in eval_res["violations"]))

        sec = self._make_valid_sections()
        del sec["ZERO_STATE"]["unsafe_system_config_count"]
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertFalse(eval_res["gate_authorized"])
        self.assertTrue(any("unsafe_system_config_count" in violation for violation in eval_res["violations"]))

    def test_proof_mode_never_authorizes_cutover(self):
        sec = self._make_valid_sections()
        eval_res = evaluate_preflight_results(sec, is_proof_mode=True)
        self.assertEqual(eval_res["status"], "PROOF_MODE_ZERO_STATE_VERIFIED")
        self.assertFalse(eval_res["gate_authorized"])  # Deve ser FALSE mesmo com exit code 0
        self.assertEqual(eval_res["exit_code"], 0)

    def test_fatal_error_when_database_is_legacy(self):
        sec = self._make_valid_sections(db="catbrasil")
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "FATAL_CONFIG_ERROR")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 2)
        self.assertTrue(any("BANCO_PROIBIDO" in err for err in eval_res["fatal_errors"]))

    def test_fatal_error_when_database_is_staging(self):
        sec = self._make_valid_sections(db="catbrasil_staging")
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "FATAL_CONFIG_ERROR")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 2)
        self.assertTrue(any("BANCO_PROIBIDO" in err for err in eval_res["fatal_errors"]))

    def test_block_when_migrations_incoherent(self):
        sec = self._make_valid_sections()
        sec["MIGRATIONS"]["versions_applied"] = [1, 2, 3, 5, 6, 7, 8]  # Faltando versão 4
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        self.assertTrue(any("MIGRACOES_INCOERENTES" in v for v in eval_res["violations"]))

    def test_block_when_operational_tables_non_zero(self):
        # Testa que cada tabela operacional isoladamente bloqueia o corte se tiver registro
        for op_tbl in OPERATIONAL_ZERO_STATE_TABLES:
            sec = self._make_valid_sections()
            metric_key = f"{op_tbl}_count" if not op_tbl.endswith("_count") else op_tbl
            if metric_key in sec["ZERO_STATE"]:
                sec["ZERO_STATE"][metric_key] = 1
                eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
                self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED", f"Falhou para tabela {op_tbl}")
                self.assertFalse(eval_res["gate_authorized"])
                self.assertEqual(eval_res["exit_code"], 1)
                self.assertTrue(any(metric_key in v for v in eval_res["violations"]))

    def test_block_when_balance_sum_not_zero(self):
        sec = self._make_valid_sections()
        sec["ZERO_STATE"]["users_balance_sum"] = "0.01"
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        self.assertTrue(any("users_balance_sum" in v for v in eval_res["violations"]))

    def test_block_when_migration_10_missing(self):
        sec = self._make_valid_sections()
        sec["MIGRATIONS"]["versions_applied"] = [1, 2, 3, 4, 5, 6, 7, 8, 9]  # Faltando versão 10
        sec["MIGRATIONS"]["max_version"] = 9
        sec["MIGRATIONS"]["migrations_count"] = 9
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        self.assertTrue(any("MIGRACOES_INCOERENTES" in v for v in eval_res["violations"]))

    def test_detect_risk_state_unlinked_payout_submissions(self):
        sec = self._make_valid_sections()
        sec["ZERO_STATE"]["payout_submissions_count"] = 1
        sec["ZERO_STATE"]["unlinked_payout_submissions_count"] = 1
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        # Verifica detecção de risco operacional sem expor PII ou dados sensíveis
        unlinked_viols = [v for v in eval_res["violations"] if "unlinked_payout_submissions_count" in v]
        self.assertTrue(len(unlinked_viols) > 0)
        self.assertIn("RISCO_OPERACIONAL", unlinked_viols[0])
        self.assertIn("Submissões de saque sem retorno do provedor vinculado", unlinked_viols[0])

    def test_detect_risk_state_pending_payout_events(self):
        sec = self._make_valid_sections()
        sec["ZERO_STATE"]["payout_event_queue_count"] = 2
        sec["ZERO_STATE"]["pending_payout_events_count"] = 2
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        # Verifica detecção de risco operacional sem expor PII ou dados sensíveis
        pending_viols = [v for v in eval_res["violations"] if "pending_payout_events_count" in v]
        self.assertTrue(len(pending_viols) > 0)
        self.assertIn("RISCO_OPERACIONAL", pending_viols[0])
        self.assertIn("Eventos de saque pendentes de reconciliação manual", pending_viols[0])

    def test_fail_closed_when_any_table_missing(self):
        sec = self._make_valid_sections()
        sec["TABLES"]["security_logs"] = False
        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        self.assertTrue(any("security_logs" in v and "METRICA_UNKNOWN" in v for v in eval_res["violations"]))

    def test_block_when_migration_10_and_tables_missing(self):
        """Simula saída de execute_sql_preflight_audit quando o banco está apenas na migração 9."""
        sec = self._make_valid_sections()
        sec["MIGRATIONS"]["versions_applied"] = [1, 2, 3, 4, 5, 6, 7, 8, 9]
        sec["MIGRATIONS"]["max_version"] = 9
        sec["MIGRATIONS"]["migrations_count"] = 9
        sec["TABLES"]["payout_submissions"] = False
        sec["TABLES"]["payout_event_queue"] = False
        sec["ZERO_STATE"]["payout_submissions_count"] = None
        sec["ZERO_STATE"]["unlinked_payout_submissions_count"] = None
        sec["ZERO_STATE"]["payout_event_queue_count"] = None
        sec["ZERO_STATE"]["pending_payout_events_count"] = None

        eval_res = evaluate_preflight_results(sec, is_proof_mode=False)
        self.assertEqual(eval_res["status"], "VIOLATIONS_DETECTED")
        self.assertFalse(eval_res["gate_authorized"])
        self.assertEqual(eval_res["exit_code"], 1)
        self.assertEqual(len(eval_res["fatal_errors"]), 0)

        violations_str = " ".join(eval_res["violations"])
        self.assertIn("MIGRACOES_INCOERENTES", violations_str)
        self.assertIn("payout_submissions", violations_str)
        self.assertIn("payout_event_queue", violations_str)


if __name__ == "__main__":
    unittest.main()
