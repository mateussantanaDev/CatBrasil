"""Small private-network Alertmanager webhook to ntfy relay.

Only alert names and severities are forwarded. Alert annotations and labels may
contain customer information, so they must never enter notification text/logs.
"""

import json
import os
import re
import socket
from dataclasses import dataclass
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


MAX_BODY_BYTES = 65536
MAX_ALERTS = 100
MAX_DISPLAYED_ALERTS = 20
SAFE_NAME = re.compile(r"[^A-Za-z0-9_. -]")
SEVERITIES = {"critical", "warning", "info"}


@dataclass(frozen=True)
class Config:
    ntfy_url: str
    ntfy_token: str = ""
    app_env: str = "development"

    def validate(self):
        if not self.ntfy_url:
            raise ValueError("NTFY_URL is required")
        url = urlsplit(self.ntfy_url)
        if url.scheme not in ("https", "http") or not url.netloc or not url.hostname:
            raise ValueError("NTFY_URL must be an absolute HTTP(S) URL")
        if url.username or url.password or url.fragment:
            raise ValueError("NTFY_URL cannot contain userinfo or fragment")
        if self.app_env == "production" and url.scheme != "https":
            raise ValueError("NTFY_URL must use HTTPS in production")


def _safe_name(value):
    if not isinstance(value, str):
        return "unknown"
    return (SAFE_NAME.sub("", value)[:64].strip() or "unknown")


def format_message(payload):
    if not isinstance(payload, dict) or payload.get("version") != "4":
        raise ValueError("invalid Alertmanager webhook version")
    alerts = payload.get("alerts")
    if not isinstance(alerts, list) or not 1 <= len(alerts) <= MAX_ALERTS:
        raise ValueError("invalid alert count")
    lines = []
    critical = False
    for alert in alerts:
        if not isinstance(alert, dict) or alert.get("status") not in ("firing", "resolved"):
            raise ValueError("invalid alert")
        labels = alert.get("labels")
        if not isinstance(labels, dict):
            raise ValueError("invalid labels")
        severity = labels.get("severity", "info")
        if severity not in SEVERITIES:
            severity = "info"
        if severity == "critical" and alert["status"] == "firing":
            critical = True
        if len(lines) < MAX_DISPLAYED_ALERTS:
            lines.append(f"{alert['status']}: {_safe_name(labels.get('alertname'))} ({severity})")
    if len(alerts) > len(lines):
        lines.append(f"+{len(alerts) - len(lines)} more alerts")
    status = payload.get("status")
    if status not in ("firing", "resolved"):
        raise ValueError("invalid group status")
    title = f"CatBrasil: {len(alerts)} {status} alert(s)"
    priority = "5" if critical else "3"
    return title, "\n".join(lines).encode("utf-8"), priority


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        # A redirect could expose the bearer token to another host or HTTP.
        return None


def deliver(config, title, body, priority):
    headers = {
        "Content-Type": "text/plain; charset=utf-8",
        "Title": title,
        "Priority": priority,
        "Tags": "rotating_light" if priority == "5" else "warning",
    }
    if config.ntfy_token:
        headers["Authorization"] = "Bearer " + config.ntfy_token
    request = Request(config.ntfy_url, data=body, headers=headers, method="POST")
    with build_opener(NoRedirect()).open(request, timeout=5) as response:
        if not 200 <= response.status < 300:
            raise URLError("notification sink rejected request")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        # HTTP request paths and headers can contain secrets; never log them.
        pass

    def respond(self, status, result):
        data = json.dumps({"status": result}, separators=(",", ":")).encode("ascii")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/healthz":
            return self.respond(200, "alive")
        if self.path == "/readyz":
            try:
                self.server.config.validate()
            except ValueError:
                return self.respond(503, "unready")
            return self.respond(200, "ready")
        return self.respond(404, "not_found")

    def do_POST(self):
        if self.path != "/alertmanager":
            return self.respond(404, "not_found")
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            return self.respond(400, "invalid_body")
        if not 0 < length <= MAX_BODY_BYTES:
            return self.respond(413, "body_limit")
        self.connection.settimeout(5)
        try:
            payload = json.loads(self.rfile.read(length))
            title, body, priority = format_message(payload)
        except (ValueError, UnicodeDecodeError, socket.timeout):
            return self.respond(400, "invalid_payload")
        try:
            deliver(self.server.config, title, body, priority)
        except HTTPError as exc:
            exc.close()
            print('{"event":"notification_delivery_failed"}', flush=True)
            return self.respond(502, "delivery_failed")
        except (URLError, TimeoutError, OSError):
            # Alertmanager retries on 5xx. Never log the destination or token.
            print('{"event":"notification_delivery_failed"}', flush=True)
            return self.respond(502, "delivery_failed")
        print('{"event":"notification_delivered"}', flush=True)
        return self.respond(202, "accepted")


def main():
    config = Config(
        ntfy_url=os.environ.get("NTFY_URL", ""),
        ntfy_token=os.environ.get("NTFY_TOKEN", ""),
        app_env=os.environ.get("APP_ENV", "development"),
    )
    try:
        config.validate()
    except ValueError as exc:
        raise SystemExit(str(exc)) from None
    server = ThreadingHTTPServer(("0.0.0.0", int(os.environ.get("PORT", "8080"))), Handler)
    server.config = config
    server.serve_forever()


if __name__ == "__main__":
    main()
