#!/usr/bin/env bash
# leader_failover.sh — chaos proof: kill Redis lease holder, measure failover ms.
set -euo pipefail

# --- paths & config ---
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/deploy/compose/docker-compose.load.yml}"

START=$(date +%s)

log(){
  echo "T+$(( $(date +%s) - START ))s  $*" >&2
}

compose(){
  docker compose -f "$COMPOSE_FILE" "$@"
}

leader_to_service() {
  case "$1" in
    api-1) echo "api-1" ;;
    api-2) echo "api-2" ;;
    *)
      echo "unknown leader: $1" >&2
      exit 1
      ;;
  esac
}

get_leader() {
  compose exec -T redis redis-cli GET chronos:leader | tr -d '\r\n'
}

now_ms() {
  python3 -c 'import time; print(int(time.time() * 1000))'
}

wait_until_leader_changes() {
  # $1 = old leader id
  local old="$1"
  local deadline=$(( $(date +%s) + 30 ))
  local cur

  while [ "$(date +%s)" -lt "$deadline" ]; do
    cur=$(get_leader)
    if [ -n "$cur" ] && [ "$cur" != "$old" ]; then
      echo "$cur"
      return 0
    fi
    sleep 0.5
  done
  log "FAIL: timeout waiting for leader change (still='$cur')"
  exit 1
}

log "leader_failover starting (ROOT=$ROOT)"
log "compose file: $COMPOSE_FILE"

LEADER=$(get_leader)
if [ -z "$LEADER" ]; then
  log "FAIL: no leader in Redis"
  exit 1
fi
SVC=$(leader_to_service "$LEADER")
log "current leader=$LEADER service=$SVC"

t0=$(now_ms)
compose kill -s SIGKILL "$SVC"
log "killed $SVC"

NEW=$(wait_until_leader_changes "$LEADER")
t1=$(now_ms)
elapsed=$(( t1 - t0 ))

log "new leader=$NEW"
log "failover_ms=$elapsed"

compose up -d "$SVC"
log "restored $SVC"

if [ "$elapsed" -lt 30000 ]; then
  log "PASS: failover under 30s"
  exit 0
fi
log "FAIL: failover_ms=$elapsed >= 30000"
exit 1