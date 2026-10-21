#!/usr/bin/env python3
"""Configura apenas o stub PIX no banco isolado catbrasil-staging."""

import argparse
import json
from pathlib import Path
import subprocess
import sys


class BootstrapError(Exception):
    pass


ROOT = Path(__file__).resolve().parents[2]
COMPOSE = ["docker", "compose", "-p", "catbrasil-staging", "--env-file", ".staging.env",
           "-f", "docker-compose.yml", "-f", "compose.staging.yml"]
PSQL = ["exec", "-T", "postgres", "sh", "-c",
        'exec psql -XAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"']


def run_compose(args, stdin=None):
    try:
        return subprocess.run(COMPOSE + args, cwd=ROOT, input=stdin, text=True,
                              capture_output=True, check=True, timeout=30).stdout.strip()
    except (subprocess.CalledProcessError, OSError, subprocess.TimeoutExpired) as exc:
        # Compose config e psql podem conter segredos; não imprimir stderr.
        raise BootstrapError(f"Comando de staging falhou ({type(exc).__name__})") from exc


def literal(value):
    return "'" + value.replace("'", "''") + "'"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--confirm-staging", action="store_true", required=True)
    parser.parse_args()
    if not (ROOT / ".staging.env").is_file():
        raise BootstrapError(".staging.env ausente")
    try:
        config = json.loads(run_compose(["config", "--format", "json"]))
        api = config["services"]["backend-api"]["environment"]
        stub = config["services"]["pix-sandbox"]["environment"]
        client_id = stub["PIX_SANDBOX_CLIENT_ID"]
        client_secret = stub["PIX_SANDBOX_CLIENT_SECRET"]
    except (KeyError, ValueError, TypeError) as exc:
        raise BootstrapError("Configuração Compose de staging incompleta") from exc
    if config.get("name") != "catbrasil-staging":
        raise BootstrapError("Projeto Compose não é catbrasil-staging")
    if api.get("APP_ENV") != "staging" or api.get("PIX_MODE") != "suitpay":
        raise BootstrapError("API não está em APP_ENV=staging/PIX_MODE=suitpay")
    if api.get("PIX_SANDBOX_URL") != "http://pix-sandbox:8080/api/v1/gateway":
        raise BootstrapError("API não aponta para o stub PIX interno")
    if not client_id or len(client_secret) < 32:
        raise BootstrapError("Credenciais do stub PIX ausentes")
    container = run_compose(["ps", "-q", "postgres"])
    if not container:
        raise BootstrapError("PostgreSQL de staging não está em execução")
    try:
        label = subprocess.run(
            ["docker", "inspect", "-f", '{{ index .Config.Labels "com.docker.compose.project" }}', container],
            capture_output=True, text=True, check=True, timeout=10).stdout.strip()
    except (subprocess.CalledProcessError, OSError, subprocess.TimeoutExpired) as exc:
        raise BootstrapError("Não foi possível verificar o projeto do PostgreSQL") from exc
    if label != "catbrasil-staging":
        raise BootstrapError("PostgreSQL pertence a outro projeto")

    # Valores são escapados como literais SQL e enviados por stdin, nunca em argv/log.
    sql = ("UPDATE system_config SET "
           "gateway_name='PIX staging local', "
           f"suitpay_client_id={literal(client_id)}, "
           f"suitpay_client_secret={literal(client_secret)}, "
           "suitpay_sandbox=TRUE, suitpay_enabled=TRUE, "
           "suitpay_callback_url='http://backend-api:8080/api/v1/webhooks/suitpay', "
           "updated_at=NOW() WHERE id=1 RETURNING id;\n")
    if run_compose(PSQL, sql).splitlines()[0:1] != ["1"]:
        raise BootstrapError("system_config de staging não foi atualizado")
    proof = run_compose(PSQL, "SELECT suitpay_sandbox, suitpay_enabled, "
                                   "length(suitpay_client_id)>0, length(suitpay_client_secret)>0 "
                                   "FROM system_config WHERE id=1;\n")
    if proof != "t|t|t|t":
        raise BootstrapError("Configuração PIX sandbox não passou na leitura de volta")
    print("Gateway PIX stub configurado e verificado no projeto catbrasil-staging.")


if __name__ == "__main__":
    try:
        main()
    except BootstrapError as exc:
        print("FALHA:", exc, file=sys.stderr)
        sys.exit(1)
