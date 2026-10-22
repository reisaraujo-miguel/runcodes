#!/usr/bin/env python3
"""Minimal webhook receiver used by the local Alertmanager.

It exists so the smoke test can prove an alert travels from a Prometheus rule
through Alertmanager to a real receiver. It is not an alerting product: it keeps
the last 100 payloads in memory and exposes them for inspection.

GET    /healthz  liveness
GET    /alerts   every payload received, newest first, with receipt time
DELETE /alerts   clear the buffer (the smoke test uses this)
POST   /alerts   Alertmanager webhook endpoint
"""

import json
import os
import threading
from collections import deque
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_PAYLOADS = 100
MAX_BODY_BYTES = 1 << 20

_payloads = deque(maxlen=MAX_PAYLOADS)
_lock = threading.Lock()


def _now():
    return datetime.now(timezone.utc).isoformat()


class Handler(BaseHTTPRequestHandler):
    server_version = "runcodes-alert-sink/1.0"

    def _send_json(self, status, payload):
        body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/healthz":
            self._send_json(200, {"status": "ok"})
            return
        if self.path == "/alerts":
            with _lock:
                payloads = list(_payloads)
            self._send_json(200, {"count": len(payloads), "payloads": payloads})
            return
        self._send_json(404, {"error": "not found"})

    def do_DELETE(self):
        if self.path != "/alerts":
            self._send_json(404, {"error": "not found"})
            return
        with _lock:
            _payloads.clear()
        self._send_json(200, {"status": "cleared"})

    def do_POST(self):
        if self.path != "/alerts":
            self._send_json(404, {"error": "not found"})
            return

        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            self._send_json(400, {"error": "invalid content-length"})
            return
        if length <= 0 or length > MAX_BODY_BYTES:
            self._send_json(400, {"error": "invalid or oversized body"})
            return

        raw = self.rfile.read(length)
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            self._send_json(400, {"error": "body is not valid JSON"})
            return

        entry = {"received_at": _now(), "payload": payload}
        with _lock:
            _payloads.appendleft(entry)

        alerts = payload.get("alerts", [])
        print(
            f"alert-sink: received {len(alerts)} alert(s) "
            f"status={payload.get('status', 'unknown')}",
            flush=True,
        )
        self._send_json(200, {"status": "received", "alerts": len(alerts)})

    def log_message(self, fmt, *args):
        print(f"alert-sink: {self.address_string()} {fmt % args}", flush=True)


def main():
    port = int(os.environ.get("ALERT_SINK_PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    print(f"alert-sink: listening on 0.0.0.0:{port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
