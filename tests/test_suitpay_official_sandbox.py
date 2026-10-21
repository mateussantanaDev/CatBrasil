#!/usr/bin/env python3
"""Testes de scripts/release/suitpay_official_sandbox.py (preflight + evidence).

Cobre: preflight sem URL, com stub e live=true; evidence com manifesto ausente,
inconsistente, consistente e duplicado; não-vazamento de sentinela. Sem mocks de
eventos SuitPay: apenas fixtures locais estáticas.
"""

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "release" / "suitpay_official_sandbox.py"
SENTINEL = "SENTINEL_NAO_VAZAR_9f3a7c"
VALID_ENV = (
    "APP_ENV=staging\nPIX_MODE=suitpay\nPIX_SANDBOX_URL=\n"
    "SUITPAY_LIVE_ENABLED=false\nAPI_DOMAIN=api.sandbox.example.com\n"
    "SUITPAY_CALLBACK_URL=https://api.sandbox.example.com/api/v1/webhooks/suitpay\n"
)


def run_cli(*args):
    return subprocess.run(
        [sys.executable, str(SCRIPT), *args],
        capture_output=True, text=True, timeout=30,
    )


def build_manifest():
    cashin_req = f"req-cashin-{SENTINEL}"
    cashin_ptx = f"ptx-cashin-{SENTINEL}"
    # name, amount, ledger_delta, local, provider_status, provider_type,
    # delivery, provider_tx_id, gross_reserved_cents
    specs = [
        ("cashin_paid", 1500, 1500, "sucesso", "PAID_OUT", "PIX",
         "callback", cashin_ptx, None),
        ("cashin_duplicate", 1500, 0, "sucesso", "PAID_OUT", "PIX",
         "callback", cashin_ptx, None),
        ("cashin_chargeback", 1500, -1500, "cancelado", "CHARGEBACK", "PIX",
         "callback", cashin_ptx, None),
        ("cashout_paid", 2000, 0, "sucesso", "PAID_OUT", "PIX_CASHOUT",
         "callback", f"ptx-cashout-paid-{SENTINEL}", None),
        ("cashout_canceled", 2000, 2100, "cancelado", "CANCELED", "PIX_CASHOUT",
         "callback", f"ptx-cashout-canceled-{SENTINEL}", 2100),
        ("cashout_document_mismatch", 2000, 0, "processando", "DOCUMENT_VALIDATE",
         "PIX_CASHOUT", "sync_response", None, None),
    ]
    scenarios = []
    for i, (name, amount, delta, local, pstatus, ptype, delivery, ptx, gross) in enumerate(specs):
        entry = {
            "scenario": name,
            "provider_ref": f"pref-{i}-{SENTINEL}",
            "request_id": cashin_req if name.startswith("cashin") else f"req-{i}-{SENTINEL}",
            "amount_cents": amount,
            "provider_status": pstatus,
            "provider_type": ptype,
            "delivery": delivery,
            "local_status": local,
            "ledger_delta_cents": delta,
            "evidence_ref": f"stmt-{i}-{SENTINEL}",
        }
        if ptx is not None:
            entry["provider_tx_id"] = ptx
        if gross is not None:
            entry["gross_reserved_cents"] = gross
        scenarios.append(entry)
    return {
        "provider": "suitpay-official-sandbox",
        "run_id": "local-run-1",
        "scenarios": scenarios,
    }


def by_scenario(data):
    return {entry["scenario"]: entry for entry in data["scenarios"]}


class PreflightTests(unittest.TestCase):
    def _write_env(self, tmp, text):
        path = Path(tmp) / "env"
        path.write_text(text, encoding="utf-8")
        return str(path)

    def test_ready_when_static_config_ok(self):
        with tempfile.TemporaryDirectory() as tmp:
            proc = run_cli("preflight", "--env-file", self._write_env(tmp, VALID_ENV))
        out = json.loads(proc.stdout)
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(out, {"status": "ready", "errors": []})

    def test_blocked_without_callback_and_api_domain(self):
        text = "APP_ENV=staging\nPIX_MODE=suitpay\nPIX_SANDBOX_URL=\nSUITPAY_LIVE_ENABLED=false\n"
        with tempfile.TemporaryDirectory() as tmp:
            proc = run_cli("preflight", "--env-file", self._write_env(tmp, text))
        out = json.loads(proc.stdout)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_API_DOMAIN_MISSING", out["errors"])
        self.assertIn("E_CALLBACK_MISSING", out["errors"])

    def test_blocked_when_stub_url_present(self):
        text = VALID_ENV.replace("PIX_SANDBOX_URL=\n", "PIX_SANDBOX_URL=http://stub.local\n")
        with tempfile.TemporaryDirectory() as tmp:
            proc = run_cli("preflight", "--env-file", self._write_env(tmp, text))
        out = json.loads(proc.stdout)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_PIX_SANDBOX_URL_NOT_EMPTY", out["errors"])

    def test_blocked_when_live_enabled_true(self):
        text = VALID_ENV.replace("SUITPAY_LIVE_ENABLED=false", "SUITPAY_LIVE_ENABLED=true")
        with tempfile.TemporaryDirectory() as tmp:
            proc = run_cli("preflight", "--env-file", self._write_env(tmp, text))
        out = json.loads(proc.stdout)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_LIVE_ENABLED_NOT_FALSE", out["errors"])

    def test_sentinel_not_leaked_on_blocked_preflight(self):
        text = VALID_ENV.replace("PIX_SANDBOX_URL=\n", f"PIX_SANDBOX_URL={SENTINEL}\n")
        with tempfile.TemporaryDirectory() as tmp:
            proc = run_cli("preflight", "--env-file", self._write_env(tmp, text))
        self.assertEqual(proc.returncode, 2)
        self.assertNotIn(SENTINEL, proc.stdout + proc.stderr)


class EvidenceTests(unittest.TestCase):
    def _run(self, data=None, missing=False):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "manifest.json"
            if not missing:
                path.write_text(
                    json.dumps(data if data is not None else build_manifest()),
                    encoding="utf-8",
                )
            proc = run_cli("evidence", "--manifest", str(path))
        return proc, json.loads(proc.stdout)

    def test_missing_manifest_file(self):
        proc, out = self._run(missing=True)
        self.assertEqual(proc.returncode, 2)
        self.assertEqual(out, {"status": "incomplete", "errors": ["E_MANIFEST_UNREADABLE"]})

    def test_consistent_manifest(self):
        proc, out = self._run()
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(out, {"status": "consistent", "errors": []})

    def test_inconsistent_missing_scenario(self):
        data = build_manifest()
        data["scenarios"] = [
            s for s in data["scenarios"] if s["scenario"] != "cashin_paid"
        ]
        proc, out = self._run(data)
        self.assertEqual(proc.returncode, 2)
        self.assertEqual(out["status"], "incomplete")
        self.assertIn("E_SCENARIO_MISSING", out["errors"])

    def test_inconsistent_wrong_ledger_delta(self):
        data = build_manifest()
        data["scenarios"][0]["ledger_delta_cents"] = 0
        proc, out = self._run(data)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_LEDGER_DELTA_MISMATCH", out["errors"])

    def test_inconsistent_duplicate_scenario(self):
        data = build_manifest()
        dup = dict(data["scenarios"][0])
        for field, prefix in (
            ("provider_ref", "pref-dup-"), ("request_id", "req-dup-"),
            ("provider_tx_id", "ptx-dup-"), ("evidence_ref", "stmt-dup-"),
        ):
            dup[field] = prefix + SENTINEL
        data["scenarios"].append(dup)
        proc, out = self._run(data)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_SCENARIO_DUPLICATED", out["errors"])

    def test_sentinel_not_leaked_on_consistent_evidence(self):
        proc, out = self._run()
        self.assertEqual(out["status"], "consistent")
        self.assertNotIn(SENTINEL, proc.stdout + proc.stderr)

    def test_exact_local_statuses(self):
        data = by_scenario(build_manifest())
        self.assertEqual(data["cashin_paid"]["local_status"], "sucesso")
        self.assertEqual(data["cashin_duplicate"]["local_status"], "sucesso")
        self.assertEqual(data["cashin_chargeback"]["local_status"], "cancelado")
        self.assertEqual(data["cashout_paid"]["local_status"], "sucesso")
        self.assertEqual(data["cashout_canceled"]["local_status"], "cancelado")
        self.assertEqual(data["cashout_document_mismatch"]["local_status"], "processando")

    def test_exact_provider_statuses(self):
        data = by_scenario(build_manifest())
        self.assertEqual(data["cashin_paid"]["provider_status"], "PAID_OUT")
        self.assertEqual(data["cashin_duplicate"]["provider_status"], "PAID_OUT")
        self.assertEqual(data["cashin_chargeback"]["provider_status"], "CHARGEBACK")
        self.assertEqual(data["cashout_paid"]["provider_status"], "PAID_OUT")
        self.assertEqual(data["cashout_canceled"]["provider_status"], "CANCELED")
        self.assertEqual(data["cashout_document_mismatch"]["provider_status"], "DOCUMENT_VALIDATE")

    def test_valid_linked_cashin_flow(self):
        data = by_scenario(build_manifest())
        self.assertEqual(data["cashin_paid"]["request_id"], data["cashin_duplicate"]["request_id"])
        self.assertEqual(data["cashin_paid"]["request_id"], data["cashin_chargeback"]["request_id"])
        self.assertEqual(data["cashin_paid"]["provider_tx_id"], data["cashin_chargeback"]["provider_tx_id"])
        proc, out = self._run()
        self.assertEqual(out, {"status": "consistent", "errors": []})

    def test_mismatched_provider_tx_linkage(self):
        data = build_manifest()
        by_scenario(data)["cashin_chargeback"]["provider_tx_id"] = "ptx-other-" + SENTINEL
        proc, out = self._run(data)
        self.assertEqual(proc.returncode, 2)
        self.assertIn("E_CASHIN_LINKAGE_MISMATCH", out["errors"])

    def test_mismatched_request_linkage(self):
        data = build_manifest()
        by_scenario(data)["cashin_duplicate"]["request_id"] = "req-other-" + SENTINEL
        proc, out = self._run(data)
        self.assertIn("E_CASHIN_LINKAGE_MISMATCH", out["errors"])

    def test_wrong_provider_status(self):
        data = build_manifest()
        by_scenario(data)["cashin_paid"]["provider_status"] = "PAID"
        proc, out = self._run(data)
        self.assertIn("E_PROVIDER_STATUS_UNEXPECTED", out["errors"])

    def test_chargeback_with_paid_out_status(self):
        data = build_manifest()
        by_scenario(data)["cashin_chargeback"]["provider_status"] = "PAID_OUT"
        proc, out = self._run(data)
        self.assertIn("E_PROVIDER_STATUS_UNEXPECTED", out["errors"])

    def test_document_mismatch_without_provider_tx_id_valid(self):
        data = build_manifest()
        self.assertNotIn("provider_tx_id", by_scenario(data)["cashout_document_mismatch"])
        proc, out = self._run(data)
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(out, {"status": "consistent", "errors": []})
        self.assertNotIn("E_PROVIDER_TX_ID_EMPTY", out["errors"])

    def test_cashin_requires_provider_tx_id(self):
        data = build_manifest()
        del by_scenario(data)["cashin_paid"]["provider_tx_id"]
        proc, out = self._run(data)
        self.assertIn("E_PROVIDER_TX_ID_EMPTY", out["errors"])

    def test_document_mismatch_is_sync_not_callback(self):
        data = build_manifest()
        by_scenario(data)["cashout_document_mismatch"]["delivery"] = "callback"
        proc, out = self._run(data)
        self.assertIn("E_DELIVERY_UNEXPECTED", out["errors"])

    def test_document_mismatch_no_automatic_refund(self):
        data = build_manifest()
        by_scenario(data)["cashout_document_mismatch"]["ledger_delta_cents"] = 2000
        proc, out = self._run(data)
        self.assertIn("E_LEDGER_DELTA_MISMATCH", out["errors"])

    def test_document_mismatch_local_status_processando(self):
        data = build_manifest()
        by_scenario(data)["cashout_document_mismatch"]["local_status"] = "cancelado"
        proc, out = self._run(data)
        self.assertIn("E_LOCAL_STATUS_UNEXPECTED", out["errors"])

    def test_payout_canceled_gross_greater_than_net(self):
        data = build_manifest()
        canceled = by_scenario(data)["cashout_canceled"]
        self.assertGreater(canceled["gross_reserved_cents"], canceled["amount_cents"])
        self.assertEqual(canceled["ledger_delta_cents"], canceled["gross_reserved_cents"])
        proc, out = self._run(data)
        self.assertEqual(out, {"status": "consistent", "errors": []})

    def test_payout_canceled_requires_gross_reserved(self):
        data = build_manifest()
        del by_scenario(data)["cashout_canceled"]["gross_reserved_cents"]
        proc, out = self._run(data)
        self.assertIn("E_GROSS_RESERVED_CENTS_INVALID", out["errors"])

    def test_payout_canceled_ledger_must_equal_gross(self):
        data = build_manifest()
        by_scenario(data)["cashout_canceled"]["ledger_delta_cents"] = 2000
        proc, out = self._run(data)
        self.assertIn("E_LEDGER_DELTA_MISMATCH", out["errors"])

    def test_payout_canceled_gross_below_net(self):
        data = build_manifest()
        canceled = by_scenario(data)["cashout_canceled"]
        canceled["gross_reserved_cents"] = 1900
        canceled["ledger_delta_cents"] = 1900
        proc, out = self._run(data)
        self.assertIn("E_GROSS_BELOW_NET", out["errors"])

    def test_payout_paid_ledger_zero_at_callback(self):
        data = build_manifest()
        paid = by_scenario(data)["cashout_paid"]
        self.assertEqual(paid["ledger_delta_cents"], 0)
        proc, out = self._run(data)
        self.assertEqual(out, {"status": "consistent", "errors": []})

    def test_duplicate_evidence_ref(self):
        data = build_manifest()
        by_scenario(data)["cashin_chargeback"]["evidence_ref"] = \
            by_scenario(data)["cashin_paid"]["evidence_ref"]
        proc, out = self._run(data)
        self.assertIn("E_DUPLICATE_EVIDENCE_REF", out["errors"])


if __name__ == "__main__":
    unittest.main()
