#!/usr/bin/env python3
"""Prova P0.10 em PostgreSQL descartável; não toca staging nem produção."""

import base64
import hashlib
import hmac
import os
import pty
import re
import secrets
import select
import socket
import subprocess
import time
from pathlib import Path
from urllib import error as urlerror
from urllib import request as urlrequest
from urllib.parse import quote

ROOT = Path(__file__).resolve().parents[2]
BACKEND = ROOT / "backend"


def command(args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, text=True, **kwargs)


def read_until(fd, marker, timeout=60):
    output = b""
    deadline = time.monotonic() + timeout
    while marker not in output:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise RuntimeError("bootstrap não apresentou o próximo prompt")
        ready, _, _ = select.select([fd], [], [], remaining)
        if not ready:
            raise RuntimeError("timeout no bootstrap")
        try:
            chunk = os.read(fd, 4096)
        except OSError as exc:
            if marker == b"Senha exclusiva do diretor":
                raise RuntimeError("bootstrap terminou antes do primeiro prompt: " + output.decode(errors="replace")[:300]) from exc
            raise RuntimeError("bootstrap terminou antes do prompt") from exc
        if not chunk:
            if marker == b"Senha exclusiva do diretor":
                raise RuntimeError("bootstrap terminou antes do primeiro prompt: " + output.decode(errors="replace")[:300])
            raise RuntimeError("bootstrap terminou antes do prompt")
        output += chunk
    return output


def totp(secret):
    key = base64.b32decode(secret)
    counter = int(time.time() // 30).to_bytes(8, "big")
    digest = hmac.new(key, counter, hashlib.sha1).digest()
    offset = digest[-1] & 15
    return f"{(int.from_bytes(digest[offset:offset+4], 'big') & 0x7fffffff) % 1000000:06d}"


def interactive_bootstrap(env):
    master, slave = pty.openpty()
    process = subprocess.Popen(
        ["go", "run", "./cmd/bootstrap-admin", "--username", "release_director"],
        cwd=BACKEND, env=env, stdin=slave, stdout=slave, stderr=slave,
    )
    os.close(slave)
    try:
        try:
            first = read_until(master, b"Senha exclusiva do diretor")
        except RuntimeError as exc:
            process.wait(timeout=5)
            raise RuntimeError(f"prompt 1: processo saiu com status {process.returncode}; " + str(exc)) from None
        os.write(master, b"senha-unica-forte-p010-2026\n")
        try:
            second = read_until(master, b"Repita a senha")
        except RuntimeError:
            raise RuntimeError("prompt 2: bootstrap terminou antes da confirmação") from None
        os.write(master, b"senha-unica-forte-p010-2026\n")
        try:
            third = read_until(master, b"C\xc3\xb3digo TOTP atual")
        except RuntimeError:
            raise RuntimeError("prompt 3: bootstrap terminou antes do código TOTP") from None
        secret_match = re.search(rb"secret=([A-Z2-7]+)", first + second + third)
        if not secret_match:
            raise RuntimeError("bootstrap não forneceu segredo TOTP ao terminal")
        os.write(master, totp(secret_match.group(1).decode()).encode() + b"\n")
        process.wait(timeout=30)
        if process.returncode != 0:
            raise RuntimeError("bootstrap inicial falhou")
        return secret_match.group(1).decode()
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        os.close(master)


def assert_second_bootstrap_refused(env):
    master, slave = pty.openpty()
    process = subprocess.Popen(
        ["go", "run", "./cmd/bootstrap-admin", "--username", "outro_diretor"],
        cwd=BACKEND, env=env, stdin=slave, stdout=slave, stderr=slave,
    )
    os.close(slave)
    try:
        output = b""
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            ready, _, _ = select.select([master], [], [], 0.2)
            if not ready and process.poll() is not None:
                break
            if not ready:
                continue
            try:
                chunk = os.read(master, 4096)
            except OSError:
                break
            if not chunk:
                break
            output += chunk
        process.wait(timeout=5)
        if process.returncode == 0 or b"bootstrap exige banco sem usu" not in output:
            raise RuntimeError("segundo bootstrap interativo não foi recusado pelo estado do banco")
    finally:
        if process.poll() is None:
            process.kill()
            process.wait()
        os.close(master)


def prove_erp_login(env, totp_secret):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    api_env = dict(env, PORT=str(port), PIX_MODE="suitpay",
                   JWT_SECRET="p010-jwt-secret-0123456789abcdef0123",
                   PIX_WEBHOOK_SECRET="p010-webhook-secret-0123456789abcdef",
                   IDENTITY_SECRET="p010-identity-secret-0123456789abcdef",
                   CORS_ALLOWED_ORIGINS=f"http://127.0.0.1:{port}")
    process = subprocess.Popen(["go", "run", "./cmd/api"], cwd=BACKEND, env=api_env,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        for _ in range(80):
            try:
                with urlrequest.urlopen(f"http://127.0.0.1:{port}/ready", timeout=1) as response:
                    if response.status == 200:
                        break
            except (OSError, urlerror.URLError):
                if process.poll() is not None:
                    raise RuntimeError("API de prova encerrou antes de /ready")
            time.sleep(0.25)
        else:
            raise RuntimeError("API de prova não ficou pronta")

        def login(pin, code):
            payload = ('{"username":"release_director","password":"senha-unica-forte-p010-2026",'
                       f'"pin":"{pin}","twoFactorCode":"{code}"}}').encode()
            req = urlrequest.Request(f"http://127.0.0.1:{port}/api/v1/admin/auth/login", payload,
                                     {"Content-Type": "application/json"}, method="POST")
            try:
                with urlrequest.urlopen(req, timeout=5) as response:
                    return response.status
            except urlerror.HTTPError as exc:
                return exc.code

        if login("pin-incorreto", totp(totp_secret)) != 401:
            raise RuntimeError("ERP aceitou PIN incorreto")
        wrong_code = "000000" if totp(totp_secret) != "000000" else "000001"
        if login("pin-p010-exclusivo", wrong_code) != 401:
            raise RuntimeError("ERP aceitou TOTP incorreto")
        if login("pin-p010-exclusivo", totp(totp_secret)) != 200:
            raise RuntimeError("ERP não aceitou o diretor criado pelo bootstrap")
    finally:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


def main():
    container = "catbrasil-p010-bootstrap-" + secrets.token_hex(5)
    db_password = secrets.token_urlsafe(24)
    created = False
    try:
        command(["docker", "run", "--rm", "--pull", "never", "-d", "--name", container,
                 "-e", "POSTGRES_USER=p010", "-e", "POSTGRES_DB=p010",
                 "-e", "POSTGRES_PASSWORD=" + db_password,
                 "-p", "127.0.0.1::5432", "postgres:16-alpine"])
        created = True
        for _ in range(60):
            ready = subprocess.run(["docker", "exec", container, "psql", "-XAt", "-U", "p010", "-d", "p010", "-c", "SELECT 1"], capture_output=True)
            if ready.returncode == 0:
                break
            time.sleep(0.5)
        else:
            raise RuntimeError("PostgreSQL descartável indisponível")
        port = int(command(["docker", "port", container, "5432/tcp"]).stdout.strip().rsplit(":", 1)[1])
        env = dict(os.environ,
                   DATABASE_URL=f"postgres://p010:{quote(db_password, safe='')}@127.0.0.1:{port}/p010?sslmode=disable",
                   APP_ENV="staging", ADMIN_SECRET_PIN="pin-p010-exclusivo")
        command(["go", "run", "./cmd/migrate", "up"], cwd=BACKEND, env=env)
        before = command(["docker", "exec", container, "psql", "-XAt", "-U", "p010", "-d", "p010", "-c",
                          "SELECT count(*) FROM users; SELECT count(*) FROM transactions;"]).stdout.splitlines()
        if before != ["0", "0"]:
            raise RuntimeError("migração criou contas ou transações fictícias")
        demo_state = command(["docker", "exec", container, "psql", "-XAt", "-U", "p010", "-d", "p010", "-c",
                              "SELECT (SELECT count(*) FROM fleet_catalog), (SELECT count(*) FROM level_configs), (SELECT count(*) FROM roulette_slices), (SELECT count(*) FROM announcements), global_withdrawals_locked, auto_approve_enabled, referral_commission_percent, bonus_per_mutirao_amount FROM system_config WHERE id=1;"]).stdout.strip()
        if demo_state != "0|0|0|0|t|f|0.00|0.00":
            raise RuntimeError("catálogo, prêmios, campanhas ou configuração econômica fictícia persistem")

        totp_secret = interactive_bootstrap(env)
        result = command(["docker", "exec", container, "psql", "-XAt", "-U", "p010", "-d", "p010", "-c",
                          "SELECT count(*), min(role), min(balance), min(yield_balance_available), bool_and(two_factor_enabled), bool_and(password_hash LIKE '$2%') FROM users; SELECT count(*) FROM transactions;"]).stdout.splitlines()
        if result != ["1|diretoria|0.00|0.00|t|t", "0"]:
            raise RuntimeError("diretor inicial não atende papel, bcrypt, 2FA ou saldo zero")

        assert_second_bootstrap_refused(env)
        prove_erp_login(env, totp_secret)

        second = subprocess.run(["go", "run", "./cmd/bootstrap-admin", "--username", "outro_diretor"],
                                cwd=BACKEND, env=env, capture_output=True, text=True)
        if second.returncode == 0 or "terminal interativo obrigatório" not in second.stderr:
            raise RuntimeError("entrada não interativa foi aceita")
        print("P0.10 PASS: migração sem dados econômicos fictícios, bootstrap TTY com bcrypt+2FA e saldos zero; segundo bootstrap e entrada sem TTY recusados; login ERP exige PIN+TOTP")
    finally:
        if created:
            subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError, subprocess.CalledProcessError) as exc:
        detail = str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__
        raise SystemExit(f"P0.10 FALHOU: {detail}") from None
