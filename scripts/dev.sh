#!/usr/bin/env bash
# Local control plane for golang-api and nodejs-api.
# Usage: ./scripts/dev.sh <setup|start|stop|status|logs|test> [golang|nodejs|all]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="${ROOT}/.run"
COMPOSE=(docker compose --project-directory "${ROOT}" -f "${ROOT}/docker-compose.yml")
GO_DIR="${ROOT}/golang"
NODE_DIR="${ROOT}/nodejs"

mkdir -p "${RUN_DIR}"

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

have_docker() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

native_pid_file() {
  printf '%s/%s.pid' "${RUN_DIR}" "$1"
}

is_running_pid() {
  local pid="$1"
  [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null
}

stop_native() {
  local name="$1"
  local file
  file="$(native_pid_file "${name}")"
  if [[ -f "${file}" ]]; then
    local pid
    pid="$(cat "${file}")"
    if is_running_pid "${pid}"; then
      kill "${pid}" 2>/dev/null || true
      wait "${pid}" 2>/dev/null || true
      log "stopped native ${name} (pid ${pid})"
    fi
    rm -f "${file}"
  fi
}

start_native_golang() {
  stop_native golang
  (
    cd "${GO_DIR}"
    PORT="${GO_PORT:-8080}" go run ./cmd/api
  ) >"${RUN_DIR}/golang.log" 2>&1 &
  echo $! >"$(native_pid_file golang)"
  log "golang-api started on :${GO_PORT:-8080} (pid $!)"
}

start_native_nodejs() {
  stop_native nodejs
  (
    cd "${NODE_DIR}"
    PORT="${NODE_PORT:-8081}" NODE_ENV="${NODE_ENV:-development}" node src/index.js
  ) >"${RUN_DIR}/nodejs.log" 2>&1 &
  echo $! >"$(native_pid_file nodejs)"
  log "nodejs-api started on :${NODE_PORT:-8081} (pid $!)"
}

compose_up() {
  local services=("$@")
  "${COMPOSE[@]}" up --build -d "${services[@]}"
}

cmd_setup() {
  log "installing golang modules"
  (cd "${GO_DIR}" && go mod download)

  log "installing nodejs modules"
  (cd "${NODE_DIR}" && npm install)

  if have_docker; then
    log "building docker images"
    "${COMPOSE[@]}" build
  else
    log "docker not available; skipped image build (native start still works)"
  fi

  log "setup complete"
}

cmd_start() {
  local target="${1:-all}"
  case "${target}" in
    all|"")
      if have_docker; then
        compose_up golang nodejs
      else
        start_native_golang
        start_native_nodejs
      fi
      ;;
    golang|go)
      if have_docker; then
        compose_up golang
      else
        start_native_golang
      fi
      ;;
    nodejs|node)
      if have_docker; then
        compose_up nodejs
      else
        start_native_nodejs
      fi
      ;;
    *)
      die "unknown service '${target}' (use golang, nodejs, or all)"
      ;;
  esac
  cmd_status
}

cmd_stop() {
  stop_native golang
  stop_native nodejs
  if have_docker; then
    "${COMPOSE[@]}" down --remove-orphans
  fi
  log "stopped"
}

cmd_status() {
  if have_docker; then
    "${COMPOSE[@]}" ps || true
  fi
  for name in golang nodejs; do
    local file pid
    file="$(native_pid_file "${name}")"
    if [[ -f "${file}" ]]; then
      pid="$(cat "${file}")"
      if is_running_pid "${pid}"; then
        log "native ${name}: running (pid ${pid})"
      else
        log "native ${name}: stale pid file"
      fi
    fi
  done
}

cmd_logs() {
  local target="${1:-all}"
  if have_docker && "${COMPOSE[@]}" ps -q >/dev/null 2>&1; then
    case "${target}" in
      all|"") "${COMPOSE[@]}" logs -f --tail=100 ;;
      golang|go) "${COMPOSE[@]}" logs -f --tail=100 golang ;;
      nodejs|node) "${COMPOSE[@]}" logs -f --tail=100 nodejs ;;
      *) die "unknown service '${target}'" ;;
    esac
    return
  fi
  case "${target}" in
    golang|go) tail -n 100 -f "${RUN_DIR}/golang.log" ;;
    nodejs|node) tail -n 100 -f "${RUN_DIR}/nodejs.log" ;;
    *) tail -n 100 -f "${RUN_DIR}/golang.log" "${RUN_DIR}/nodejs.log" ;;
  esac
}

cmd_test() {
  log "testing golang-api"
  (cd "${GO_DIR}" && go test ./...)
  log "testing nodejs-api"
  (cd "${NODE_DIR}" && npm test)
}

cmd="${1:-}"
shift || true
case "${cmd}" in
  setup) cmd_setup ;;
  start) cmd_start "${1:-all}" ;;
  stop) cmd_stop ;;
  status) cmd_status ;;
  logs) cmd_logs "${1:-all}" ;;
  test) cmd_test ;;
  *)
    cat <<'EOF'
Usage: ./scripts/dev.sh <command> [service]

Commands:
  setup              Install deps and build images
  start [service]    Start golang, nodejs, or all (default: all)
  stop               Stop containers and native processes
  status             Show running services
  logs [service]     Follow logs
  test               Run unit tests

Services: golang | nodejs | all

If Docker is running, start uses docker compose.
Otherwise processes start natively:
  golang-api  http://127.0.0.1:8080
  nodejs-api  http://127.0.0.1:8081
EOF
    exit 1
    ;;
esac
