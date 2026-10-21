#!/usr/bin/env python3
"""Daily, read-only internal ledger audit of the isolated production PostgreSQL.

This does not reconcile custody against SuitPay's official statement.
"""

from __future__ import annotations

import datetime as dt
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]
REPORT_NAME = re.compile(r"^internal-prod-\d{8}T\d{6}Z-[0-9a-f]{8}\.(json|md)$")
MARKER_NAME = re.compile(r"^successful-\d{8}\.json$")


def configured_path(name: str, *, suffix: str | None = None) -> Path:
    raw = os.environ.get(name, "")
    path = Path(raw)
    if not raw or not path.is_absolute() or path.is_symlink() or (suffix and path.suffix != suffix):
        raise ValueError(f"{name} inválido")
    return path


def private_dir(path: Path) -> None:
    if path.is_symlink():
        raise ValueError("Diretório de relatório não pode ser symlink")
    path.mkdir(mode=0o700, parents=True, exist_ok=True)
    if not path.is_dir() or path.stat().st_mode & 0o077:
        raise ValueError("Diretório de relatório deve ser privado (0700)")


def publish_status(path: Path, success: bool, timestamp: int | None = None,
                   audited_day: str | None = None) -> None:
    if path.is_symlink() or not path.parent.is_dir():
        raise ValueError("RECONCILE_STATUS_FILE inválido")
    previous = path.read_text() if path.is_file() else ""
    last_success = 0
    for line in previous.splitlines():
        if line.startswith("catbrasil_internal_reconcile_last_success_timestamp_seconds "):
            try:
                last_success = int(line.split()[1])
            except (IndexError, ValueError):
                pass
    lines = [
        f"catbrasil_internal_reconcile_last_run_success {int(success)}",
        f"catbrasil_internal_reconcile_last_run_timestamp_seconds {int(time.time())}",
        f"catbrasil_internal_reconcile_last_success_timestamp_seconds {timestamp or last_success}",
    ]
    if audited_day:
        lines.append(f"catbrasil_internal_reconcile_audited_day{{day=\"{audited_day}\"}} 1")
    temp = path.with_name(path.name + f".{os.getpid()}.partial")
    try:
        with temp.open("x", encoding="utf-8") as stream:
            os.fchmod(stream.fileno(), 0o600)
            stream.write("\n".join(lines) + "\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temp, path)
    finally:
        temp.unlink(missing_ok=True)


def retain_reports(directory: Path, days: int, now: dt.datetime) -> None:
    if days < 30:
        raise ValueError("RECONCILE_RETENTION_DAYS deve ser >= 30")
    cutoff = now - dt.timedelta(days=days)
    for path in directory.iterdir():
        if not path.is_file() or path.is_symlink():
            continue
        if REPORT_NAME.fullmatch(path.name):
            created = dt.datetime.strptime(path.name[14:29], "%Y%m%dT%H%M%S").replace(tzinfo=dt.timezone.utc)
        elif MARKER_NAME.fullmatch(path.name):
            created = dt.datetime.strptime(path.name[11:19], "%Y%m%d").replace(tzinfo=dt.timezone.utc)
        else:
            continue
        if created < cutoff:
            path.unlink()


def run() -> int:
    os.umask(0o077)
    reports = configured_path("RECONCILE_REPORT_DIR")
    status = configured_path("RECONCILE_STATUS_FILE", suffix=".prom")
    retention_days = int(os.environ.get("RECONCILE_RETENTION_DAYS", "90"))
    if retention_days < 30:
        raise ValueError("RECONCILE_RETENTION_DAYS deve ser >= 30")
    private_dir(reports)
    now = dt.datetime.now(dt.timezone.utc)
    audited_date = (now - dt.timedelta(days=1)).date()
    day = audited_date.isoformat()
    marker = reports / f"successful-{audited_date:%Y%m%d}.json"
    lock = reports / ".daily.lock"
    with lock.open("a+", encoding="utf-8") as stream:
        os.fchmod(stream.fileno(), 0o600)
        fcntl.flock(stream.fileno(), fcntl.LOCK_EX)
        if marker.is_file() and not marker.is_symlink():
            try:
                prior = json.loads(marker.read_text())
                prior_report = prior.get("report", "")
                if (prior.get("day") == day and prior.get("target") == "production"
                        and isinstance(prior_report, str)
                        and REPORT_NAME.fullmatch(prior_report)
                        and not (reports / prior_report).is_symlink()
                        and (reports / prior_report).is_file()):
                    publish_status(status, True, int(prior["completed_at"]), day)
                    return 0
            except (OSError, ValueError, KeyError, TypeError):
                pass

        stamp = now.strftime("%Y%m%dT%H%M%SZ") + f"-{os.urandom(4).hex()}"
        json_path = reports / f"internal-prod-{stamp}.json"
        md_path = reports / f"internal-prod-{stamp}.md"
        try:
            proc = subprocess.run([
                sys.executable, str(ROOT / "scripts/release/reconcile.py"),
                "--target", "production", "--confirm-production", "--check-only",
                "--output-json", str(json_path), "--output-md", str(md_path),
            ], cwd=ROOT, capture_output=True, timeout=180, check=False)
            if proc.returncode:
                raise RuntimeError("Auditoria interna falhou ou encontrou divergência")
            report = json.loads(json_path.read_text())
            if (report.get("audit_metadata", {}).get("target") != "production"
                    or report.get("audit_metadata", {}).get("database_name") != "catbrasil_prod"
                    or report.get("balance_vs_latest_event", {}).get("discrepancies_count") != 0
                    or report.get("financial_events_coverage_and_continuity", {})
                    .get("continuity_verification", {}).get("anomalies_count") != 0
                    or not md_path.is_file()):
                raise RuntimeError("Relatório inválido ou divergente")
            retain_reports(reports, retention_days, now)
            completed_at = int(time.time())
            with marker.open("x", encoding="utf-8") as result:
                os.fchmod(result.fileno(), 0o600)
                json.dump({"day": day, "target": "production", "completed_at": completed_at,
                           "report": json_path.name}, result)
                result.write("\n")
                result.flush()
                os.fsync(result.fileno())
            publish_status(status, True, completed_at, day)
            return 0
        except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired, json.JSONDecodeError):
            publish_status(status, False)
            raise


if __name__ == "__main__":
    try:
        raise SystemExit(run())
    except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired, json.JSONDecodeError) as exc:
        print(f"Auditoria interna diária falhou ({type(exc).__name__}); verifique relatório privado e status.", file=sys.stderr)
        raise SystemExit(1)
