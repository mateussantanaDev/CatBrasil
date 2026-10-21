#!/usr/bin/env python3
"""Read-only gate for the isolated local staging stack (no SuitPay calls)."""

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
RUNNING = {
    "postgres", "backend-api", "backend-cron", "app-client", "app-admin",
    "caddy", "sms-sandbox", "pix-sandbox", "prometheus", "alertmanager", "alert-relay", "alert-sink",
    "node-exporter",
}
HEALTHY = {"postgres", "backend-api", "app-client", "app-admin", "sms-sandbox", "pix-sandbox",
           "alertmanager", "alert-relay", "alert-sink"}


def migration_versions(root=ROOT):
    versions = sorted(int(match.group(1)) for path in (root / "backend/migrations").glob("*.up.sql")
                      if (match := re.fullmatch(r"(\d{6})_.+\.up\.sql", path.name)))
    if not versions or versions != list(range(1, len(versions) + 1)):
        raise ValueError("migrações SQL de origem ausentes ou fora de sequência")
    return versions


def run(args, *, timeout=20):
    try:
        result = subprocess.run(args, cwd=ROOT, capture_output=True, text=True, timeout=timeout, check=True)
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
        # Docker/Compose stderr can include interpolated environment values. Never print it.
        raise ValueError(f"comando falhou: {args[0]} {args[1] if len(args) > 1 else ''}") from exc
    return result.stdout


def parse_compose_ps(output):
    entries = []
    for line in output.splitlines():
        if line.strip():
            value = json.loads(line)
            entries.extend(value if isinstance(value, list) else [value])
    return {entry["Service"]: entry for entry in entries}


def check_services(services):
    errors = []
    for name in sorted(RUNNING):
        service = services.get(name)
        if not service or service.get("State") != "running":
            errors.append(f"{name}: serviço ausente ou parado")
        elif name in HEALTHY and service.get("Health") != "healthy":
            errors.append(f"{name}: healthcheck não está healthy")
    migration = services.get("backend-migrate")
    if not migration or migration.get("State") != "exited" or migration.get("ExitCode") != 0:
        errors.append("backend-migrate: execução concluída com exit 0 não encontrada")
    return errors


def database_versions(postgres_id):
    # Environment expands inside the container; credentials never enter argv or output.
    query = "SELECT coalesce(string_agg(version::text, ',' ORDER BY version), '') FROM schema_migrations"
    output = run(["docker", "exec", postgres_id, "sh", "-c",
                  'psql -X -q -t -A -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$1"',
                  "staging-check", query])
    try:
        return [int(value) for value in output.strip().split(",") if value]
    except ValueError as exc:
        raise ValueError("resposta inválida de schema_migrations") from exc


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", default="catbrasil-staging")
    parser.add_argument("--env-file", default=".staging.env")
    args = parser.parse_args(argv)
    if args.project != "catbrasil-staging":
        parser.error("este validador aceita apenas o projeto catbrasil-staging")
    env_file = (ROOT / args.env_file).resolve()
    if not env_file.is_file():
        parser.error("arquivo de ambiente de staging não encontrado")
    try:
        expected = migration_versions()
        compose = ["docker", "compose", "-p", args.project, "--env-file", str(env_file),
                   "-f", "docker-compose.yml", "-f", "compose.staging.yml"]
        services = parse_compose_ps(run(compose + ["ps", "--all", "--format", "json"]))
        errors = check_services(services)
        postgres = services.get("postgres")
        applied = database_versions(postgres["ID"]) if postgres and postgres.get("State") == "running" else []
        if applied != expected:
            errors.append(f"schema_migrations={applied}; origem exige {expected}")
    except (ValueError, KeyError, json.JSONDecodeError) as exc:
        print(f"FALHA: {exc}", file=sys.stderr)
        return 1
    for error in errors:
        print(f"FALHA: {error}", file=sys.stderr)
    if errors:
        return 1
    print(f"OK: staging isolado, serviços ativos e migrações 1..{expected[-1]} aplicadas")
    print("Nota: este gate não comprova que as imagens correspondem ao commit; reconstrua-as antes do ensaio.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
