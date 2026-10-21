#!/usr/bin/env python3
"""Carga limitada de leitura para staging (respeita o rate limit global da API)."""

import argparse
import concurrent.futures
import json
import math
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


def sample(url):
    started = time.monotonic()
    try:
        with urlopen(Request(url, headers={"Accept": "application/json"}), timeout=5) as response:
            body = response.read(4096)
            status = response.status
            if status == 200:
                try:
                    if json.loads(body).get("status") != "ready":
                        status = 598
                except (ValueError, AttributeError):
                    status = 598
    except HTTPError as exc:
        status = exc.code
    except (OSError, URLError):
        status = 0
    return status, (time.monotonic() - started) * 1000


def percentile(sorted_values, p):
    if not sorted_values:
        return 0
    return sorted_values[max(0, math.ceil(len(sorted_values) * p) - 1)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", required=True)
    parser.add_argument("--confirm-staging", required=True, action="store_true")
    parser.add_argument("--rps", type=float, default=2.0, help="Máximo 4 requisições/s")
    parser.add_argument("--duration", type=int, default=30, help="Segundos; máximo 90")
    parser.add_argument("--p95-limit-ms", type=float, default=1000)
    parser.add_argument("--error-limit-pct", type=float, default=1.0)
    parser.add_argument("--output", help="Arquivo JSON de resultados")
    args = parser.parse_args()
    parsed = urlparse(args.base_url)
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        parser.error("--base-url precisa ser URL absoluta")
    local = parsed.hostname in ("127.0.0.1", "localhost", "::1") or (parsed.hostname or "").endswith(".localhost")
    if parsed.scheme != "https" and not local:
        parser.error("HTTPS é obrigatório fora de localhost")
    if not (0 < args.rps <= 4 and 1 <= args.duration <= 90):
        parser.error("limites: 0 < rps <= 4 e 1 <= duration <= 90")
    if args.p95_limit_ms <= 0 or not (0 <= args.error_limit_pct <= 100):
        parser.error("limites inválidos")

    url = args.base_url.rstrip("/") + "/ready"
    n = math.floor(args.rps * args.duration)
    if n < 1:
        parser.error("rps × duration precisa gerar ao menos uma requisição")
    started = time.monotonic()
    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        futures = []
        for i in range(n):
            target = started + i / args.rps
            delay = target - time.monotonic()
            if delay > 0:
                time.sleep(delay)
            futures.append(pool.submit(sample, url))
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())

    latencies = sorted(latency for _, latency in results)
    statuses = {}
    for status, _ in results:
        statuses[str(status)] = statuses.get(str(status), 0) + 1
    errors = sum(count for status, count in statuses.items() if status != "200")
    report = {
        "endpoint": "/ready", "requests": n, "targetRps": args.rps,
        "elapsedSeconds": round(time.monotonic() - started, 3),
        "statuses": statuses, "errorPct": round(100 * errors / n, 2),
        "latencyMs": {"p50": round(percentile(latencies, .50), 2),
                      "p95": round(percentile(latencies, .95), 2),
                      "p99": round(percentile(latencies, .99), 2)},
        "limits": {"p95Ms": args.p95_limit_ms, "errorPct": args.error_limit_pct},
    }
    report["passed"] = (report["errorPct"] <= args.error_limit_pct and
                        report["latencyMs"]["p95"] <= args.p95_limit_ms)
    encoded = json.dumps(report, indent=2) + "\n"
    print(encoded, end="")
    if args.output:
        with open(args.output, "x", encoding="utf-8") as output:
            output.write(encoded)
    return 0 if report["passed"] else 1


if __name__ == "__main__":
    sys.exit(main())
