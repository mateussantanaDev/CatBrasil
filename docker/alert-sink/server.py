"""Disposable staging-only alert receiver with an observable delivery count."""

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

count = 0


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path not in ("/health", "/count"):
            self.send_error(404)
            return
        body = b"ok" if self.path == "/health" else json.dumps({"count": count}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json" if self.path == "/count" else "text/plain")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_POST(self):
        global count
        if self.path != "/notify":
            self.send_error(404)
            return
        size = int(self.headers.get("Content-Length", "0"))
        if not 0 < size <= 65536:
            self.send_error(413)
            return
        self.rfile.read(size)
        count += 1
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
