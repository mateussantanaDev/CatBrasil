"""Operational backup tests without touching a live database or remote host."""

import datetime as dt
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[1] / "scripts/release/backup-automation.py"
SPEC = importlib.util.spec_from_file_location("backup_automation", SCRIPT)
backup = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(backup)


class BackupAutomationTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.mount = self.root / "remote"
        self.mount.mkdir()
        self.status = self.root / "backup.prom"

    def test_missing_or_local_mount_blocks_before_backup(self):
        with mock.patch.dict(os.environ, {"BACKUP_REMOTE_MOUNT": str(self.mount)}):
            with mock.patch.object(backup.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "", "")):
                with self.assertRaisesRegex(ValueError, "mount remoto"):
                    backup.remote_directory()
            with mock.patch.object(backup.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "ext4\n", "")):
                with self.assertRaisesRegex(ValueError, "mount remoto"):
                    backup.remote_directory()
        self.assertFalse((self.mount / "catbrasil-prod").exists())

    def test_remote_mount_allows_only_explicit_network_filesystems(self):
        with mock.patch.dict(os.environ, {"BACKUP_REMOTE_MOUNT": str(self.mount)}):
            with mock.patch.object(backup.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, "nfs4\n", "")):
                self.assertEqual(backup.remote_directory(), self.mount / "catbrasil-prod")

    def test_retention_keeps_latest_and_unrelated_files(self):
        directory = self.mount
        names = ["catbrasil-prod-20260101T000000Z.dump.age", "catbrasil-prod-20261019T000000Z.dump.age"]
        for name in names:
            (directory / name).write_bytes(b"encrypted")
            (directory / (name + ".sha256")).write_text("checksum")
        (directory / "unrelated.dump.age").write_bytes(b"other")
        backup.retention(directory, 30, dt.datetime(2026, 10, 20, tzinfo=dt.timezone.utc))
        self.assertFalse((directory / names[0]).exists())
        self.assertFalse((directory / (names[0] + ".sha256")).exists())
        self.assertTrue((directory / names[1]).exists())
        self.assertTrue((directory / "unrelated.dump.age").exists())

    def test_failed_backup_sets_failure_but_preserves_last_success(self):
        backup.publish_status(self.status, success=True, timestamp=100)
        env_file = self.root / "prod.env"
        recipient = self.root / "recipient.pub"
        env_file.write_text("COMPOSE_PROJECT_NAME=catbrasil-prod\n")
        recipient.write_text("age1example")
        with mock.patch.dict(os.environ, {"BACKUP_ENV_FILE": str(env_file), "AGE_RECIPIENT_FILE": str(recipient)}):
            with mock.patch.object(backup.subprocess, "run", side_effect=subprocess.CalledProcessError(1, ["backup.sh"])):
                with self.assertRaises(subprocess.CalledProcessError):
                    backup.run_backup(self.mount, self.status)
        status = self.status.read_text()
        self.assertIn("catbrasil_backup_last_run_success 0", status)
        self.assertIn("catbrasil_backup_last_success_timestamp_seconds 100", status)
        self.assertFalse(list(self.mount.glob("*.dump.age")))

    def test_restore_proof_records_rto_and_host_only_after_success(self):
        name = "catbrasil-prod-20261019T000000Z.dump.age"
        (self.mount / name).write_bytes(b"encrypted")
        (self.mount / (name + ".sha256")).write_text("checksum")
        identity = self.root / "identity.key"
        identity.write_text("private")
        with mock.patch.dict(os.environ, {"AGE_IDENTITY_FILE": str(identity), "BACKUP_RESTORE_MAX_SECONDS": "3600"}):
            with mock.patch.object(backup.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)):
                with mock.patch.object(backup, "verify_remote_mount"):
                    backup.run_restore_proof(self.mount, self.status)
        self.assertIn("catbrasil_restore_proof_last_run_success 1", self.status.read_text())
        report = self.mount / (name + ".restore-proof.json")
        self.assertIn('"restore_duration_seconds"', report.read_text())


if __name__ == "__main__":
    unittest.main()
