"""Relay tests use a loopback-only notification sink."""

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import server as relay


class Sink(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def do_POST(self):
        self.server.requests.append({
            "body": self.rfile.read(int(self.headers["Content-Length"])),
            "headers": self.headers,
        })
        self.send_response(self.server.response_status)
        self.send_header("Content-Length", "0")
        self.end_headers()


def sample_payload():
    return {
        "version": "4",
        "status": "firing",
        "alerts": [{
            "status": "firing",
            "labels": {"alertname": "GatewayErrors", "severity": "critical", "customer": "SECRET-CUSTOMER"},
            "annotations": {"description": "SECRET-PAYMENT-DATA"},
        }],
    }


class RelayTest(unittest.TestCase):
    def setUp(self):
        self.sink = ThreadingHTTPServer(("127.0.0.1", 0), Sink)
        self.sink.requests = []
        self.sink.response_status = 200
        self.sink_thread = threading.Thread(target=self.sink.serve_forever, daemon=True)
        self.sink_thread.start()
        self.config = relay.Config(
            ntfy_url=f"http://127.0.0.1:{self.sink.server_port}/catbrasil",
            ntfy_token="SECRET-TOKEN",
            app_env="staging",
        )
        self.relay = ThreadingHTTPServer(("127.0.0.1", 0), relay.Handler)
        self.relay.config = self.config
        self.relay_thread = threading.Thread(target=self.relay.serve_forever, daemon=True)
        self.relay_thread.start()
        self.base = f"http://127.0.0.1:{self.relay.server_port}"

    def tearDown(self):
        self.relay.shutdown()
        self.relay.server_close()
        self.sink.shutdown()
        self.sink.server_close()
        self.relay_thread.join(timeout=2)
        self.sink_thread.join(timeout=2)

    def post(self, payload):
        body = json.dumps(payload).encode()
        request = Request(
            self.base + "/alertmanager",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        return urlopen(request, timeout=3)

    def test_local_delivery_filters_sensitive_data_and_sends_token(self):
        self.assertEqual(urlopen(self.base + "/healthz").status, 200)
        self.assertEqual(urlopen(self.base + "/readyz").status, 200)
        self.assertEqual(self.post(sample_payload()).status, 202)
        self.assertEqual(len(self.sink.requests), 1)
        sent = self.sink.requests[0]
        self.assertEqual(sent["headers"]["Authorization"], "Bearer SECRET-TOKEN")
        self.assertEqual(sent["headers"]["Priority"], "5")
        self.assertIn(b"GatewayErrors", sent["body"])
        self.assertNotIn(b"SECRET-CUSTOMER", sent["body"])
        self.assertNotIn(b"SECRET-PAYMENT-DATA", sent["body"])

    def test_sink_failure_is_retryable_and_invalid_payload_is_rejected(self):
        self.sink.response_status = 503
        with self.assertRaises(HTTPError) as error:
            self.post(sample_payload())
        self.assertEqual(error.exception.code, 502)
        error.exception.close()
        self.assertEqual(len(self.sink.requests), 1)
        with self.assertRaises(HTTPError) as error:
            self.post({"version": "4", "status": "firing", "alerts": []})
        self.assertEqual(error.exception.code, 400)
        error.exception.close()
        self.assertEqual(len(self.sink.requests), 1)

    def test_body_limit(self):
        request = Request(self.base + "/alertmanager", data=b"x" * (relay.MAX_BODY_BYTES + 1), method="POST")
        with self.assertRaises(HTTPError) as error:
            urlopen(request, timeout=3)
        self.assertEqual(error.exception.code, 413)
        error.exception.close()

    def test_production_requires_https(self):
        with self.assertRaisesRegex(ValueError, "HTTPS"):
            relay.Config("http://127.0.0.1:8080/topic", app_env="production").validate()
        relay.Config("https://notify.example.org/topic", app_env="production").validate()
        with self.assertRaisesRegex(ValueError, "required"):
            relay.Config("").validate()

    def test_message_is_bounded(self):
        payload = sample_payload()
        payload["alerts"] = payload["alerts"] * relay.MAX_ALERTS
        title, body, priority = relay.format_message(payload)
        self.assertLess(len(body), 2000)
        self.assertIn(b"+80 more alerts", body)
        self.assertEqual(priority, "5")


if __name__ == "__main__":
    unittest.main()
