#!/usr/bin/env bash
# worker_reclaim.sh — chaos proof: kill holder mid-job, wait for other worker.
set -euo pipefail

# --- paths & config ---
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$SCRIPT_DIR/../.." && pwd)
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT/deploy/compose/docker-compose.load.yml}"
BASE_URL="${BASE_URL:-http://localhost:8080}"

START=$(date +%s)

log(){
  echo "T+$(( $(date +%s) - START ))s  $*" >&2
}

compose(){
    docker compose -f "$COMPOSE_FILE" "$@"
}

worker_to_service() {
    case "$1" in
        compose-worker-1) echo "worker-1" ;;
        compose-worker-2) echo "worker-2" ;;
        *)
          echo "unkown locked_by: $1" >&2
          exit 1
          ;;
    esac
}

get_token() {
  local body
  body=$(curl -s -X POST "$BASE_URL/auth/token" \
    -H 'Content-Type: application/json' \
    -d '{"tenant_id":1}')
  echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])"
}

create_job() {
  # $1 = token
  local token="$1"
  local key="chaos-$(date +%s)"
  local body
  body=$(curl -s -X POST "$BASE_URL/jobs" \
    -H "Authorization: Bearer $token" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $key" \
    -d '{
      "queue_id": 1,
      "url": "https://httpbin.org/delay/30",
      "method": "GET",
      "timeout_ms": 60000,
      "max_attempts": 2
    }')
  echo "$body" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])"
}
get_job() {
  # $1=token  $2=job_id
  curl -s "$BASE_URL/jobs/$2" \
    -H "Authorization: Bearer $1"
}

job_field() {
  # $1=json  $2=field name
  echo "$1" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get(sys.argv[1],''))" "$2"
}

wait_until_claimed() {
  # $1=token  $2=job_id
  local token="$1"
  local job_id="$2"
  local deadline=$(( $(date +%s) + 30 ))
  local json state locked

  while [ "$(date +%s)" -lt "$deadline" ]; do
    json=$(get_job "$token" "$job_id")
    state=$(job_field "$json" "state")
    locked=$(job_field "$json" "locked_by")
    if [ "$state" = "running" ] && [ -n "$locked" ]; then
      log "claimed: state=$state locked_by=$locked"
      echo "$locked"   # return locked_by on stdout for the caller
      return 0
    fi
    sleep 1
  done
  log "timeout waiting for claim (last state=$(job_field "$json" "state"))"
  exit 1
}

wait_until_other_worker() {
  # $1=token  $2=job_id  $3=old locked_by
  local token="$1"
  local job_id="$2"
  local old="$3"
  local deadline=$(( $(date +%s) + 90 ))
  local json state locked
  local saw_runnable=0

  while [ "$(date +%s)" -lt "$deadline" ]; do
    json=$(get_job "$token" "$job_id")
    state=$(job_field "$json" "state")
    locked=$(job_field "$json" "locked_by")

    if [ "$state" = "runnable" ] && [ "$saw_runnable" -eq 0 ]; then
      log "reclaimed: state=runnable (lock cleared)"
      saw_runnable=1
    fi

    if [ -n "$locked" ] && [ "$locked" != "$old" ]; then
      log "PASS: reclaimed by other locked_by=$locked state=$state"
      echo "$locked"
      return 0
    fi

    sleep 2
  done

  log "FAIL: timeout waiting for other worker (last state=$state locked_by=$locked)"
  exit 1
}


log "worker_reclaim starting (ROOT=$ROOT)"
log "compose file: $COMPOSE_FILE"
log "base url: $BASE_URL"

TOKEN=$(get_token)
log "got token (len=${#TOKEN})"

JOB_ID=$(create_job "$TOKEN")
log "created job id=$JOB_ID"

LOCKED_BY=$(wait_until_claimed "$TOKEN" "$JOB_ID")
SVC=$(worker_to_service "$LOCKED_BY")
log "ready to kill holder=$LOCKED_BY service=$SVC"

compose kill -s SIGKILL "$SVC"
log "killed $SVC"

NEW_LOCKED=$(wait_until_other_worker "$TOKEN" "$JOB_ID" "$LOCKED_BY")
log "done job=$JOB_ID old=$LOCKED_BY new=$NEW_LOCKED"

# optional: bring killed worker back for the next run
compose up -d "$SVC"
log "restored $SVC"

exit 0