#!/usr/bin/env python3
"""EFP browser bridge 验证用本地服务（原型，只用于验证，不是产品代码）。

验证目标：
  1. https 的 Portal 页面能否调用 127.0.0.1 上的本地程序（CORS + 私有网络访问预检）。
  2. 本地程序能否把命令转给 tools 仓库的 browser CLI 并把 JSON 结果带回页面。

安全边界（验证期也要守住）：
  - 只监听 127.0.0.1。
  - 只接受 Origin 等于 --portal-origin 的请求。
  - /run 必须带 X-EFP-Bridge-Token，值为启动时打印的一次性 token。
  - /run 只放行 browser CLI 的只读或页面交互子命令，不经过 shell。

用法：
  python efp_bridge_probe.py --portal-origin https://portal.example.com --browser-exe C:\\path\\to\\browser.exe
"""
from __future__ import annotations

import argparse
import json
import os
import secrets
import subprocess
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs

# browser CLI 里允许被页面触发的一级子命令。bookmark 写操作、session stop 等不放行。
ALLOWED_FIRST_ARGS = {"version", "commands", "schema", "tab", "page", "session", "frame"}
DENIED_PAIRS = {("session", "stop"), ("session", "start")}
RUN_TIMEOUT_SECONDS = 60


class Config:
    portal_origin = ""
    browser_exe = "browser"
    token = ""


JOBS: dict[str, dict] = {}
JOBS_LOCK = threading.Lock()
JOB_DELAY_SECONDS = 1.5


def _job_args_allowed(args: list[str]) -> bool:
    if not args or args[0] not in ALLOWED_FIRST_ARGS:
        return False
    if len(args) > 1 and (args[0], args[1]) in DENIED_PAIRS:
        return False
    return True


def _exec_browser(args: list[str]) -> tuple[int, dict]:
    """运行 browser CLI，返回 (http_status, payload)。不经过 shell。"""
    if "--json" not in args:
        args = [*args, "--json"]
    cmd = [Config.browser_exe, *args]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=RUN_TIMEOUT_SECONDS, shell=False)
    except FileNotFoundError:
        return 500, {"ok": False, "error": {"code": "browser_exe_not_found", "browser_exe": Config.browser_exe}}
    except subprocess.TimeoutExpired:
        return 504, {"ok": False, "error": {"code": "timeout", "seconds": RUN_TIMEOUT_SECONDS}}
    stdout = proc.stdout.strip()
    try:
        parsed = json.loads(stdout) if stdout else None
    except ValueError:
        parsed = None
    return 200, {
        "ok": proc.returncode == 0,
        "exit_code": proc.returncode,
        "args": args,
        "result": parsed,
        "stdout": None if parsed is not None else stdout[-4000:],
        "stderr": proc.stderr.strip()[-2000:],
    }


def _run_job(job_id: str, args: list[str]) -> None:
    with JOBS_LOCK:
        JOBS[job_id] = {"state": "running", "args": args}
    status, payload = _exec_browser(args)
    with JOBS_LOCK:
        JOBS[job_id] = {"state": "done", "args": args, "http_status": status, "payload": payload}


def _json_bytes(payload: dict) -> bytes:
    return json.dumps(payload, ensure_ascii=False).encode("utf-8")


class Handler(BaseHTTPRequestHandler):
    server_version = "efp-bridge-probe/0.1"

    # ---- helpers -------------------------------------------------------
    def _origin_ok(self) -> bool:
        return self.headers.get("Origin", "") == Config.portal_origin

    def _cors_headers(self) -> None:
        self.send_header("Access-Control-Allow-Origin", Config.portal_origin)
        self.send_header("Vary", "Origin")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, X-EFP-Bridge-Token")
        # Chrome 私有网络访问预检要求的头；新版 Local Network Access 用的头名不同，两个都给。
        self.send_header("Access-Control-Allow-Private-Network", "true")
        self.send_header("Access-Control-Allow-Local-Network", "true")
        self.send_header("Access-Control-Max-Age", "600")

    def _reply(self, status: int, payload: dict) -> None:
        body = _json_bytes(payload)
        self.send_response(status)
        self._cors_headers()
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _read_json(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length > 0 else b""
        if not raw:
            return {}
        data = json.loads(raw.decode("utf-8"))
        return data if isinstance(data, dict) else {}

    # ---- routes --------------------------------------------------------
    def do_OPTIONS(self) -> None:  # noqa: N802
        if not self._origin_ok():
            self.send_response(403)
            self.end_headers()
            return
        self.send_response(204)
        self._cors_headers()
        self.end_headers()

    def do_GET(self) -> None:  # noqa: N802
        if not self._origin_ok():
            self._reply(403, {"ok": False, "error": {"code": "origin_denied", "origin": self.headers.get("Origin", "")}})
            return
        path, _, query = self.path.partition("?")
        if path == "/ping":
            self._reply(200, {"ok": True, "data": {"bridge": "efp-bridge-probe", "version": "0.1", "browser_exe": Config.browser_exe}})
            return
        if path == "/run":
            # GET 形式只给验证脚本用：browser page fetch 只能发不带自定义头的 GET，
            # 所以 token 和 args 走查询串，args 用逗号分隔，例如 ?token=...&args=tab,list
            params = parse_qs(query)
            token = (params.get("token") or [""])[0]
            if token != Config.token:
                self._reply(401, {"ok": False, "error": {"code": "token_mismatch"}})
                return
            raw_args = (params.get("args") or [""])[0]
            args = [a for a in raw_args.split(",") if a]
            job_id = (params.get("job") or [""])[0]
            if job_id:
                # 异步模式：验证脚本用 browser page fetch 触发本请求时，外层 CLI 命令正握着 default
                # 会话锁，若在此同步再调 browser 会得到 409 busy。所以先应答，延迟执行，结果放到
                # /jobs/<id>，由验证脚本直接轮询。真实产品里页面自己发 fetch，不存在这个嵌套。
                if not _job_args_allowed(args):
                    self._reply(403, {"ok": False, "error": {"code": "command_not_allowed", "args": args}})
                    return
                with JOBS_LOCK:
                    JOBS[job_id] = {"state": "queued", "args": args}
                threading.Timer(JOB_DELAY_SECONDS, _run_job, args=(job_id, args)).start()
                self._reply(202, {"ok": True, "data": {"job": job_id, "state": "queued"}})
                return
            self._run_and_reply(args)
            return
        if path.startswith("/jobs/"):
            job_id = path[len("/jobs/"):]
            with JOBS_LOCK:
                job = JOBS.get(job_id)
            if job is None:
                self._reply(404, {"ok": False, "error": {"code": "job_not_found"}})
                return
            self._reply(200, {"ok": True, "data": job})
            return
        self._reply(404, {"ok": False, "error": {"code": "not_found"}})

    def do_POST(self) -> None:  # noqa: N802
        if not self._origin_ok():
            self._reply(403, {"ok": False, "error": {"code": "origin_denied", "origin": self.headers.get("Origin", "")}})
            return
        if self.path.split("?", 1)[0] != "/run":
            self._reply(404, {"ok": False, "error": {"code": "not_found"}})
            return
        if self.headers.get("X-EFP-Bridge-Token", "") != Config.token:
            self._reply(401, {"ok": False, "error": {"code": "token_mismatch"}})
            return
        try:
            data = self._read_json()
        except (ValueError, UnicodeDecodeError) as exc:
            self._reply(400, {"ok": False, "error": {"code": "bad_json", "message": str(exc)}})
            return
        args = data.get("args")
        if not isinstance(args, list) or not all(isinstance(a, str) for a in args):
            self._reply(400, {"ok": False, "error": {"code": "bad_args", "hint": "body must be {\"args\": [\"tab\", \"list\"]}"}})
            return
        self._run_and_reply(args)

    def _run_and_reply(self, args: list[str]) -> None:
        if not args:
            self._reply(400, {"ok": False, "error": {"code": "bad_args", "hint": "args must not be empty"}})
            return
        if not _job_args_allowed(args):
            self._reply(403, {"ok": False, "error": {"code": "command_not_allowed", "args": args}})
            return
        status, payload = _exec_browser(args)
        self._reply(status, payload)

    def log_message(self, fmt: str, *args) -> None:  # noqa: D401
        sys.stderr.write("[probe] %s - %s\n" % (self.address_string(), fmt % args))


def main() -> int:
    # Windows 控制台默认 cp936/cp1252，中文提示会触发 UnicodeEncodeError；统一按 UTF-8 输出。
    for stream in (sys.stdout, sys.stderr):
        if hasattr(stream, "reconfigure"):
            stream.reconfigure(encoding="utf-8", errors="replace", line_buffering=True)
    parser = argparse.ArgumentParser(description="EFP browser bridge local probe server")
    parser.add_argument("--portal-origin", required=True, help="Portal 页面的 origin，例如 https://portal.example.com（无路径、无尾斜杠）")
    parser.add_argument("--browser-exe", default=os.environ.get("EFP_BROWSER_EXE", "browser"), help="tools 仓库 browser CLI 的可执行文件路径")
    parser.add_argument("--port", type=int, default=8765)
    parser.add_argument("--token", default="", help="固定 token（验证脚本用）；留空则随机生成并打印")
    args = parser.parse_args()

    Config.portal_origin = args.portal_origin.rstrip("/")
    Config.browser_exe = args.browser_exe
    Config.token = args.token or secrets.token_urlsafe(24)

    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    print("efp-bridge-probe listening on http://127.0.0.1:%d" % args.port)
    print("allowed origin : %s" % Config.portal_origin)
    print("browser exe    : %s" % Config.browser_exe)
    print("bridge token   : %s" % Config.token)
    print("在 Portal 页面的 DevTools console 里执行（把 TOKEN 换成上面的值）:")
    print('  await fetch("http://127.0.0.1:%d/ping").then(r => r.json())' % args.port)
    print('  await fetch("http://127.0.0.1:%d/run", {method:"POST", headers:{"Content-Type":"application/json","X-EFP-Bridge-Token":"TOKEN"}, body: JSON.stringify({args:["tab","list"]})}).then(r => r.json())' % args.port)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
