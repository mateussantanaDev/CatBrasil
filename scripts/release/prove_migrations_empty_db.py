#!/usr/bin/env python3
"""Prova o CLI de migração em PostgreSQL 16 descartável, sem tocar nas stacks existentes."""

import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import time
from urllib.parse import quote

from preflight_cutover import ALL_REQUIRED_TABLES, EXPECTED_MIGRATION_SEQUENCE


ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "backend"
REQUIRED_COLUMNS = (
    ("users", "insurance_vouchers_count"),  # 000002
    ("users", "level12_bonus_suspended"),   # 000003
    ("system_config", "gateway_provider"),  # 000004
    ("transactions", "gateway_provider"),   # 000004
)


def command(args, *, cwd=None, env=None, check=True):
    try:
        return subprocess.run(args, cwd=cwd, env=env, check=check, capture_output=True, text=True)
    except subprocess.CalledProcessError as exc:
        # A saída dos comandos pode conter o DSN sintético; mostre só a etapa.
        stage = " ".join(args[:2])
        if args[:2] == ["docker", "exec"] and len(args) > 3:
            stage += " " + args[3]
        raise RuntimeError(f"comando {stage} falhou") from exc


def psql(container, sql):
    return command(["docker", "exec", container, "psql", "-X", "-A", "-t", "-U", "p07", "-d", "p07", "-c", sql]).stdout.strip()


def main():
    for binary in ("docker", "go"):
        if not shutil.which(binary):
            raise RuntimeError(f"dependência ausente: {binary}")
    if command(["docker", "info"], check=False).returncode:
        raise RuntimeError("Docker indisponível")

    container = "catbrasil-p07-migrations-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(24)
    started = time.monotonic()
    created = False
    try:
        command([
            "docker", "run", "--rm", "--pull", "never", "-d", "--name", container,
            "-e", "POSTGRES_USER=p07", "-e", "POSTGRES_DB=p07",
            "-e", "POSTGRES_PASSWORD=" + password,
            "-p", "127.0.0.1::5432", "postgres:16-alpine",
        ])
        created = True
        for _ in range(60):
            ready = command([
                "docker", "exec", container, "psql", "-X", "-A", "-t", "-U", "p07", "-d", "p07", "-c", "SELECT 1",
            ], check=False)
            if ready.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("PostgreSQL descartável não ficou pronto")

        port_output = command(["docker", "port", container, "5432/tcp"]).stdout.strip()
        if not port_output.startswith("127.0.0.1:"):
            raise RuntimeError("PostgreSQL descartável não está restrito ao loopback")
        port = int(port_output.rsplit(":", 1)[1])
        dsn = f"postgres://p07:{quote(password, safe='')}@127.0.0.1:{port}/p07?sslmode=disable"
        env = dict(os.environ, DATABASE_URL=dsn)

        initial = psql(container, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
        if initial != "0":
            raise RuntimeError("o banco de prova não começou vazio")

        command(["go", "run", "./cmd/migrate", "up"], cwd=BACKEND, env=env)
        first_history = psql(container, "SELECT version || ':' || name || ':' || checksum FROM schema_migrations ORDER BY version")
        versions = [int(row.split(":", 1)[0]) for row in first_history.splitlines()]
        if versions != EXPECTED_MIGRATION_SEQUENCE:
            raise RuntimeError("sequência das migrações divergente")

        tables = set(psql(container, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name").splitlines())
        if tables != set(ALL_REQUIRED_TABLES):
            raise RuntimeError("tabelas criadas divergem do contrato 12/26")

        for table, column in REQUIRED_COLUMNS:
            found = psql(container, f"SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='{table}' AND column_name='{column}'")
            if found != "1":
                raise RuntimeError(f"coluna exigida pela API ausente: {table}.{column}")

        plain_pin = command([
            "docker", "exec", container, "psql", "-X", "-v", "ON_ERROR_STOP=1",
            "-U", "p07", "-d", "p07", "-c",
            "INSERT INTO users (id, username, password_hash, security_password) VALUES ('pin-plain', 'pin-plain', 'hash', '123456')",
        ], check=False)
        if plain_pin.returncode == 0:
            raise RuntimeError("schema aceitou PIN financeiro em texto simples")
        if psql(container, "SELECT COUNT(*) FROM users") != "0":
            raise RuntimeError("teste de PIN alterou o estado zero")

        hashed_pin = command([
            "docker", "exec", container, "psql", "-X", "-v", "ON_ERROR_STOP=1",
            "-U", "p07", "-d", "p07", "-c",
            "BEGIN; INSERT INTO users (id, username, password_hash, security_password) VALUES ('pin-hash', 'pin-hash', 'hash', crypt('123456', gen_salt('bf', 12))); ROLLBACK",
        ], check=False)
        if hashed_pin.returncode != 0:
            raise RuntimeError("schema rejeitou hash bcrypt válido")

        command(["go", "run", "./cmd/migrate", "up"], cwd=BACKEND, env=env)
        second_history = psql(container, "SELECT version || ':' || name || ':' || checksum FROM schema_migrations ORDER BY version")
        if second_history != first_history:
            raise RuntimeError("reexecutar migrações alterou o histórico")
        command(["go", "run", "./cmd/migrate", "status"], cwd=BACKEND, env=env)

        compose_env = dict(os.environ, **{
            "DB_PASSWORD": "synthetic", "DATABASE_URL": "postgres://p07:synthetic@postgres:5432/catbrasil?sslmode=disable",
            "JWT_SECRET": "synthetic", "APP_ENV": "staging", "PIX_MODE": "suitpay",
            "PIX_WEBHOOK_SECRET": "synthetic", "ADMIN_SECRET_PIN": "synthetic",
            "IDENTITY_SECRET": "synthetic", "CORS_ALLOWED_ORIGINS": "http://localhost",
            "CLIENT_DOMAIN": "localhost", "ADMIN_DOMAIN": "admin.localhost",
            "API_DOMAIN": "api.localhost", "ACME_EMAIL": "synthetic@localhost",
        })
        rendered = command([
            "docker", "compose", "-p", "catbrasil-p07-config", "-f", "docker-compose.yml",
            "config", "--format", "json",
        ], cwd=ROOT, env=compose_env)
        services = json.loads(rendered.stdout)["services"]
        pg_health = services["postgres"]["healthcheck"]["test"]
        if pg_health[0] != "CMD-SHELL" or "psql" not in pg_health[1] or "$POSTGRES_DB" not in pg_health[1]:
            raise RuntimeError("healthcheck do PostgreSQL não testa conexão SQL ao banco criado")
        if services["backend-migrate"]["command"] != ["/app/migrate", "up"]:
            raise RuntimeError("Compose não invoca o CLI de migrações")
        for service in ("backend-api", "backend-cron"):
            condition = services[service]["depends_on"]["backend-migrate"]["condition"]
            if condition != "service_completed_successfully":
                raise RuntimeError(f"Compose não aguarda migração antes de {service}")

        report = {
            "result": "PASS",
            "database": "disposable PostgreSQL 16; loopback only",
            "versions": versions,
            "tables": len(tables),
            "representative_columns": len(REQUIRED_COLUMNS),
            "plaintext_financial_pin_rejected": True,
            "second_run_unchanged": True,
            "compose_api_and_cron_wait_for_migrate": True,
            "postgres_healthcheck_connects_to_database": True,
            "elapsed_seconds": round(time.monotonic() - started, 2),
            "production_touched": False,
        }
        print(json.dumps(report, ensure_ascii=False, sort_keys=True))
        return 0
    finally:
        if created:
            subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except RuntimeError as exc:
        print(f"PROVA P0.7 BLOQUEADA: {exc}", file=sys.stderr)
        sys.exit(1)
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"PROVA P0.7 BLOQUEADA: {type(exc).__name__}", file=sys.stderr)
        sys.exit(1)
