#!/usr/bin/env python3
"""CLI stdlib read-only do sandbox oficial SuitPay (sem rede, sem docker, sem segredos na saída).

preflight: validação estática do .env; NUNCA afirma credenciais, IP, TLS ou webhook entregue.
evidence: consistência local das evidências; NÃO chama 'pass'/homologado (consistência != autenticidade).
Saída JSON apenas com 'status' e 'errors' (códigos estáticos); exit 0 ou 2.
"""

import argparse
import importlib.util
import ipaddress
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlparse

PROVIDER = "suitpay-official-sandbox"
CALLBACK_PATH = "/api/v1/webhooks/suitpay"

# Status local exato do domínio (backend/internal/domain/transaction.go).
LOCAL_SUCESSO = "sucesso"
LOCAL_CANCELADO = "cancelado"
LOCAL_PROCESSANDO = "processando"

# typeTransaction do provedor: cash-in PIX x cash-out PIX_CASHOUT.
PROVIDER_TYPE_CASHIN = "PIX"
PROVIDER_TYPE_CASHOUT = "PIX_CASHOUT"
# DOCUMENT_VALIDATE é resposta SÍNCRONA da API no envio, nunca callback.
DELIVERY_CALLBACK = "callback"
DELIVERY_SYNC = "sync_response"

# SC-02/SC-03/SC-04 são a MESMA transação (paid -> replay -> chargeback):
# compartilham request_id e provider_tx_id (idTransaction/requestNumber já
# liquidados). Vínculos divergentes entre eles são inconsistência financeira.
CASHIN_LINKED = ("cashin_paid", "cashin_duplicate", "cashin_chargeback")

# Semântica exata por cenário (provider_status/type, entrega, status local e
# efeito no razão):
#   amount     -> ledger_delta_cents == +amount_cents (crédito de depósito)
#   neg_amount -> ledger_delta_cents == -amount_cents (estorno/chargeback)
#   zero       -> ledger_delta_cents == 0 (replay, liquidação de saque, recusa
#                 síncrona sem estorno automático)
#   gross      -> ledger_delta_cents == gross_reserved_cents, com
#                 gross_reserved_cents >= amount_cents (webhook traz o líquido;
#                 o razão estorna o valor BRUTO reservado)
SCENARIOS = {
    "cashin_paid": {
        "provider_status": "PAID_OUT", "provider_type": PROVIDER_TYPE_CASHIN,
        "delivery": DELIVERY_CALLBACK, "local_status": LOCAL_SUCESSO,
        "ledger": "amount", "provider_tx": True,
    },
    "cashin_duplicate": {
        "provider_status": "PAID_OUT", "provider_type": PROVIDER_TYPE_CASHIN,
        "delivery": DELIVERY_CALLBACK, "local_status": LOCAL_SUCESSO,
        "ledger": "zero", "provider_tx": True,
    },
    "cashin_chargeback": {
        "provider_status": "CHARGEBACK", "provider_type": PROVIDER_TYPE_CASHIN,
        "delivery": DELIVERY_CALLBACK, "local_status": LOCAL_CANCELADO,
        "ledger": "neg_amount", "provider_tx": True,
    },
    "cashout_paid": {
        "provider_status": "PAID_OUT", "provider_type": PROVIDER_TYPE_CASHOUT,
        "delivery": DELIVERY_CALLBACK, "local_status": LOCAL_SUCESSO,
        "ledger": "zero", "provider_tx": True,
    },
    "cashout_canceled": {
        "provider_status": "CANCELED", "provider_type": PROVIDER_TYPE_CASHOUT,
        "delivery": DELIVERY_CALLBACK, "local_status": LOCAL_CANCELADO,
        "ledger": "gross", "provider_tx": True,
    },
    "cashout_document_mismatch": {
        "provider_status": "DOCUMENT_VALIDATE", "provider_type": PROVIDER_TYPE_CASHOUT,
        "delivery": DELIVERY_SYNC, "local_status": LOCAL_PROCESSANDO,
        "ledger": "zero", "provider_tx": False,
    },
}
REQUIRED = tuple(SCENARIOS)
STR_FIELDS = ("provider_ref", "request_id", "provider_status",
              "provider_type", "delivery", "local_status", "evidence_ref")
UNIQUE_FIELDS = ("evidence_ref",)
FQDN_RE = re.compile(r"^(?=.{1,253}$)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$")
_CACHE = {}


def _helper(name):
    """Import dinâmico opcional do validador Compose (apenas definições; nunca docker/rede)."""
    if "mod" not in _CACHE:
        try:
            path = Path(__file__).with_name("validate_suitpay_sandbox_compose.py")
            spec = importlib.util.spec_from_file_location("_suitpay_sb_compose", path)
            mod = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(mod)
            _CACHE["mod"] = mod
        except Exception:
            _CACHE["mod"] = None
    mod = _CACHE["mod"]
    return getattr(mod, name, None) if mod else None


def _parse_env(path):
    parser = _helper("parse_env_file")
    if parser:
        return parser(path)
    env = {}
    with open(path, "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                key, val = line.split("=", 1)
                env[key.strip()] = val.strip().strip("'\"")
    return env


def _is_local(host):
    host = (host or "").strip().lower()
    fn = _helper("is_localhost_or_private_host")
    if (fn and fn(host)) or not host or host in ("localhost", "127.0.0.1", "::1", "0.0.0.0"):
        return True
    return host.endswith((".localhost", ".local", ".invalid", ".test"))


def _is_ip(host):
    try:
        ipaddress.ip_address(host)
        return True
    except ValueError:
        return False


def _fqdn_ok(host):
    host = (host or "").strip().lower()
    return bool(FQDN_RE.fullmatch(host)) and not _is_local(host)


def _check_callback(url, api_domain):
    errs = []
    try:
        parsed = urlparse(url)
        port = parsed.port
    except ValueError:
        return ["E_CALLBACK_MALFORMED"]
    if parsed.scheme != "https":
        errs.append("E_CALLBACK_NOT_HTTPS")
    if parsed.username or parsed.password:
        errs.append("E_CALLBACK_HAS_USERINFO")
    if parsed.query:
        errs.append("E_CALLBACK_HAS_QUERY")
    if parsed.fragment:
        errs.append("E_CALLBACK_HAS_FRAGMENT")
    if port is not None:
        errs.append("E_CALLBACK_HAS_PORT")
    host = parsed.hostname or ""
    if _is_ip(host):
        errs.append("E_CALLBACK_IP_NOT_ALLOWED")
    elif not _fqdn_ok(host):
        errs.append("E_CALLBACK_HOST_NOT_PUBLIC")
    if parsed.path != CALLBACK_PATH:
        errs.append("E_CALLBACK_PATH_MISMATCH")
    if api_domain and url != "https://" + api_domain + CALLBACK_PATH:
        errs.append("E_CALLBACK_NOT_EXACT")
    return errs


def preflight(env_file):
    try:
        env = _parse_env(env_file)
    except Exception:
        return ["E_ENV_FILE_UNREADABLE"]
    errs = []
    if env.get("APP_ENV", "").strip() != "staging":
        errs.append("E_APP_ENV_NOT_STAGING")
    if env.get("PIX_MODE", "").strip() != "suitpay":
        errs.append("E_PIX_MODE_NOT_SUITPAY")
    if env.get("PIX_SANDBOX_URL", "").strip():
        errs.append("E_PIX_SANDBOX_URL_NOT_EMPTY")
    if env.get("SUITPAY_LIVE_ENABLED", "").strip() != "false":
        errs.append("E_LIVE_ENABLED_NOT_FALSE")
    api_domain = env.get("API_DOMAIN", "").strip()
    callback = env.get("SUITPAY_CALLBACK_URL", "").strip()
    if not api_domain:
        errs.append("E_API_DOMAIN_MISSING")
    elif not _fqdn_ok(api_domain):
        errs.append("E_API_DOMAIN_NOT_PUBLIC_FQDN")
    if not callback:
        errs.append("E_CALLBACK_MISSING")
    else:
        errs.extend(_check_callback(callback, api_domain))
    return errs


def _check_item(name, entry):
    spec = SCENARIOS[name]
    errs = []
    for field in STR_FIELDS:
        value = entry.get(field)
        if not isinstance(value, str) or not value.strip():
            errs.append("E_" + field.upper() + "_EMPTY")
    # provider_tx_id é obrigatório, exceto no mismatch documental síncrono, que
    # pode não ter idTransaction porque a transferência nunca foi aceita.
    if spec["provider_tx"]:
        ptx = entry.get("provider_tx_id")
        if not isinstance(ptx, str) or not ptx.strip():
            errs.append("E_PROVIDER_TX_ID_EMPTY")
    amount = entry.get("amount_cents")
    delta = entry.get("ledger_delta_cents")
    if not isinstance(amount, int) or isinstance(amount, bool) or amount <= 0:
        errs.append("E_AMOUNT_CENTS_INVALID")
        amount = None
    if not isinstance(delta, int) or isinstance(delta, bool):
        errs.append("E_LEDGER_DELTA_CENTS_INVALID")
        delta = None
    for field, expected in (("provider_status", spec["provider_status"]),
                            ("provider_type", spec["provider_type"]),
                            ("delivery", spec["delivery"]),
                            ("local_status", spec["local_status"])):
        value = entry.get(field)
        if isinstance(value, str) and value.strip() and value.strip() != expected:
            errs.append("E_" + field.upper() + "_UNEXPECTED")
    if spec["ledger"] == "gross":
        gross = entry.get("gross_reserved_cents")
        if not isinstance(gross, int) or isinstance(gross, bool) or gross <= 0:
            errs.append("E_GROSS_RESERVED_CENTS_INVALID")
            gross = None
        if gross is not None and amount is not None and gross < amount:
            errs.append("E_GROSS_BELOW_NET")
        if gross is not None and delta is not None and delta != gross:
            errs.append("E_LEDGER_DELTA_MISMATCH")
    elif amount is not None and delta is not None:
        sign = {"amount": 1, "neg_amount": -1, "zero": 0}[spec["ledger"]]
        if delta != sign * amount:
            errs.append("E_LEDGER_DELTA_MISMATCH")
    return errs


def evidence(manifest):
    try:
        data = json.loads(Path(manifest).read_text(encoding="utf-8"))
    except Exception:
        return ["E_MANIFEST_UNREADABLE"]
    if not isinstance(data, dict):
        return ["E_MANIFEST_NOT_OBJECT"]
    errs = []
    if data.get("provider") != PROVIDER:
        errs.append("E_PROVIDER_MISMATCH")
    run_id = data.get("run_id")
    if not isinstance(run_id, str) or not run_id.strip():
        errs.append("E_RUN_ID_MISSING")
    raw = data.get("scenarios")
    if not isinstance(raw, list):
        errs.append("E_SCENARIOS_NOT_LIST")
        raw = []
    items = {}
    seen = {}
    for entry in raw:
        if not isinstance(entry, dict) or entry.get("scenario") not in SCENARIOS:
            errs.append("E_SCENARIO_UNKNOWN")
            continue
        name = entry["scenario"]
        if name in items:
            errs.append("E_SCENARIO_DUPLICATED")
            continue
        items[name] = entry
        errs.extend(_check_item(name, entry))
        for field in UNIQUE_FIELDS:
            value = entry.get(field)
            if isinstance(value, str) and value.strip():
                used = seen.setdefault(field, set())
                if value in used:
                    errs.append("E_DUPLICATE_" + field.upper())
                else:
                    used.add(value)
    # cash-in paid/replay/chargeback são a mesma transação: request_id e
    # provider_tx_id precisam coincidir (vínculo não-único entre eles).
    linkage = []
    for name in CASHIN_LINKED:
        entry = items.get(name)
        if entry is None:
            continue
        req = entry.get("request_id")
        ptx = entry.get("provider_tx_id")
        if isinstance(req, str) and req.strip() and isinstance(ptx, str) and ptx.strip():
            linkage.append((req.strip(), ptx.strip()))
    if len(set(linkage)) > 1:
        errs.append("E_CASHIN_LINKAGE_MISMATCH")
    for name in REQUIRED:
        if name not in items:
            errs.append("E_SCENARIO_MISSING")
    return list(dict.fromkeys(errs))


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="SuitPay sandbox oficial: preflight e evidence (read-only, stdlib)."
    )
    sub = parser.add_subparsers(dest="cmd", required=True)
    pre = sub.add_parser("preflight")
    pre.add_argument("--env-file", required=True, type=Path)
    evi = sub.add_parser("evidence")
    evi.add_argument("--manifest", required=True, type=Path)
    args = parser.parse_args(argv)
    if args.cmd == "preflight":
        errors = preflight(args.env_file)
        status = "ready" if not errors else "blocked"
    else:
        errors = evidence(args.manifest)
        status = "consistent" if not errors else "incomplete"
    print(json.dumps({"status": status, "errors": errors}, ensure_ascii=False))
    return 0 if not errors else 2


if __name__ == "__main__":
    sys.exit(main())
