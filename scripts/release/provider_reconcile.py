#!/usr/bin/env python3
"""Compare two operator-normalized, complete financial snapshots offline.

This is NOT an official SuitPay integration. An operator must independently
obtain the provider statement and map its fields to the documented contract.
Both inputs must cover exactly the same scope and snapshot cutoff. The tool
cannot prove that either supplied file is complete or authentic.

CSV headers (JSON accepts an array of objects with the same keys):
  internal: transaction_id,direction,external_id,amount,status
  provider: external_id,direction,amount,status,internal_ref

direction: deposit | payout
status: pending | settled | failed | reversed
amount: decimal string with at most two fractional digits, BRL; provider
        always positive; internal deposit positive and payout negative.
        For an internal payout, export -transactions.net_amount: this is the
        net amount actually sent to the provider. Never export the gross
        wallet debit (transactions.valor), which also includes the fee.
internal_ref may be empty in the provider statement, but when supplied must
match transaction_id. A record is identified by (direction, external_id).
No implicit status conversion is performed: the export operator must map
source-specific statuses after reviewing their meaning.

Usage: python3 scripts/release/provider_reconcile.py --internal internal.csv \
  --provider statement.csv --scope '2026-10-20T00:00Z..2026-10-21T00:00Z' \
  --report private/provider_reconciliation.json

Exit codes: 0 exact match; 1 mismatch/missing/empty scope; 2 invalid input.
The report contains raw transaction IDs and must be handled as private data.
"""

import argparse
import csv
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
import hashlib
import json
import os
from pathlib import Path
import re
import sys


FIELDS = {
    "internal": ("transaction_id", "direction", "external_id", "amount", "status"),
    "provider": ("external_id", "direction", "amount", "status", "internal_ref"),
}
STATUSES = {"pending", "settled", "failed", "reversed"}
DIRECTIONS = {"deposit", "payout"}
AMOUNT_RE = re.compile(r"^(?:0|[1-9][0-9]*)(?:\.[0-9]{1,2})?$")
INTERNAL_AMOUNT_RE = re.compile(r"^-?(?:0|[1-9][0-9]*)(?:\.[0-9]{1,2})?$")
MAX_CENTS = 999_999_999_999_999  # NUMERIC(15,2), same as backend/internal/money.MaxCents.


class InputError(ValueError):
    pass


def _no_duplicate_json_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise InputError("JSON contém chave duplicada")
        result[key] = value
    return result


def read_rows(path: Path, kind: str):
    if not path.is_file():
        raise InputError(f"Arquivo {kind} inexistente")
    if path.suffix.lower() == ".csv":
        with path.open("r", encoding="utf-8-sig", newline="") as handle:
            reader = csv.DictReader(handle, strict=True)
            if reader.fieldnames is None or len(reader.fieldnames) != len(set(reader.fieldnames)):
                raise InputError(f"Cabeçalho CSV inválido em {kind}")
            if set(reader.fieldnames) != set(FIELDS[kind]):
                raise InputError(f"Colunas CSV inválidas em {kind}; esperado: {','.join(FIELDS[kind])}")
            rows = list(reader)
    elif path.suffix.lower() == ".json":
        with path.open("r", encoding="utf-8-sig") as handle:
            rows = json.load(handle, parse_float=Decimal, object_pairs_hook=_no_duplicate_json_keys)
        if not isinstance(rows, list):
            raise InputError(f"JSON de {kind} deve ser uma lista")
    else:
        raise InputError(f"Formato de {kind} deve ser .csv ou .json")
    return rows


def cents(value, kind: str, direction: str):
    if not isinstance(value, str):
        raise InputError("Montante deve ser texto decimal, jamais número JSON")
    pattern = INTERNAL_AMOUNT_RE if kind == "internal" else AMOUNT_RE
    if not pattern.fullmatch(value):
        raise InputError("Montante inválido; use decimal BRL com até duas casas")
    try:
        amount = Decimal(value)
        units = int(amount * 100)
    except (InvalidOperation, OverflowError, ValueError):
        raise InputError("Montante decimal inválido") from None
    if units == 0 or abs(units) > MAX_CENTS:
        raise InputError("Montante deve ser não zero e caber em NUMERIC(15,2)")
    if kind == "provider" and units < 0:
        raise InputError("Montante do provedor deve ser positivo")
    if kind == "internal" and (units < 0) != (direction == "payout"):
        raise InputError("Depósito interno deve ser positivo e saque interno negativo")
    return abs(units)


def normalize(rows, kind: str):
    indexed = {}
    internal_ids = set()
    for position, row in enumerate(rows, 1):
        if not isinstance(row, dict) or set(row) != set(FIELDS[kind]):
            raise InputError(f"Linha {position} de {kind} tem colunas inválidas")
        if any(not isinstance(value, str) for value in row.values()):
            raise InputError(f"Linha {position} de {kind} contém campo não textual")
        direction = row["direction"]
        status = row["status"]
        external_id = row["external_id"]
        if direction not in DIRECTIONS or status not in STATUSES:
            raise InputError(f"Linha {position} de {kind} tem direção/status inválido")
        if not external_id or external_id != external_id.strip() or len(external_id) > 255:
            raise InputError(f"Linha {position} de {kind} tem external_id inválido")
        if kind == "internal":
            tx_id = row["transaction_id"]
            if not tx_id or tx_id != tx_id.strip() or len(tx_id) > 64:
                raise InputError(f"Linha {position} de {kind} tem transaction_id inválido")
            if tx_id in internal_ids:
                raise InputError("transaction_id interno duplicado")
            internal_ids.add(tx_id)
        else:
            ref = row["internal_ref"]
            if ref != ref.strip() or len(ref) > 64:
                raise InputError(f"Linha {position} de {kind} tem internal_ref inválido")
        key = (direction, external_id)
        if key in indexed:
            raise InputError(f"Identificador externo duplicado em {kind}")
        indexed[key] = {
            "amount_cents": cents(row["amount"], kind, direction),
            "status": status,
            "internal_ref": row["transaction_id"] if kind == "internal" else row["internal_ref"],
        }
    return indexed


def compare(internal, provider):
    issues = []
    for key in sorted(set(internal) | set(provider)):
        local = internal.get(key)
        external = provider.get(key)
        identity = {"direction": key[0], "external_id": key[1]}
        if local is None or external is None:
            issues.append({**identity, "issue": "missing_internal" if local is None else "missing_provider"})
            continue
        for field in ("amount_cents", "status"):
            if local[field] != external[field]:
                issues.append({**identity, "issue": f"{field}_mismatch", "internal": local[field], "provider": external[field]})
        if external["internal_ref"] and external["internal_ref"] != local["internal_ref"]:
            issues.append({**identity, "issue": "internal_ref_mismatch", "internal": local["internal_ref"], "provider": external["internal_ref"]})
    return issues


def file_hash(path):
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def write_private_report(path, report):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
        json.dump(report, handle, ensure_ascii=False, indent=2)
        handle.write("\n")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--internal", type=Path, required=True)
    parser.add_argument("--provider", type=Path, required=True)
    parser.add_argument("--scope", required=True, help="Descrição explícita do mesmo período/corte de ambos os arquivos")
    parser.add_argument("--report", type=Path, required=True, help="Novo arquivo JSON privado; jamais sobrescrito")
    args = parser.parse_args(argv)
    try:
        if not args.scope.strip() or len(args.scope) > 500:
            raise InputError("Escopo é obrigatório e deve ser descritivo")
        if args.internal.resolve() == args.provider.resolve():
            raise InputError("Arquivos interno e provedor devem ser distintos")
        if args.report.resolve() in (args.internal.resolve(), args.provider.resolve()):
            raise InputError("Relatório não pode sobrescrever arquivos de entrada")
        internal = normalize(read_rows(args.internal, "internal"), "internal")
        provider = normalize(read_rows(args.provider, "provider"), "provider")
        issues = compare(internal, provider)
        if not internal or not provider:
            issues.append({"issue": "empty_scope"})
        report = {
            "tool": "provider_reconcile_v1",
            "generated_at": datetime.now(timezone.utc).isoformat(),
            "scope_operator_attested": args.scope,
            "limitations": "Arquivos normalizados pelo operador; autenticidade, completude e semântica do extrato não são verificadas por esta ferramenta. Não é homologação oficial SuitPay.",
            "inputs": {
                "internal_sha256": file_hash(args.internal),
                "provider_sha256": file_hash(args.provider),
                "internal_count": len(internal),
                "provider_count": len(provider),
            },
            "result": "mismatch" if issues else "match",
            "issues": issues,
        }
        write_private_report(args.report, report)
    except (InputError, OSError, csv.Error, json.JSONDecodeError, UnicodeError) as exc:
        print(f"Entrada inválida ou relatório indisponível: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 2
    print(f"Conciliação offline: {report['result']} | interno={len(internal)} | provedor={len(provider)} | divergências={len(issues)} | relatório={args.report}")
    return 1 if issues else 0


if __name__ == "__main__":
    raise SystemExit(main())
