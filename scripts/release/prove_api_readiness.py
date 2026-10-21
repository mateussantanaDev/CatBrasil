#!/usr/bin/env python3
"""Prova fail-fast na partida e recuperação da API após queda do PostgreSQL."""

import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import urlopen


BACKEND = Path(__file__).resolve().parents[2] / "backend"


def run(args, **kwargs):
    try:
        return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs)
    except subprocess.CalledProcessError as exc:
        raise RuntimeError(f"comando {args[0]} {args[1]} falhou") from exc


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def status(port, path):
    try:
        with urlopen(f"http://127.0.0.1:{port}{path}", timeout=4) as response:
            response.read(256)
            return response.status
    except HTTPError as exc:
        return exc.code
    except (OSError, URLError):
        return 0


def await_condition(check, label, timeout=30):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if check():
            return
        time.sleep(0.25)
    raise RuntimeError(f"tempo esgotado aguardando {label}")


def synthetic_env(dsn, port):
    return dict(os.environ, **{
        "APP_ENV": "development", "PIX_MODE": "simulated", "DATABASE_URL": dsn,
        "JWT_SECRET": "synthetic-jwt-secret-0123456789abcdef",
        "PIX_WEBHOOK_SECRET": "synthetic-webhook-secret-0123456789abcdef",
        "ADMIN_SECRET_PIN": "synthetic-pin", "PORT": str(port),
        "SUITPAY_LIVE_ENABLED": "false", "IDENTITY_SECRET": "",
        "SMS_GATEWAY_URL": "", "PIX_SANDBOX_URL": "",
    })


def main():
    container = "catbrasil-api-readiness-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(24)
    api_port = free_port()
    reserved_db_port = free_port()
    created = False
    api = None
    with tempfile.TemporaryDirectory(prefix="catbrasil-readiness-") as temp:
        api_binary = Path(temp) / "api"
        log_file = Path(temp) / "api.log"
        try:
            run(["go", "build", "-o", str(api_binary), "./cmd/api"], cwd=BACKEND)

            # Sem banco, a API deve encerrar antes de abrir a porta HTTP.
            missing_dsn = "postgres://p07:synthetic@postgres.invalid:5432/p07?sslmode=disable"
            failed = subprocess.run([str(api_binary)], env=synthetic_env(missing_dsn, api_port), capture_output=True, timeout=12)
            if failed.returncode == 0 or status(api_port, "/health") != 0:
                raise RuntimeError("API iniciou ou abriu HTTP sem banco")

            run([
                "docker", "run", "--pull", "never", "-d", "--name", container,
                "-e", "POSTGRES_USER=p07", "-e", "POSTGRES_DB=p07",
                "-e", "POSTGRES_PASSWORD=" + password,
                "-p", f"127.0.0.1:{reserved_db_port}:5432", "postgres:16-alpine",
            ])
            created = True
            await_condition(lambda: subprocess.run([
                "docker", "exec", container, "psql", "-X", "-A", "-t", "-U", "p07", "-d", "p07", "-c", "SELECT 1",
            ], capture_output=True).returncode == 0, "criação do banco")
            binding = run(["docker", "port", container, "5432/tcp"]).stdout.strip()
            if not binding.startswith("127.0.0.1:"):
                raise RuntimeError("PostgreSQL de prova não está restrito ao loopback")
            db_port = int(binding.rsplit(":", 1)[1])
            if db_port != reserved_db_port:
                raise RuntimeError("porta do banco divergiu da reserva de prova")
            dsn = f"postgres://p07:{quote(password, safe='')}@127.0.0.1:{db_port}/p07?sslmode=disable"
            run(["go", "run", "./cmd/migrate", "up"], cwd=BACKEND, env=synthetic_env(dsn, api_port))

            with log_file.open("wb") as log:
                api = subprocess.Popen([str(api_binary)], env=synthetic_env(dsn, api_port), stdout=log, stderr=subprocess.STDOUT)
                await_condition(lambda: status(api_port, "/ready") == 200, "readiness inicial da API")
                if status(api_port, "/health") != 200 or status(api_port, "/api/v1/store/machines") != 200:
                    raise RuntimeError("API saudável não atende liveness ou catálogo")

                run(["docker", "stop", "--time", "2", container])
                await_condition(lambda: status(api_port, "/ready") == 503, "readiness 503 após queda do banco", timeout=15)
                if status(api_port, "/health") != 200:
                    raise RuntimeError("liveness deve permanecer 200 durante queda do banco")
                if api.poll() is not None:
                    raise RuntimeError("API encerrou durante queda transitória do banco")

                run(["docker", "start", container])
                rebound = run(["docker", "port", container, "5432/tcp"]).stdout.strip()
                if rebound != binding:
                    raise RuntimeError("porta efêmera do Docker mudou no reinício do banco")
                await_condition(lambda: subprocess.run([
                    "docker", "exec", container, "psql", "-X", "-A", "-t", "-U", "p07", "-d", "p07", "-c", "SELECT 1",
                ], capture_output=True).returncode == 0, "reinício do banco")
                history = run(["docker", "exec", container, "psql", "-X", "-A", "-t", "-U", "p07", "-d", "p07", "-c", "SELECT count(*) FROM schema_migrations"]).stdout.strip()
                if history != "10":
                    raise RuntimeError("histórico das migrações não persistiu após reinício do banco")
                run(["go", "run", "./cmd/migrate", "status"], cwd=BACKEND, env=synthetic_env(dsn, api_port))
                await_condition(lambda: status(api_port, "/ready") == 200, "readiness após reconexão", timeout=30)
                if status(api_port, "/api/v1/store/machines") != 200:
                    raise RuntimeError("catálogo não recuperou após retorno do banco")

            print(json.dumps({
                "result": "PASS", "startup_without_database": "failed_closed",
                "liveness_during_database_outage": 200,
                "readiness_during_database_outage": 503,
                "readiness_after_reconnect": 200,
                "catalog_after_reconnect": 200,
                "production_touched": False,
            }, sort_keys=True))
            return 0
        finally:
            if api is not None and api.poll() is None:
                api.terminate()
                try:
                    api.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    api.kill()
                    api.wait()
            if created:
                subprocess.run(["docker", "rm", "-f", "-v", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.TimeoutExpired) as exc:
        print(f"PROVA DE READINESS BLOQUEADA: {type(exc).__name__}: {exc if isinstance(exc, RuntimeError) else ''}", file=sys.stderr)
        sys.exit(1)
