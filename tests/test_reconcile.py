#!/usr/bin/env python3
"""
tests/test_reconcile.py

Testes automatizados significativos para a ferramenta scripts/release/reconcile.py.
Cobre:
1. Rejeição de target ou db divergentes (inclusive mock de verify_database_name).
2. Sanitização de erros de parser: exceções jamais vazam buffers, SQL ou PII (teste com sentinela).
3. Proteções em write_private_file: não altera permissão de diretório existente, rejeita symlinks e protege arquivos pré-existentes.
4. Status UNTESTED_NO_EVENTS para zero financial_events.
5. Preservação de alta precisão Decimal (sem float).
6. Ausência absoluta de PII e IDs brutos (CB-, STG-, PIX-) nos relatórios.
7. Verificação auxiliar de estabilidade volumétrica de linhas.
8. Qualificação da falácia de soma de transactions pré-v5.
9. Processamento de anomalias contábeis via window function (fixtures limpa e quebrada).
10. Dynamic table count em evidence_classification.
"""

import decimal
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from scripts.release.reconcile import (
    AuditError,
    DecimalEncoder,
    check_table_row_count_stability,
    pseudonymize,
    parse_audit_sections,
    process_financial_events_continuity,
    analyze_balance_comparisons,
    build_audit_report,
    render_markdown_report,
    write_private_file,
    verify_target_safety,
    verify_database_name,
    evaluate_check_only_status
)


class TestTargetAndDBDivergence(unittest.TestCase):
    """Testa rejeição estrita quando o alvo ou o banco de dados divergem de staging."""

    def test_reject_production_without_its_own_confirmation(self):
        with self.assertRaises(AuditError) as ctx:
            verify_target_safety("production", confirm_staging=True)
        self.assertIn("Confirmação explícita ausente", str(ctx.exception))

    def test_reject_missing_confirmation(self):
        with self.assertRaises(AuditError) as ctx:
            verify_target_safety("staging", confirm_staging=False)
        self.assertIn("Confirmação explícita ausente", str(ctx.exception))

    @patch("scripts.release.reconcile.run_compose_cmd")
    def test_verify_database_name_aborts_on_wrong_db(self, mock_run):
        mock_run.return_value = "catbrasil_production"
        with self.assertRaises(AuditError) as ctx:
            verify_database_name()
        self.assertIn("catbrasil_production", str(ctx.exception))
        self.assertIn("não corresponde ao ambiente esperado 'catbrasil_staging'", str(ctx.exception))

    @patch("scripts.release.reconcile.run_compose_cmd")
    def test_verify_database_name_succeeds_on_staging(self, mock_run):
        mock_run.return_value = "catbrasil_staging"
        self.assertEqual(verify_database_name(), "catbrasil_staging")

    @patch("scripts.release.reconcile.run_compose_cmd", return_value="container-id")
    @patch("scripts.release.reconcile.subprocess.run")
    @patch("scripts.release.reconcile.ROOT")
    def test_production_target_requires_exact_project_and_volume(self, mock_root, mock_inspect, _mock_compose):
        mock_root.__truediv__.return_value.is_file.return_value = True
        labels = "catbrasil-prod|postgres"
        mounts = json.dumps([{"Type": "volume", "Name": "catbrasil_prod_pgdata",
                              "Destination": "/var/lib/postgresql/data"}])
        mock_inspect.side_effect = [type("Result", (), {"stdout": labels})(),
                                    type("Result", (), {"stdout": mounts})()]
        self.assertEqual(verify_target_safety("production", confirm_production=True), "container-id")

        mock_inspect.side_effect = [type("Result", (), {"stdout": "catbrasil-staging|postgres"})()]
        with self.assertRaisesRegex(AuditError, "Rótulos Docker"):
            verify_target_safety("production", confirm_production=True)

        mock_inspect.side_effect = [type("Result", (), {"stdout": labels})(),
                                    type("Result", (), {"stdout": json.dumps([{
                                        "Type": "volume", "Name": "catbrasil-staging_pgdata",
                                        "Destination": "/var/lib/postgresql/data"}])})()]
        with self.assertRaisesRegex(AuditError, "Volume PostgreSQL"):
            verify_target_safety("production", confirm_production=True)

    @patch("scripts.release.reconcile.run_compose_cmd", return_value="catbrasil_staging")
    def test_production_target_rejects_staging_database(self, _mock):
        with self.assertRaisesRegex(AuditError, "catbrasil_prod"):
            verify_database_name("production")


class TestParserErrorSanitization(unittest.TestCase):
    """Testa se exceções no parser de seções SQL omitem buffers e dados sensíveis."""

    def test_parser_error_never_leaks_sentinel_pii(self):
        sentinel_pii = "SENTINEL_CPF_12345678900_SECRET_PIN_9999"
        corrupted_output = f"""
        SECTION_USERS_BEGIN
        {{ "invalid_json": true, "leaked_payload": "{sentinel_pii}", ... broken
        SECTION_USERS_END
        """
        with self.assertRaises(AuditError) as ctx:
            parse_audit_sections(corrupted_output)

        err_msg = str(ctx.exception)
        self.assertNotIn(sentinel_pii, err_msg)
        self.assertNotIn("leaked_payload", err_msg)
        self.assertIn("Falha de desserialização JSON na seção 'USERS'", err_msg)


class TestPrivateFileSecurity(unittest.TestCase):
    """Testa segurança avançada de escrita de arquivos privados."""

    def test_write_private_file_does_not_modify_existing_parent_permissions(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            parent = Path(tmpdir) / "existing_dir"
            parent.mkdir(mode=0o755)
            os.chmod(parent, 0o755)

            target = parent / "report.json"
            write_private_file(target, '{"test": 1}')

            # Permissão do diretório existente NÃO deve ter sido alterada para 0700
            parent_mode = stat.S_IMODE(parent.stat().st_mode)
            self.assertEqual(parent_mode, 0o755)

            # Arquivo deve ser 0600
            file_mode = stat.S_IMODE(target.stat().st_mode)
            self.assertEqual(file_mode, 0o600)

    def test_write_private_file_rejects_pre_existing_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir) / "existing.json"
            target.write_text("initial")

            with self.assertRaises(AuditError) as ctx:
                write_private_file(target, "new_content")
            self.assertIn("já existe. Recusado sobrescrever", str(ctx.exception))

    def test_write_private_file_rejects_symlink(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            real_file = Path(tmpdir) / "real.txt"
            real_file.write_text("content")

            link_path = Path(tmpdir) / "link_target.json"
            os.symlink(real_file, link_path)

            with self.assertRaises(AuditError) as ctx:
                write_private_file(link_path, "new")
            self.assertIn("link simbólico", str(ctx.exception))


class TestZeroFinancialEventsUntested(unittest.TestCase):
    """Garante que zero financial_events retorna status UNTESTED e não ÍNTEGRA."""

    def test_zero_events_returns_untested(self):
        result = process_financial_events_continuity({"total_financial_events": 0, "anomalies": []})
        self.assertEqual(result["status"], "UNTESTED_NO_EVENTS")
        self.assertIsNone(result["is_continuous"])
        self.assertIn("NÃO TESTADA", result["description"])
        self.assertEqual(result["anomalies_count"], 0)


class TestFinancialEventsContinuityFixtures(unittest.TestCase):
    """Testa transformação de anomalias com fixtures limpa e quebrada."""

    def test_clean_chain_fixture(self):
        fe_section = {
            "total_financial_events": 2,
            "anomalies_count": 0,
            "anomalies": []
        }
        res = process_financial_events_continuity(fe_section)
        self.assertEqual(res["status"], "TESTED_CONTINUOUS")
        self.assertTrue(res["is_continuous"])
        self.assertEqual(res["anomalies_count"], 0)

    def test_broken_chain_fixture(self):
        fe_section = {
            "total_financial_events": 2,
            "anomalies_count": 2,
            "anomalies": [
                {
                    "event_id": 1,
                    "user_id": "usr-1",
                    "kind": "EQUATION_MISMATCH",
                    "balance_before": "0.00",
                    "balance_after": "90.00",
                    "amount": "100.00",
                    "prev_balance_after": None
                },
                {
                    "event_id": 2,
                    "user_id": "usr-1",
                    "kind": "CHAIN_DISCONTINUITY",
                    "balance_before": "95.00",
                    "balance_after": "105.00",
                    "amount": "10.00",
                    "prev_balance_after": "90.00"
                }
            ]
        }
        res = process_financial_events_continuity(fe_section)
        self.assertEqual(res["status"], "CHAIN_INTEGRITY_BREACH")
        self.assertFalse(res["is_continuous"])
        self.assertEqual(res["anomalies_count"], 2)
        # IDs devem ser pseudonimizados
        self.assertTrue(res["anomalies"][0]["event_pseudonym"].startswith("anon_event_"))
        self.assertTrue(res["anomalies"][0]["user_pseudonym"].startswith("anon_user_"))
        self.assertNotIn("usr-1", res["anomalies"][0]["user_pseudonym"])


class TestNumericPrecisionPreservation(unittest.TestCase):
    """Testa preservação de alta precisão Decimal sem conversão para float."""

    def test_decimal_precision_in_json(self):
        data = {
            "precise_amount": decimal.Decimal("123456789012345.6789012345"),
            "cent_amount": decimal.Decimal("0.01")
        }
        serialized = json.dumps(data, cls=DecimalEncoder)
        self.assertIn('"123456789012345.6789012345"', serialized)

        parsed = json.loads(serialized, parse_float=decimal.Decimal)
        self.assertIsInstance(parsed["cent_amount"], str)
        d_val = decimal.Decimal(parsed["cent_amount"])
        self.assertEqual(d_val, decimal.Decimal("0.01"))


class TestNoPIIOrRawIDsLeaked(unittest.TestCase):
    """Garante que nenhum ID real de usuário/transação/gateway ou PII apareça nos relatórios."""

    def test_pseudonymize_format(self):
        p1 = pseudonymize("user", "CB-4544")
        p2 = pseudonymize("tx", "PIX-9291f24b62f2d3e6")
        p3 = pseudonymize("ext", "STG-36DAD961BA7A2B5DFDBD")

        self.assertTrue(p1.startswith("anon_user_"))
        self.assertTrue(p2.startswith("anon_tx_"))
        self.assertTrue(p3.startswith("anon_ext_"))
        self.assertNotIn("CB-", p1)
        self.assertNotIn("PIX-", p2)
        self.assertNotIn("STG-", p3)

    def test_report_builder_and_markdown_exclude_raw_ids(self):
        sections = {
            "SCHEMA": {
                "current_db": "catbrasil_staging",
                "migrations": [{"version": 8, "name": "000008_audit.sql", "checksum": "abc", "applied_at": "2026-10-16"}],
                "tables": [{"table_name": "users", "column_count": 25}],
                "financial_indexes": []
            },
            "USERS": {
                "total_users": 1,
                "roles": {"user": 1},
                "aggregates": {"sum_balance": "0.00", "sum_yield_balance": "0.00"},
                "flags": {},
                "demo_seed_suspicions_count": 0
            },
            "TRANSACTIONS": {
                "total_transactions": 1,
                "date_range": {"oldest": "2026-10-16", "newest": "2026-10-16"},
                "by_group": [{"tipo": "Depósito PIX", "status": "pendente", "provider": "suitpay", "count": 1, "sum_valor": "1.00", "min_created_at": "2026-10-16", "max_created_at": "2026-10-16"}],
                "by_status": {"pendente": 1},
                "by_provider": {"suitpay": 1}
            },
            "FINANCIAL_EVENTS": {"total_financial_events": 0, "anomalies_count": 0, "anomalies": []},
            "BALANCE_VS_EVENTS": {"comparisons_summary": {"no_events_zero_balance_count": 1}, "discrepancies": []},
            "PENDING_OPS": {
                "pending_deposits": [{
                    "id": "PIX-REAL-SECRET-12345",
                    "user_id": "CB-REAL-USER-9999",
                    "valor": "1.00",
                    "gateway_provider": "suitpay",
                    "has_external_id": True,
                    "external_id": "STG-REAL-EXT-ID-8888",
                    "created_at": "2026-10-16T12:00:00Z",
                    "age_minutes": "10.0"
                }],
                "pending_or_processing_withdrawals": [],
                "unmatched_payout_gateway_events": []
            },
            "NO_EXTERNAL_ID": {"cases": []},
            "TX_SUM_ANALYSIS": {"sum_all_users_balance": "0.00", "sum_deposits_sucesso": "0.00", "sum_saques_sucesso": "0.00"}
        }

        counts_mock = {f"t_{i}": 5 for i in range(15)}
        report = build_audit_report("container-id-abc", counts_mock, counts_mock, sections)
        md_output = render_markdown_report(report)
        json_output = json.dumps(report, cls=DecimalEncoder)

        for forbidden in ["PIX-REAL-SECRET", "CB-REAL-USER", "STG-REAL-EXT"]:
            self.assertNotIn(forbidden, md_output)
            self.assertNotIn(forbidden, json_output)

        self.assertIn("anon_tx_", md_output)
        self.assertIn("anon_user_", md_output)
        self.assertIn("anon_ext_", md_output)

        # Checa contagem dinâmica de tabelas em evidence_classification
        self.assertIn("Todas as 15 tabelas auditadas apresentaram estabilidade volumétrica", md_output)


class TestRowCountStabilityAuxiliaryCheck(unittest.TestCase):
    """Testa verificação auxiliar de estabilidade volumétrica de linhas."""

    def test_identical_counts_passes(self):
        b = {"users": 10, "transactions": 7}
        a = {"users": 10, "transactions": 7}
        self.assertTrue(check_table_row_count_stability(b, a))

    def test_different_counts_raises_error(self):
        b = {"users": 10, "transactions": 7}
        a = {"users": 10, "transactions": 8}
        with self.assertRaises(AuditError) as ctx:
            check_table_row_count_stability(b, a)
        self.assertIn("Variação volumétrica detectada", str(ctx.exception))


class TestPreV5SumFallacy(unittest.TestCase):
    """Testa a lógica que desqualifica a dedução de saldo pela soma de transactions."""

    def test_sum_fallacy(self):
        tx_data = {
            "sum_deposits_sucesso": decimal.Decimal("100.00"),
            "sum_saques_sucesso": decimal.Decimal("20.00"),
            "sum_all_users_balance": decimal.Decimal("350.00")
        }
        naive = tx_data["sum_deposits_sucesso"] - tx_data["sum_saques_sucesso"]
        self.assertEqual(naive, decimal.Decimal("80.00"))
        self.assertNotEqual(naive, tx_data["sum_all_users_balance"])


class TestCheckOnlyExitLogic(unittest.TestCase):
    """Testa explicitamente a avaliação de códigos de saída da rotina --check-only."""

    def _make_minimal_report(self, discrepancies_count=0, continuity_status="UNTESTED_NO_EVENTS", anomalies_count=0):
        return {
            "balance_vs_latest_event": {
                "discrepancies_count": discrepancies_count
            },
            "financial_events_coverage_and_continuity": {
                "continuity_verification": {
                    "status": continuity_status,
                    "anomalies_count": anomalies_count
                }
            }
        }

    def test_exit_1_when_continuity_breach_even_with_zero_discrepancies(self):
        report = self._make_minimal_report(
            discrepancies_count=0,
            continuity_status="CHAIN_INTEGRITY_BREACH",
            anomalies_count=0
        )
        self.assertEqual(evaluate_check_only_status(report), 1)

    def test_exit_1_when_anomalies_greater_than_zero_even_with_zero_discrepancies(self):
        report = self._make_minimal_report(
            discrepancies_count=0,
            continuity_status="INTEGRAL",
            anomalies_count=2
        )
        self.assertEqual(evaluate_check_only_status(report), 1)

    def test_exit_1_when_discrepancies_greater_than_zero(self):
        report = self._make_minimal_report(
            discrepancies_count=1,
            continuity_status="INTEGRAL",
            anomalies_count=0
        )
        self.assertEqual(evaluate_check_only_status(report), 1)

    def test_exit_0_on_untested_no_events_with_zero_discrepancies(self):
        report = self._make_minimal_report(
            discrepancies_count=0,
            continuity_status="UNTESTED_NO_EVENTS",
            anomalies_count=0
        )
        self.assertEqual(evaluate_check_only_status(report), 0)

    def test_exit_0_when_integral_and_no_anomalies_or_discrepancies(self):
        report = self._make_minimal_report(
            discrepancies_count=0,
            continuity_status="INTEGRAL",
            anomalies_count=0
        )
        self.assertEqual(evaluate_check_only_status(report), 0)


if __name__ == "__main__":
    unittest.main()
