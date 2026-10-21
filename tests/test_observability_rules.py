"""Static checks for the local Prometheus alert rules and Compose wiring.

These checks deliberately use the standard library, so they run on a host
without promtool or a Python YAML package. Prometheus must still parse the
rules in staging before a release is approved.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]
RULES = ROOT / "docker/prometheus/alerts.yml"


class ObservabilityRulesTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.rules = RULES.read_text()
        cls.config = (ROOT / "docker/prometheus/prometheus.yml").read_text()
        cls.compose = (ROOT / "docker-compose.yml").read_text()
        cls.metrics = (ROOT / "backend/internal/delivery/http/metrics.go").read_text()

    def rule(self, name):
        match = re.search(
            rf"(?ms)^\s+- alert: {re.escape(name)}\s*\n(?P<body>.*?)(?=^\s+- alert:|\Z)",
            self.rules,
        )
        self.assertIsNotNone(match, f"missing alert {name}")
        body = match.group("body")
        expr = re.search(r"(?m)^\s+expr: (.+)$", body)
        duration = re.search(r"(?m)^\s+for: (\S+)$", body)
        self.assertIsNotNone(expr, f"missing expression for {name}")
        self.assertIsNotNone(duration, f"missing sustained duration for {name}")
        self.assertGreater(int(duration.group(1).removesuffix("m")), 0)
        return expr.group(1)

    def test_rule_file_is_loaded_read_only_by_prometheus(self):
        self.assertIn('"/etc/prometheus/alerts.yml"', self.config)
        self.assertIn(
            "./docker/prometheus/alerts.yml:/etc/prometheus/alerts.yml:ro",
            self.compose,
        )
        self.assertTrue(RULES.is_file())

    def test_down_alert_uses_scrape_signal_for_unreachable_api(self):
        expr = self.rule("CatBrasilAPIDown")
        self.assertIn('up{job="catbrasil-api"} == 0', expr)
        self.assertIn('absent(up{job="catbrasil-api"})', expr)
        self.assertIn('catbrasil_up{job="catbrasil-api"} == 0', expr)
        self.assertIn("catbrasil_up 1", self.metrics)
        self.assertIn("job_name: catbrasil-api", self.config)

    def test_error_and_latency_rules_use_exported_series(self):
        errors = self.rule("CatBrasilSustained5xx")
        latency = self.rule("CatBrasilHighP95Latency")
        self.assertIn('status=~"5.."', errors)
        self.assertIn("catbrasil_http_requests_total", errors)
        self.assertIn("[5m]", errors)
        self.assertIn("histogram_quantile(0.95", latency)
        self.assertIn("catbrasil_http_request_duration_seconds_bucket", latency)
        self.assertIn("by (le)", latency)
        for series in (
            "catbrasil_http_requests_total",
            "catbrasil_http_request_duration_seconds_bucket",
        ):
            self.assertIn(series, self.metrics)

    def test_host_and_recovery_collectors_are_distinct_and_private(self):
        self.assertIn("job_name: catbrasil-host", self.config)
        self.assertIn('targets: ["node-exporter:9100"]', self.config)
        self.assertIn("job_name: catbrasil-recovery", self.config)
        self.assertIn('/tmp/recovery-targets.yml', self.config)
        exporter = re.search(r"(?ms)^  node-exporter:\n(?P<body>.*?)(?=^  [\w-]+:|^volumes:)", self.compose)
        self.assertIsNotNone(exporter)
        body = exporter.group("body")
        self.assertIn("/var/lib/node_exporter/textfile_collector", body)
        self.assertIn("create_host_path: false", body)
        self.assertIn("read_only: true", body)
        self.assertNotIn("ports:", body)

    def test_operational_alerts_cover_failure_absence_and_staleness(self):
        for name, series, job, seconds in (
            ("Backup", "catbrasil_backup", "catbrasil-host", 4200),
            ("RestoreProof", "catbrasil_restore_proof", "catbrasil-recovery", 691200),
            ("InternalReconcile", "catbrasil_internal_reconcile", "catbrasil-host", 90000),
        ):
            failed = self.rule(f"CatBrasil{name}Failed")
            stale = self.rule(f"CatBrasil{name}MissingOrStale")
            self.assertIn(f'{series}_last_run_success{{job="{job}"}} == 0', failed)
            self.assertIn(f'absent({series}_last_success_timestamp_seconds{{job="{job}"}})', stale)
            self.assertIn(f" > {seconds}", stale)
        self.assertIn('absent(up{job="catbrasil-recovery"})', self.rule("CatBrasilRecoveryMetricsUnavailable"))
        self.assertIn('absent(up{job="catbrasil-host"})', self.rule("CatBrasilHostTextfileUnavailable"))


if __name__ == "__main__":
    unittest.main()
