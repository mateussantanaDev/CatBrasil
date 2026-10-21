"""Local SuitPay-compatible response stub for isolated staging only."""

import hashlib
import hmac
import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit


CLIENT_ID = os.environ.get("PIX_SANDBOX_CLIENT_ID", "")
CLIENT_SECRET = os.environ.get("PIX_SANDBOX_CLIENT_SECRET", "")
INSPECT_TOKEN = os.environ.get("PIX_SANDBOX_INSPECT_TOKEN", "")
if os.environ.get("APP_ENV") != "staging":
    raise SystemExit("pix-sandbox só pode iniciar com APP_ENV=staging")
if not CLIENT_ID or min(len(CLIENT_SECRET), len(INSPECT_TOKEN)) < 32:
    raise SystemExit("credenciais PIX sandbox e token de inspeção são obrigatórios")

transactions = {}
lock = threading.Lock()


def transaction_id(seed):
    return "STG-" + hashlib.sha256(seed.encode()).hexdigest()[:20].upper()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def respond(self, status, body):
        data = json.dumps(body, separators=(",", ":")).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        path = urlsplit(self.path).path
        if path == "/health":
            return self.respond(200, {"status": "ok"})
        if path != "/transactions":
            return self.respond(404, {"error": "not_found"})
        if not hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + INSPECT_TOKEN):
            return self.respond(401, {"error": "unauthorized"})
        with lock:
            return self.respond(200, {"transactions": list(transactions.values())})

    def do_POST(self):
        path = urlsplit(self.path).path
        if path not in ("/api/v1/gateway/request-qrcode", "/api/v1/gateway/pix-payment"):
            return self.respond(404, {"error": "not_found"})
        if not (hmac.compare_digest(self.headers.get("ci", ""), CLIENT_ID)
                and hmac.compare_digest(self.headers.get("cs", ""), CLIENT_SECRET)):
            return self.respond(401, {"response": "ERROR", "message": "invalid_credentials"})
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            return self.respond(400, {"response": "ERROR", "message": "invalid_body"})
        if not 0 < length <= 8192:
            return self.respond(413, {"response": "ERROR", "message": "body_limit"})
        try:
            data = json.loads(self.rfile.read(length))
        except (ValueError, TypeError):
            return self.respond(400, {"response": "ERROR", "message": "invalid_json"})
        if not isinstance(data, dict):
            return self.respond(400, {"response": "ERROR", "message": "invalid_json"})

        if path.endswith("request-qrcode"):
            request_number = data.get("requestNumber")
            amount = data.get("amount")
            if not isinstance(request_number, str) or not request_number or not isinstance(amount, (int, float)) or amount <= 0:
                return self.respond(400, {"response": "ERROR", "message": "invalid_payment"})
            client = data.get("client")
            if not isinstance(client, dict):
                return self.respond(400, {"response": "ERROR", "message": "client_required"})
            document = client.get("document")
            name = client.get("name")
            email = client.get("email")
            if (not isinstance(document, str) or len(document) != 11 or not document.isdigit()
                    or len(set(document)) == 1 or not isinstance(name, str) or len(name.split()) < 2
                    or not isinstance(email, str) or "@" not in email):
                return self.respond(400, {"response": "ERROR", "message": "client_identity_invalid"})
            for size in (9, 10):
                check = (sum(int(document[i]) * (size + 1 - i) for i in range(size)) * 10) % 11
                if int(document[size]) != (0 if check == 10 else check):
                    return self.respond(400, {"response": "ERROR", "message": "client_identity_invalid"})
            tx_id = transaction_id("deposit:" + request_number)
            item = {"idTransaction": tx_id, "requestNumber": request_number, "value": amount, "type": "deposit", "status": "PENDING"}
            with lock:
                previous = transactions.get(tx_id)
                if previous is not None and previous != item:
                    return self.respond(409, {"response": "ERROR", "message": "idempotency_conflict"})
                transactions[tx_id] = item
                if len(transactions) > 2000:
                    transactions.pop(next(iter(transactions)))
            # String de teste deliberadamente inválida como PIX real.
            return self.respond(200, {"response": "OK", "idTransaction": tx_id,
                                      "paymentCode": "STAGING-ONLY-" + tx_id})

        external_id = data.get("externalId")
        value = data.get("value")
        key = data.get("key")
        if not isinstance(external_id, str) or not external_id or not isinstance(value, (int, float)) or value <= 0 or not isinstance(key, str) or not key:
            return self.respond(400, {"response": "ERROR", "message": "invalid_transfer"})
        tx_id = transaction_id("payout:" + external_id)
        item = {"idTransaction": tx_id, "externalId": external_id, "value": value, "type": "payout", "status": "PROCESSING"}
        with lock:
            previous = transactions.get(tx_id)
            if previous is not None and previous != item:
                return self.respond(409, {"response": "ERROR", "message": "idempotency_conflict"})
            transactions[tx_id] = item
            if len(transactions) > 2000:
                transactions.pop(next(iter(transactions)))
        return self.respond(200, {"response": "OK", "idTransaction": tx_id, "status": "PROCESSING"})


ThreadingHTTPServer(("0.0.0.0", int(os.environ.get("PORT", "8080"))), Handler).serve_forever()
