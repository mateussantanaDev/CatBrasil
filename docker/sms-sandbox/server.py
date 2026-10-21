"""Local SMS sink for isolated staging. Never route real user traffic here."""

import hmac
import json
import os
import re
import threading
import time
from collections import deque
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit


SEND_TOKEN = os.environ.get("SMS_GATEWAY_TOKEN", "")
INSPECT_TOKEN = os.environ.get("SMS_SANDBOX_INSPECT_TOKEN", "")
if os.environ.get("APP_ENV") != "staging":
    raise SystemExit("sms-sandbox só pode iniciar com APP_ENV=staging")
if min(len(SEND_TOKEN), len(INSPECT_TOKEN)) < 32 or SEND_TOKEN == INSPECT_TOKEN:
    raise SystemExit("tokens SMS sandbox distintos e com 32+ caracteres são obrigatórios")

messages = deque(maxlen=200)
lock = threading.Lock()
code_pattern = re.compile(r"(?<!\d)\d{6}(?!\d)")
phone_pattern = re.compile(r"^\+55\d{10,11}$")


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        # Não incluir conteúdo da mensagem, código SMS ou Authorization em logs.
        pass

    def respond(self, status, body):
        data = json.dumps(body, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def authorized(self, token):
        candidate = self.headers.get("Authorization", "")
        return hmac.compare_digest(candidate, "Bearer " + token)

    def do_GET(self):
        parsed = urlsplit(self.path)
        if parsed.path == "/health":
            return self.respond(200, {"status": "ok"})
        if parsed.path != "/messages":
            return self.respond(404, {"error": "not_found"})
        if not self.authorized(INSPECT_TOKEN):
            return self.respond(401, {"error": "unauthorized"})
        phone = parse_qs(parsed.query).get("phone", [""])[0]
        if not phone_pattern.fullmatch(phone):
            return self.respond(400, {"error": "invalid_phone"})
        with lock:
            now = time.time()
            result = [dict(item) for item in reversed(messages)
                      if item["phone"] == phone and now - item["timestamp"] <= 1200]
        for item in result:
            item.pop("timestamp")
        return self.respond(200, {"messages": result})

    def do_POST(self):
        if self.path != "/send":
            return self.respond(404, {"error": "not_found"})
        if not self.authorized(SEND_TOKEN):
            return self.respond(401, {"error": "unauthorized"})
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            return self.respond(400, {"error": "invalid_body"})
        if not 0 < length <= 4096:
            return self.respond(413, {"error": "body_limit"})
        try:
            data = json.loads(self.rfile.read(length))
            phone, message = data["phone"], data["message"]
        except (ValueError, KeyError, TypeError):
            return self.respond(400, {"error": "invalid_json"})
        if not isinstance(phone, str) or not phone_pattern.fullmatch(phone):
            return self.respond(400, {"error": "invalid_phone"})
        if not isinstance(message, str) or not 0 < len(message) <= 500:
            return self.respond(400, {"error": "invalid_message"})
        match = code_pattern.search(message)
        item = {
            "phone": phone,
            "message": message,
            "code": match.group() if match else "",
            "createdAt": datetime.now(timezone.utc).isoformat(),
            "timestamp": time.time(),
        }
        with lock:
            messages.append(item)
        print(json.dumps({"event": "sms_captured", "phoneSuffix": phone[-4:]}), flush=True)
        return self.respond(202, {"accepted": True})


ThreadingHTTPServer(("0.0.0.0", int(os.environ.get("PORT", "8080"))), Handler).serve_forever()
