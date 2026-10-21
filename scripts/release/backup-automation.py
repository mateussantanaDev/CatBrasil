#!/usr/bin/env python3
"""Scheduled encrypted backups on a verified remote filesystem.

Run ``backup`` on the database host and ``restore-proof`` on an independent
recovery host. Both hosts mount the same remote backup storage. Never place an
age private identity in the database host's backup environment.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time


ROOT = Path(__file__).resolve().parents[2]
REMOTE_TYPES = {"nfs", "nfs4", "cifs", "smb3", "fuse.sshfs", "sshfs"}
NAME = re.compile(r"^catbrasil-prod-\d{8}T\d{6}Z\.dump\.age$")


def required_path(name: str) -> Path:
    value = os.environ.get(name, "")
    if not value or not Path(value).is_absolute():
        raise ValueError(f"{name} deve ser um caminho absoluto configurado")
    return Path(value)


def remote_directory() -> Path:
    mount = required_path("BACKUP_REMOTE_MOUNT")
    if mount.is_symlink() or not mount.is_dir():
        raise ValueError("BACKUP_REMOTE_MOUNT deve ser diretório montado, não symlink")
    verify_remote_mount(mount)
    directory = mount / "catbrasil-prod"
    if directory.is_symlink():
        raise ValueError("diretório de backups não pode ser symlink")
    directory.mkdir(mode=0o700, exist_ok=True)
    if directory.resolve().parent != mount.resolve():
        raise ValueError("diretório de backups escapou do mount remoto")
    verify_remote_mount(mount)
    return directory


def verify_remote_mount(mount: Path) -> None:
    result = subprocess.run(
        ["findmnt", "--mountpoint", str(mount), "--noheadings", "--output", "FSTYPE"],
        capture_output=True, text=True, check=False,
    )
    if result.returncode or result.stdout.strip() not in REMOTE_TYPES:
        raise ValueError("BACKUP_REMOTE_MOUNT não é um mount remoto NFS/CIFS/SSHFS verificado")


def status_path() -> Path:
    path = required_path("BACKUP_STATUS_FILE")
    if path.suffix != ".prom":
        raise ValueError("BACKUP_STATUS_FILE deve terminar em .prom")
    if path.is_symlink() or not path.parent.is_dir():
        raise ValueError("diretório de status inexistente ou arquivo é symlink")
    return path


def publish_status(path: Path, *, success: bool, timestamp: int | None = None, duration: float = 0,
                   restore: bool = False) -> None:
    prefix = "catbrasil_restore_proof" if restore else "catbrasil_backup"
    previous = path.read_text() if path.exists() else ""
    # Preserve the other task's metrics when both jobs write the same textfile.
    lines = [line for line in previous.splitlines() if not line.startswith(prefix + "_")]
    lines.extend([
        f"{prefix}_last_run_success {int(success)}",
        f"{prefix}_last_run_timestamp_seconds {int(time.time())}",
        f"{prefix}_last_run_duration_seconds {duration:.3f}",
    ])
    if timestamp is not None:
        lines.append(f"{prefix}_last_success_timestamp_seconds {timestamp}")
    else:
        # A failed run must not erase the previous successful timestamp.
        for line in previous.splitlines():
            if line.startswith(prefix + "_last_success_timestamp_seconds "):
                lines.append(line)
                break
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


def archives(directory: Path) -> list[Path]:
    return sorted((p for p in directory.iterdir() if NAME.fullmatch(p.name) and p.is_file()
                   and not p.is_symlink() and p.with_name(p.name + ".sha256").is_file()), reverse=True)


def retention(directory: Path, days: int, now: dt.datetime) -> None:
    if days < 7:
        raise ValueError("BACKUP_RETENTION_DAYS deve ser >= 7")
    cutoff = now - dt.timedelta(days=days)
    valid = archives(directory)
    # Never remove the newest valid pair, even if it has aged past retention.
    for archive in valid[1:]:
        created = dt.datetime.strptime(archive.name[15:30], "%Y%m%dT%H%M%S").replace(tzinfo=dt.timezone.utc)
        if created < cutoff:
            archive.unlink()
            archive.with_name(archive.name + ".sha256").unlink()


def run_backup(directory: Path, status: Path) -> None:
    env_file = required_path("BACKUP_ENV_FILE")
    recipient_file = required_path("AGE_RECIPIENT_FILE")
    if not env_file.is_file() or not recipient_file.is_file():
        raise ValueError("BACKUP_ENV_FILE e AGE_RECIPIENT_FILE devem existir")
    now = dt.datetime.now(dt.timezone.utc)
    archive = directory / f"catbrasil-prod-{now:%Y%m%dT%H%M%SZ}.dump.age"
    start = time.monotonic()
    try:
        subprocess.run([
            str(ROOT / "scripts/release/backup.sh"), "--project", "catbrasil-prod",
            "--env-file", str(env_file), "--recipient-file", str(recipient_file),
            str(archive),
        ], cwd=ROOT, check=True)
        if not archive.is_file() or not archive.with_name(archive.name + ".sha256").is_file():
            raise RuntimeError("backup não publicou os dois artefatos obrigatórios")
        verify_remote_mount(directory.parent)
        retention(directory, int(os.environ.get("BACKUP_RETENTION_DAYS", "30")), now)
    except Exception:
        publish_status(status, success=False, duration=time.monotonic() - start)
        raise
    publish_status(status, success=True, timestamp=int(now.timestamp()), duration=time.monotonic() - start)
    print(f"Backup remoto válido: {archive}")


def run_restore_proof(directory: Path, status: Path) -> None:
    identity = required_path("AGE_IDENTITY_FILE")
    if not identity.is_file():
        raise ValueError("AGE_IDENTITY_FILE não existe")
    candidates = archives(directory)
    if not candidates:
        raise ValueError("nenhum backup remoto íntegro para restaurar")
    archive = candidates[0]
    start = time.monotonic()
    try:
        subprocess.run([
            str(ROOT / "scripts/release/restore-verify.sh"), "--identity", str(identity), str(archive),
        ], cwd=ROOT, check=True)
        verify_remote_mount(directory.parent)
    except Exception:
        publish_status(status, success=False, duration=time.monotonic() - start, restore=True)
        raise
    duration = time.monotonic() - start
    max_seconds = int(os.environ.get("BACKUP_RESTORE_MAX_SECONDS", "3600"))
    if duration > max_seconds:
        publish_status(status, success=False, duration=duration, restore=True)
        raise RuntimeError(f"RTO excedido: {duration:.1f}s > {max_seconds}s")
    publish_status(status, success=True, timestamp=int(time.time()), duration=duration, restore=True)
    backup_time = dt.datetime.strptime(archive.name[15:30], "%Y%m%dT%H%M%S").replace(tzinfo=dt.timezone.utc)
    evidence = {
        "verified_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "archive": archive.name,
        "backup_age_seconds": round(time.time() - backup_time.timestamp(), 3),
        "restore_duration_seconds": round(duration, 3),
        "max_restore_seconds": max_seconds,
        "proof_host": os.uname().nodename,
    }
    report = directory / (archive.name + ".restore-proof.json")
    temp = report.with_name(report.name + f".{os.getpid()}.partial")
    try:
        with temp.open("x", encoding="utf-8") as stream:
            os.fchmod(stream.fileno(), 0o600)
            json.dump(evidence, stream, sort_keys=True)
            stream.write("\n")
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temp, report)
    finally:
        temp.unlink(missing_ok=True)
    print(json.dumps(evidence, sort_keys=True))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("backup", "restore-proof"))
    args = parser.parse_args()
    os.umask(0o077)
    try:
        status = status_path()
        directory = remote_directory()
        if args.mode == "backup":
            run_backup(directory, status)
        else:
            run_restore_proof(directory, status)
        return 0
    except (ValueError, OSError, subprocess.CalledProcessError, RuntimeError) as exc:
        print(f"ERRO: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
