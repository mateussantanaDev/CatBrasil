"""Contract tests for the operator-normalized offline statement comparator."""

import csv
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SOURCE = Path(__file__).resolve().parents[1] / "scripts/release/provider_reconcile.py"
SPEC = importlib.util.spec_from_file_location("provider_reconcile", SOURCE)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class ProviderReconcileTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.internal = self.root / "internal.csv"
        self.provider = self.root / "provider.json"
        self.report = self.root / "report.json"

    def write_internal(self, rows):
        with self.internal.open("w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=MODULE.FIELDS["internal"])
            writer.writeheader()
            writer.writerows(rows)

    def write_provider(self, rows):
        self.provider.write_text(json.dumps(rows), encoding="utf-8")

    def run_tool(self):
        return MODULE.main(["--internal", str(self.internal), "--provider", str(self.provider),
                            "--scope", "synthetic fixture; complete snapshot", "--report", str(self.report)])

    def fixture(self):
        self.write_internal([
            {"transaction_id": "tx-dep", "direction": "deposit", "external_id": "ext-dep", "amount": "10.01", "status": "settled"},
            {"transaction_id": "tx-out", "direction": "payout", "external_id": "ext-out", "amount": "-2.30", "status": "pending"},
        ])
        self.write_provider([
            {"external_id": "ext-dep", "direction": "deposit", "amount": "10.01", "status": "settled", "internal_ref": "tx-dep"},
            {"external_id": "ext-out", "direction": "payout", "amount": "2.30", "status": "pending", "internal_ref": "tx-out"},
        ])

    def test_exact_match_and_private_report(self):
        self.fixture()
        self.assertEqual(self.run_tool(), 0)
        report = json.loads(self.report.read_text())
        self.assertEqual(report["result"], "match")
        self.assertEqual(report["issues"], [])
        self.assertEqual(self.report.stat().st_mode & 0o777, 0o600)
        self.assertEqual(len(report["inputs"]["provider_sha256"]), 64)

    def test_missing_extra_amount_status_and_reference_fail(self):
        self.fixture()
        rows = json.loads(self.provider.read_text())
        rows[0]["amount"] = "10.02"
        rows[0]["status"] = "reversed"
        rows[0]["internal_ref"] = "wrong"
        rows.pop()
        rows.append({"external_id": "unknown", "direction": "payout", "amount": "1.00", "status": "settled", "internal_ref": ""})
        self.write_provider(rows)
        self.assertEqual(self.run_tool(), 1)
        kinds = {issue["issue"] for issue in json.loads(self.report.read_text())["issues"]}
        self.assertEqual(kinds, {"amount_cents_mismatch", "status_mismatch", "internal_ref_mismatch", "missing_provider", "missing_internal"})

    def test_float_json_rejected_and_no_success_report(self):
        self.fixture()
        rows = json.loads(self.provider.read_text())
        rows[0]["amount"] = 10.01
        self.write_provider(rows)
        self.assertEqual(self.run_tool(), 2)
        self.assertFalse(self.report.exists())

    def test_duplicate_external_and_transaction_rejected(self):
        self.fixture()
        rows = json.loads(self.provider.read_text())
        rows.append(rows[0])
        self.write_provider(rows)
        self.assertEqual(self.run_tool(), 2)
        self.assertFalse(self.report.exists())

    def test_zero_rows_fail_closed(self):
        self.write_internal([])
        self.write_provider([])
        self.assertEqual(self.run_tool(), 1)
        self.assertEqual(json.loads(self.report.read_text())["issues"], [{"issue": "empty_scope"}])

    def test_no_rounding_and_signed_payout(self):
        self.fixture()
        self.write_provider([{"external_id": "ext-dep", "direction": "deposit", "amount": "10.001", "status": "settled", "internal_ref": "tx-dep"}])
        self.assertEqual(self.run_tool(), 2)
        self.assertFalse(self.report.exists())
        self.fixture()
        self.write_internal([{"transaction_id": "tx-out", "direction": "payout", "external_id": "ext-out", "amount": "2.30", "status": "pending"}])
        self.assertEqual(self.run_tool(), 2)

    def test_duplicate_json_key_rejected(self):
        self.fixture()
        self.provider.write_text('[{"external_id":"x","external_id":"y"}]')
        self.assertEqual(self.run_tool(), 2)
        self.assertFalse(self.report.exists())

    def test_numeric_15_2_boundary(self):
        self.write_internal([{"transaction_id": "max", "direction": "payout", "external_id": "ext-max", "amount": "-9999999999999.99", "status": "settled"}])
        self.write_provider([{"external_id": "ext-max", "direction": "payout", "amount": "9999999999999.99", "status": "settled", "internal_ref": "max"}])
        self.assertEqual(self.run_tool(), 0)
        self.report.unlink()
        self.write_provider([{"external_id": "ext-max", "direction": "payout", "amount": "10000000000000.00", "status": "settled", "internal_ref": "max"}])
        self.assertEqual(self.run_tool(), 2)
        self.assertFalse(self.report.exists())


if __name__ == "__main__":
    unittest.main()
