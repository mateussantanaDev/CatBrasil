#!/usr/bin/env python3
"""Smoke de staging. Muta dados apenas com --full-sandbox explícito."""

import argparse
import atexit
import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import struct
import subprocess
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlparse
from urllib.request import Request, urlopen


class SmokeFailure(Exception):
    pass


def request(base, path, method="GET", payload=None, token=None, expected=(200,)):
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    body = None
    if payload is not None:
        headers["Content-Type"] = "application/json"
        body = json.dumps(payload).encode()
    req = Request(base.rstrip("/") + path, data=body, headers=headers, method=method)
    try:
        with urlopen(req, timeout=8) as response:
            status = response.status
            raw = response.read(65536)
    except HTTPError as exc:
        status = exc.code
        raw = exc.read(65536)
    except (OSError, URLError) as exc:
        raise SmokeFailure(f"{method} {path}: conexão falhou ({type(exc).__name__})") from exc
    if status not in expected:
        error_code = ""
        try:
            error_code = json.loads(raw).get("errorCode", "")
        except (ValueError, AttributeError):
            pass
        raise SmokeFailure(f"{method} {path}: HTTP {status}, esperado {expected}; {error_code}")
    try:
        return status, json.loads(raw)
    except ValueError:
        return status, raw


def check(condition, message):
    if not condition:
        raise SmokeFailure(message)
    print("OK:", message)


def totp(secret):
    key = base64.b32decode(secret.upper() + "=" * (-len(secret) % 8))
    counter = struct.pack(">Q", int(time.time()) // 30)
    digest = hmac.new(key, counter, hashlib.sha1).digest()
    offset = digest[-1] & 15
    number = (struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF) % 1000000
    return f"{number:06d}"


def synthetic_cpf():
    """CPF matematicamente válido para banco isolado; não representa KYC real."""
    digits = [secrets.randbelow(10) for _ in range(9)]
    if len(set(digits)) == 1:
        digits[0] = (digits[0] + 1) % 10
    for size in (9, 10):
        check_digit = (sum(digits[i] * (size + 1 - i) for i in range(size)) * 10) % 11
        digits.append(0 if check_digit == 10 else check_digit)
    return "".join(map(str, digits))


def latest_sms_code(inbox_url, inbox_token, phone):
    delimiter = "&" if "?" in inbox_url else "?"
    url = inbox_url + delimiter + urlencode({"phone": phone})
    headers = {"Authorization": "Bearer " + inbox_token}
    req = Request(url, headers=headers)
    for _ in range(15):
        try:
            with urlopen(req, timeout=5) as response:
                if response.status != 200:
                    raise SmokeFailure(f"Inbox sandbox respondeu HTTP {response.status}")
                data = json.load(response)
        except HTTPError as exc:
            raise SmokeFailure(f"Inbox sandbox respondeu HTTP {exc.code}") from exc
        except (OSError, ValueError) as exc:
            raise SmokeFailure("Inbox sandbox indisponível ou JSON inválido") from exc
        messages = data.get("messages", [])
        if messages:
            code = messages[0].get("code", "")
            if not code:
                match = re.search(r"\b\d{6}\b", messages[0].get("message", ""))
                code = match.group(0) if match else ""
            if re.fullmatch(r"\d{6}", code):
                return code
        time.sleep(1)
    raise SmokeFailure("Nenhum código chegou à inbox sandbox em 15 segundos")


def ensure_safe_url(url, label):
    parsed = urlparse(url)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        raise SmokeFailure(f"{label} deve ser URL HTTP(S) absoluta")
    local = parsed.hostname in ("127.0.0.1", "localhost", "::1") or (parsed.hostname or "").endswith(".localhost")
    if parsed.scheme != "https" and not local:
        raise SmokeFailure(f"{label} precisa usar HTTPS fora de localhost")


def bootstrap_staging_director(base, target_username, expected_user_id, expected_cpf,
                               evidence_ref, inbox_url, inbox_token):
    root = Path(__file__).resolve().parents[2]
    command = ["docker", "compose", "-p", "catbrasil-staging", "--env-file", ".staging.env",
               "-f", "docker-compose.yml", "-f", "compose.staging.yml"]

    def run(args, stdin=None):
        try:
            return subprocess.run(command + args, cwd=root, input=stdin, text=True,
                                  capture_output=True, check=True, timeout=30).stdout
        except (subprocess.CalledProcessError, OSError, subprocess.TimeoutExpired) as exc:
            raise SmokeFailure(f"Bootstrap ERP staging falhou ({type(exc).__name__})") from exc

    try:
        config = json.loads(run(["config", "--format", "json"]))
        api_env = config["services"]["backend-api"]["environment"]
        pix_env = config["services"]["pix-sandbox"]["environment"]
        admin_pin = api_env["ADMIN_SECRET_PIN"]
        client_id = pix_env["PIX_SANDBOX_CLIENT_ID"]
        client_secret = pix_env["PIX_SANDBOX_CLIENT_SECRET"]
    except (KeyError, ValueError, TypeError) as exc:
        raise SmokeFailure("Compose de staging incompleto para bootstrap ERP") from exc
    if api_env.get("APP_ENV") != "staging" or api_env.get("PIX_MODE") != "suitpay":
        raise SmokeFailure("Bootstrap ERP restrito a APP_ENV=staging e PIX_MODE=suitpay")
    if api_env.get("PIX_SANDBOX_URL") != "http://pix-sandbox:8080/api/v1/gateway":
        raise SmokeFailure("PIX_SANDBOX_URL não aponta para o stub interno")
    if not all((admin_pin, client_id, client_secret)):
        raise SmokeFailure("PIN e credenciais do stub PIX ausentes no Compose")

    container_id = run(["ps", "-q", "postgres"]).strip()
    if not container_id:
        raise SmokeFailure("PostgreSQL de staging não está em execução")
    try:
        label = subprocess.run(
            ["docker", "inspect", "-f", '{{ index .Config.Labels "com.docker.compose.project" }}', container_id],
            capture_output=True, text=True, check=True, timeout=10).stdout.strip()
    except (subprocess.CalledProcessError, OSError, subprocess.TimeoutExpired) as exc:
        raise SmokeFailure("Não foi possível confirmar o projeto do PostgreSQL") from exc
    if label != "catbrasil-staging":
        raise SmokeFailure("PostgreSQL não pertence ao projeto catbrasil-staging")
    if not re.fullmatch(r"release_[0-9a-f]{12}", target_username):
        raise SmokeFailure("Usuário alvo não foi criado por este smoke")
    if not isinstance(expected_user_id, str) or not re.fullmatch(r"CB-[0-9A-F]{16}", expected_user_id):
        raise SmokeFailure("ID de usuário sintético inválido para bootstrap")
    if not evidence_ref or len(evidence_ref) > 120:
        raise SmokeFailure("STAGING_IDENTITY_EVIDENCE_REF obrigatório (até 120 caracteres)")

    # O revisor deve ser uma segunda conta real no fluxo de staging, distinta do alvo.
    reviewer_username = "release_reviewer_" + secrets.token_hex(6)
    reviewer_password = secrets.token_urlsafe(24)
    reviewer_phone = "+55119" + "".join(str(secrets.randbelow(10)) for _ in range(8))
    _, sms = request(base, "/api/v1/auth/send-sms", "POST", {"phone": reviewer_phone})
    check(isinstance(sms, dict) and sms.get("success") is True and "code" not in sms,
          "SMS da conta revisora aceito sem vazar código")
    reviewer_sms_code = latest_sms_code(inbox_url, inbox_token, reviewer_phone)
    _, reviewer_registration = request(base, "/api/v1/auth/register", "POST", {
        "username": reviewer_username, "password": reviewer_password,
        "phone": reviewer_phone, "smsCode": reviewer_sms_code,
        "cpf": synthetic_cpf(), "legalName": "Pessoa Revisora Sintetica",
        "email": reviewer_username + "@example.invalid",
        "fingerprint": "release-reviewer-" + reviewer_username,
    }, expected=(201,))
    reviewer_id = reviewer_registration.get("userId") if isinstance(reviewer_registration, dict) else None
    reviewer_token = reviewer_registration.get("token") if isinstance(reviewer_registration, dict) else None
    check(bool(reviewer_token) and isinstance(reviewer_id, str) and
          re.fullmatch(r"CB-[0-9A-F]{16}", reviewer_id) and reviewer_id != expected_user_id,
          "segunda conta sintética criada para revisão")
    _, reviewer_setup = request(base, "/api/v1/auth/2fa/setup", "POST", {}, token=reviewer_token)
    reviewer_secret = reviewer_setup.get("secret") if isinstance(reviewer_setup, dict) else None
    check(bool(reviewer_secret), "2FA da conta revisora iniciado")
    request(base, "/api/v1/auth/2fa/verify", "POST",
            {"secret": reviewer_secret, "code": totp(reviewer_secret)}, token=reviewer_token)
    check(True, "2FA da conta revisora ativado")

    sql = ("WITH promoted AS (UPDATE users SET role='diretoria', updated_at=NOW() "
           f"WHERE username='{reviewer_username}' AND id='{reviewer_id}' AND role='user' RETURNING id) "
           "SELECT id FROM promoted;\n")
    updated_id = run(["exec", "-T", "postgres", "sh", "-c",
                      'exec psql -XAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], sql).strip()
    if updated_id != reviewer_id:
        raise SmokeFailure("Promoção a diretoria não alterou exatamente a conta revisora")
    check(True, "revisor distinto promovido a diretoria somente no staging")
    demoted = False

    def demote():
        nonlocal demoted
        if demoted:
            return
        sql = ("WITH demoted AS (UPDATE users SET role='user', updated_at=NOW() "
               f"WHERE username='{reviewer_username}' AND id='{reviewer_id}' AND role='diretoria' RETURNING id) "
               "SELECT id FROM demoted;\n")
        result = run(["exec", "-T", "postgres", "sh", "-c",
                      'exec psql -XAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'], sql).strip()
        if result != reviewer_id:
            raise SmokeFailure("Falha ao remover cargo diretoria da conta revisora")
        demoted = True

    atexit.register(demote)
    try:
        _, erp = request(base, "/api/v1/admin/auth/login", "POST", {
            "username": reviewer_username, "password": reviewer_password, "pin": admin_pin,
            "twoFactorCode": totp(reviewer_secret),
        })
        token = erp.get("data", {}).get("token") if isinstance(erp, dict) else None
        check(bool(token), "login ERP com PIN e TOTP")
        _, profile = request(base, "/api/v1/admin/profile", token=token)
        check(isinstance(profile, dict), "perfil ERP autenticado")
        _, inspected = request(base, f"/api/v1/admin/users/{expected_user_id}/identity/inspect", "POST", {
            "twoFactorCode": totp(reviewer_secret),
        }, token=token)
        check(isinstance(inspected, dict) and inspected.get("cpf") == expected_cpf and
              inspected.get("legalName") == "Pessoa Teste Sintetica" and
              inspected.get("identityStatus") == "pending",
              "identidade sintética inspecionada com TOTP")
        _, review = request(base, f"/api/v1/admin/users/{expected_user_id}/identity", "PATCH", {
            "status": "verified", "evidenceRef": evidence_ref,
            "twoFactorCode": totp(reviewer_secret),
        }, token=token)
        check(isinstance(review, dict) and review.get("identityStatus") == "verified",
              "revisão sintética aprovada via diretoria com TOTP")
        _, result = request(base, "/api/v1/admin/config", token=token)
        current = result.get("data", {}) if isinstance(result, dict) else {}
        gw = current.get("gateway", {})
        if not isinstance(gw, dict):
            raise SmokeFailure("Configuração ERP incompleta")
        _, saved = request(base, "/api/v1/admin/config/gateway", "PUT", {
            "name": "PIX staging local", "depositFeePercent": gw.get("depositFeePercent", 0),
            "withdrawFeeFixed": gw.get("withdrawFeeFixed", 0),
            "pixExpirationMinutes": gw.get("pixExpirationMinutes", 15),
            "collectionGraceMinutes": current.get("collectionGraceMinutes", 10),
            "suitpayClientId": client_id, "suitpayClientSecret": client_secret,
            "suitpaySandbox": True, "suitpayEnabled": True,
            "suitpayCallbackUrl": "http://backend-api:8080/api/v1/webhooks/suitpay",
        }, token=token)
        check(isinstance(saved, dict) and saved.get("success") is True,
              "gateway PIX do ERP aponta para credenciais sandbox")
        _, verified = request(base, "/api/v1/admin/config", token=token)
        configured = verified.get("data", {}).get("gateway", {})
        check(configured.get("suitpayEnabled") is True and
              configured.get("suitpaySandbox") is True and
              configured.get("suitpayClientIdConfigured") is True and
              configured.get("suitpayClientSecretConfigured") is True and
              "suitpayClientId" not in configured and
              "suitpayClientSecret" not in configured,
              "configuração PIX sandbox confirmada pela API")
    finally:
        demote()
        atexit.unregister(demote)
    check(True, "cargo diretoria removido da conta revisora")
    request(base, "/api/v1/admin/profile", token=token, expected=(403,))
    check(True, "JWT ERP anterior bloqueado após remoção do cargo")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True, help="Origem pública da API de staging")
    parser.add_argument("--client-url", help="Origem do frontend cliente")
    parser.add_argument("--admin-url", help="Origem do frontend ERP")
    parser.add_argument("--confirm-staging", action="store_true", required=True,
                        help="Confirma que as URLs são de staging")
    parser.add_argument("--full-sandbox", action="store_true",
                        help="Executa cadastro, login, PIN e 2FA com SMS sandbox")
    parser.add_argument("--pix-sandbox", action="store_true",
                        help="Cria cobrança de R$1 no stub PIX local, após verificar configuração")
    parser.add_argument("--bootstrap-erp", action="store_true",
                        help="Revisa identidade sintética por diretoria no staging e configura ERP sandbox")
    parser.add_argument("--manual-review-timeout", type=int, default=0,
                        help="Aguarda até N segundos por revisão humana; requer --pix-sandbox")
    args = parser.parse_args()
    if args.pix_sandbox and not args.full_sandbox:
        parser.error("--pix-sandbox exige --full-sandbox")
    if args.bootstrap_erp and not args.pix_sandbox:
        parser.error("--bootstrap-erp exige --pix-sandbox")
    if not 0 <= args.manual_review_timeout <= 600:
        parser.error("--manual-review-timeout deve ficar entre 0 e 600 segundos")
    if args.manual_review_timeout and (not args.pix_sandbox or args.bootstrap_erp):
        parser.error("--manual-review-timeout exige --pix-sandbox sem --bootstrap-erp")
    if args.pix_sandbox and not args.bootstrap_erp and not args.manual_review_timeout:
        parser.error("--pix-sandbox exige --bootstrap-erp ou --manual-review-timeout")
    evidence_ref = os.environ.get("STAGING_IDENTITY_EVIDENCE_REF", "")
    if args.bootstrap_erp and not evidence_ref:
        parser.error("--bootstrap-erp exige STAGING_IDENTITY_EVIDENCE_REF")
    for label, url in (("base-url", args.base_url), ("client-url", args.client_url),
                       ("admin-url", args.admin_url)):
        if url:
            ensure_safe_url(url, label)

    _, health = request(args.base_url, "/health")
    check(isinstance(health, dict) and health.get("status") == "online", "API liveness")
    _, ready = request(args.base_url, "/ready")
    check(isinstance(ready, dict) and ready.get("status") == "ready", "API readiness e schema")
    for label, url in (("Cliente", args.client_url), ("ERP", args.admin_url)):
        if url:
            request(url, "/healthz")
            check(True, f"{label} healthz")
    request(args.base_url, "/api/v1/user", expected=(401,))
    check(True, "rota de usuário bloqueia anônimo")
    request(args.base_url, "/api/v1/admin/profile", expected=(401,))
    check(True, "ERP bloqueia anônimo")
    request(args.base_url, "/api/v1/dev/simulate-webhook", method="POST", payload={}, expected=(404, 405))
    check(True, "simulação pública indisponível")
    request(args.base_url, "/api/v1/webhooks/pix", method="POST",
            payload={"txId": "smoke-invalid", "status": "CONFIRMED", "paidAmount": 1}, expected=(401,))
    check(True, "webhook PIX sem assinatura rejeitado")

    if not args.full_sandbox:
        print("Smoke sem mutações concluído. Use --full-sandbox para cadastro e 2FA.")
        return

    inbox_url = os.environ.get("SMS_SANDBOX_INBOX_URL", "")
    inbox_token = os.environ.get("SMS_SANDBOX_INSPECT_TOKEN", "")
    if not inbox_url or not inbox_token:
        raise SmokeFailure("Defina SMS_SANDBOX_INBOX_URL e SMS_SANDBOX_INSPECT_TOKEN")
    ensure_safe_url(inbox_url, "SMS_SANDBOX_INBOX_URL")
    if urlparse(inbox_url).hostname not in ("127.0.0.1", "localhost", "::1"):
        raise SmokeFailure("Inbox sandbox deve estar acessível apenas por localhost")
    phone = os.environ.get("STAGING_TEST_PHONE") or "+55119" + "".join(str(secrets.randbelow(10)) for _ in range(8))
    username = "release_" + secrets.token_hex(6)
    cpf = synthetic_cpf()
    email = username + "@example.invalid"
    password = secrets.token_urlsafe(24)
    pin = "".join(str(secrets.randbelow(10)) for _ in range(6))
    _, sms = request(args.base_url, "/api/v1/auth/send-sms", "POST", {"phone": phone})
    check(isinstance(sms, dict) and sms.get("success") is True and "code" not in sms,
          "SMS aceito sem vazar código pela API")
    code = latest_sms_code(inbox_url, inbox_token, phone)
    _, registration = request(args.base_url, "/api/v1/auth/register", "POST", {
        "username": username, "password": password, "phone": phone,
        "cpf": cpf, "legalName": "Pessoa Teste Sintetica", "email": email,
        "smsCode": code, "fingerprint": "release-smoke-" + username,
    }, expected=(201,))
    check(isinstance(registration, dict) and registration.get("token"), "cadastro com SMS sandbox")
    _, login = request(args.base_url, "/api/v1/auth/login", "POST",
                       {"username": username, "password": password})
    token = login.get("token") if isinstance(login, dict) else None
    check(bool(token), "login do usuário de teste")
    _, profile = request(args.base_url, "/api/v1/user/profile", token=token)
    check(isinstance(profile, dict) and profile.get("username") == username, "perfil autenticado")
    check(profile.get("identityStatus") == "pending", "identidade criada como pendente")
    _, pending_pix = request(args.base_url, "/api/v1/financial/deposit/pix", "POST",
                             {"amount": 1.00}, token=token, expected=(403,))
    check(isinstance(pending_pix, dict) and pending_pix.get("errorCode") == "IDENTITY_NOT_VERIFIED",
          "PIX bloqueado enquanto identidade está pendente")
    request(args.base_url, "/api/v1/financial/transactions", token=token)
    check(True, "extrato autenticado")
    request(args.base_url, "/api/v1/user/security-password", "POST",
            {"password": pin, "accountPassword": password}, token=token)
    check(True, "PIN financeiro inicial")
    _, setup = request(args.base_url, "/api/v1/auth/2fa/setup", "POST", {}, token=token)
    secret = setup.get("secret") if isinstance(setup, dict) else None
    check(bool(secret), "segredo 2FA emitido")
    request(args.base_url, "/api/v1/auth/2fa/verify", "POST",
            {"secret": secret, "code": totp(secret)}, token=token)
    check(True, "2FA ativado")
    request(args.base_url, "/api/v1/auth/login", "POST",
            {"username": username, "password": password}, expected=(401,))
    check(True, "login sem TOTP rejeitado")
    _, login2 = request(args.base_url, "/api/v1/auth/login", "POST",
                        {"username": username, "password": password, "twoFactorCode": totp(secret)})
    check(isinstance(login2, dict) and login2.get("token"), "login com TOTP")
    if args.bootstrap_erp:
        bootstrap_staging_director(args.base_url, username, registration.get("userId"),
                                   cpf, evidence_ref, inbox_url, inbox_token)
    if args.pix_sandbox:
        if args.manual_review_timeout:
            print("Aguardando revisão manual de identidade sintética no staging para userId:",
                  registration.get("userId"))
            deadline = time.monotonic() + args.manual_review_timeout
            while time.monotonic() < deadline:
                _, identity_profile = request(args.base_url, "/api/v1/user/profile",
                                              token=login2["token"])
                if identity_profile.get("identityStatus") == "verified":
                    break
                time.sleep(5)
        _, identity_profile = request(args.base_url, "/api/v1/user/profile", token=login2["token"])
        check(isinstance(identity_profile, dict) and identity_profile.get("identityStatus") == "verified",
              "identidade verificada antes da cobrança PIX")
        pix_url = os.environ.get("PIX_SANDBOX_INSPECT_URL", "")
        pix_token = os.environ.get("PIX_SANDBOX_INSPECT_TOKEN", "")
        if not pix_url or not pix_token:
            raise SmokeFailure("Defina PIX_SANDBOX_INSPECT_URL e PIX_SANDBOX_INSPECT_TOKEN")
        if not args.bootstrap_erp and os.environ.get("STAGING_GATEWAY_CONFIG_VERIFIED") != "1":
            raise SmokeFailure("Sem --bootstrap-erp, defina STAGING_GATEWAY_CONFIG_VERIFIED=1 após conferir o ERP")
        ensure_safe_url(pix_url, "PIX_SANDBOX_INSPECT_URL")
        if urlparse(pix_url).hostname not in ("127.0.0.1", "localhost", "::1"):
            raise SmokeFailure("Inspeção PIX sandbox deve estar acessível apenas por localhost")
        _, stub_health = request(pix_url, "/health")
        check(isinstance(stub_health, dict) and stub_health.get("status") == "ok",
              "stub PIX local disponível")
        request(pix_url, "/transactions", token=pix_token)
        _, pix = request(args.base_url, "/api/v1/financial/deposit/pix", "POST",
                         {"amount": 1.00}, token=login2["token"])
        tx_id = pix.get("txId") if isinstance(pix, dict) else None
        check(bool(tx_id) and str(pix.get("pixCode", "")).startswith("STAGING-ONLY-"),
              "cobrança gerada pelo stub PIX local")
        _, entries = request(pix_url, "/transactions", token=pix_token)
        check(isinstance(entries, dict) and any(
            item.get("requestNumber") == tx_id and item.get("type") == "deposit"
            for item in entries.get("transactions", [])), "cobrança registrada no stub PIX")
    print("Smoke sandbox completo. Usuário sintético criado:", username)


if __name__ == "__main__":
    try:
        main()
    except SmokeFailure as exc:
        print("FALHA:", exc, file=sys.stderr)
        sys.exit(1)
