import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest
from unittest.mock import patch


SOURCE = Path(__file__).resolve().parents[1] / "scripts/release/daily-reconcile.py"
SPEC = importlib.util.spec_from_file_location("daily_reconcile", SOURCE)
daily = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(daily)


class DailyReconcileTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.reports = root / "reports"
        self.metrics = root / "metrics.prom"
        self.env = patch.dict(os.environ, {
            "RECONCILE_REPORT_DIR": str(self.reports),
            "RECONCILE_STATUS_FILE": str(self.metrics),
            "RECONCILE_RETENTION_DAYS": "30",
        })
        self.env.start()
        self.addCleanup(self.env.stop)

    @staticmethod
    def audit_success(command, **kwargs):
        json_path = Path(command[command.index("--output-json") + 1])
        md_path = Path(command[command.index("--output-md") + 1])
        json_path.write_text(json.dumps({
            "audit_metadata": {"target": "production", "database_name": "catbrasil_prod"},
            "balance_vs_latest_event": {"discrepancies_count": 0},
            "financial_events_coverage_and_continuity": {
                "continuity_verification": {"anomalies_count": 0}},
        }))
        md_path.write_text("private report")
        return type("Result", (), {"returncode": 0})()

    @patch.object(daily.subprocess, "run", side_effect=audit_success)
    def test_success_idempotent_and_private(self, process):
        self.assertEqual(daily.run(), 0)
        self.assertEqual(daily.run(), 0)
        self.assertEqual(process.call_count, 1)
        self.assertEqual(stat.S_IMODE(self.reports.stat().st_mode), 0o700)
        for path in self.reports.iterdir():
            if path.is_file():
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(self.metrics.stat().st_mode), 0o600)
        self.assertIn("catbrasil_internal_reconcile_last_run_success 1", self.metrics.read_text())

    @patch.object(daily.subprocess, "run")
    def test_divergence_fails_and_retries(self, process):
        process.return_value = type("Result", (), {"returncode": 1})()
        with self.assertRaises(RuntimeError):
            daily.run()
        with self.assertRaises(RuntimeError):
            daily.run()
        self.assertEqual(process.call_count, 2)
        self.assertFalse(list(self.reports.glob("successful-*.json")))
        self.assertIn("catbrasil_internal_reconcile_last_run_success 0", self.metrics.read_text())

    @patch.object(daily.subprocess, "run")
    def test_inconsistent_success_output_fails_closed(self, process):
        def forged_success(command, **kwargs):
            self.audit_success(command, **kwargs)
            report = Path(command[command.index("--output-json") + 1])
            data = json.loads(report.read_text())
            data["balance_vs_latest_event"]["discrepancies_count"] = 1
            report.write_text(json.dumps(data))
            return type("Result", (), {"returncode": 0})()

        process.side_effect = forged_success
        with self.assertRaisesRegex(RuntimeError, "inválido ou divergente"):
            daily.run()
        self.assertFalse(list(self.reports.glob("successful-*.json")))
        self.assertIn("catbrasil_internal_reconcile_last_run_success 0", self.metrics.read_text())

    def test_retention_preserves_unrelated_files(self):
        self.reports.mkdir(mode=0o700)
        old = self.reports / "internal-prod-20200101T000000Z-deadbeef.json"
        marker = self.reports / "successful-20200101.json"
        unrelated = self.reports / "keep.txt"
        for path in (old, marker, unrelated):
            path.write_text("x")
        daily.retain_reports(self.reports, 30, dt.datetime(2026, 10, 20, tzinfo=dt.timezone.utc))
        self.assertFalse(old.exists())
        self.assertFalse(marker.exists())
        self.assertTrue(unrelated.exists())


if __name__ == "__main__":
    unittest.main()
