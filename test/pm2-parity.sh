#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PM2GO_BIN="${PM2GO_BIN:-$ROOT_DIR/pm2go}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/test/parity-output}"
RUN_ID="pm2go-parity-$(date +%s)"
APP_NAME="$RUN_ID-app"
FIXTURE="$ROOT_DIR/test/fixtures/test-app.py"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "$1" >&2
    exit 1
  fi
}

cleanup() {
  pm2 delete "$APP_NAME" >/dev/null 2>&1 || true
  "$PM2GO_BIN" delete "$APP_NAME" >/dev/null 2>&1 || true
}

normalize_json() {
  python3 - "$1" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as fh:
    data = json.load(fh)

def clean_process(proc):
    env = proc.get("pm2_env", {})
    monit = proc.get("monit", {})
    return {
        "name": proc.get("name") or env.get("name"),
        "status": env.get("status"),
        "exec_mode": env.get("exec_mode"),
        "script": env.get("pm_exec_path"),
        "cwd": env.get("pm_cwd") or env.get("pm_cwd_path"),
        "out_log": env.get("pm_out_log_path"),
        "err_log": env.get("pm_err_log_path"),
        "memory_type": type(monit.get("memory")).__name__,
        "cpu_type": type(monit.get("cpu")).__name__,
    }

if isinstance(data, list):
    normalized = [clean_process(proc) for proc in data]
else:
    normalized = data

json.dump(normalized, sys.stdout, indent=2, sort_keys=True)
print()
PY
}

run_capture() {
  local label="$1"
  shift
  printf 'running: %s\n' "$label"
  "$@" >"$OUT_DIR/$label.out" 2>"$OUT_DIR/$label.err"
}

main() {
  require_cmd pm2
  require_cmd python3

  if [[ ! -x "$PM2GO_BIN" ]]; then
    (cd "$ROOT_DIR" && go build -o pm2go .)
  fi

  mkdir -p "$OUT_DIR"
  cleanup
  trap cleanup EXIT

  run_capture pm2-start pm2 start python3 --name "$APP_NAME" -- "$FIXTURE" --max-count 20 --interval 1 --message "PM2 parity"
  sleep 2
  run_capture pm2-list pm2 list
  run_capture pm2-jlist pm2 jlist
  normalize_json "$OUT_DIR/pm2-jlist.out" >"$OUT_DIR/pm2-jlist.normalized.json"
  run_capture pm2-logs pm2 logs "$APP_NAME" --lines 5 --nostream
  run_capture pm2-restart pm2 restart "$APP_NAME"
  run_capture pm2-stop pm2 stop "$APP_NAME"
  run_capture pm2-delete pm2 delete "$APP_NAME"

  run_capture pm2go-start "$PM2GO_BIN" start python3 --name "$APP_NAME" -- "$FIXTURE" --max-count 20 --interval 1 --message "PM2 parity"
  sleep 2
  run_capture pm2go-list "$PM2GO_BIN" list
  run_capture pm2go-jlist "$PM2GO_BIN" jlist
  normalize_json "$OUT_DIR/pm2go-jlist.out" >"$OUT_DIR/pm2go-jlist.normalized.json"
  run_capture pm2go-pid "$PM2GO_BIN" pid "$APP_NAME"
  run_capture pm2go-logs "$PM2GO_BIN" logs "$APP_NAME" --lines 5
  run_capture pm2go-restart "$PM2GO_BIN" restart "$APP_NAME"
  run_capture pm2go-stop "$PM2GO_BIN" stop "$APP_NAME"
  run_capture pm2go-delete "$PM2GO_BIN" delete "$APP_NAME"

  printf '\nParity artifacts written to %s\n' "$OUT_DIR"
  printf 'Compare normalized JSON:\n'
  printf '  diff -u %s/pm2-jlist.normalized.json %s/pm2go-jlist.normalized.json\n' "$OUT_DIR" "$OUT_DIR"
}

main "$@"
