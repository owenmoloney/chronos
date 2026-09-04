# Chronos

Chronos is a job scheduler I'm building in Go. You submit an HTTP task (URL, method, headers, body, timeout, retries), it gets stored in Postgres, and a worker claims it and actually hits that URL.

Postgres is the source of truth for jobs and claims. Redis is only for leader election (lease), not a job queue.

## What's working right now

- JWT token issue and tenant-scoped access
- Create + get jobs over HTTP
- Idempotency-Key on create (same key returns the original job; different body gets 409)
- Workers claim with `FOR UPDATE SKIP LOCKED` so two workers don't take the same job
- HTTP execution with basic SSRF checks (private / loopback / link-local / multicast blocked)
- Failures either get requeued for retry or move to `dead_lettered` when max attempts are hit
- Retry delay uses exponential backoff + jitter (not a fixed 5s window)
- Stale `running` locks get reclaimed on a timer so a crashed worker doesn't strand jobs forever
- DLQ replay: `POST /jobs/{id}/replay` moves `dead_lettered` back to `runnable` and resets `attempt_count`
- Cancel: `POST /jobs/{id}/cancel` — pending/runnable go straight to `canceled`; running sets `cancel_requested` (cooperative)
- Worker checks `cancel_requested` after claim and acknowledges to `canceled` before HTTP (mid-flight cancel still finishes the in-flight request)
- Worker heartbeats refresh `locked_at` every 10s during HTTP so reclaim doesn't steal long-running healthy jobs
- GitHub Actions CI runs unit tests + migrates Postgres + store integration tests on push/PR
- API (`cmd/chronos`) and worker (`cmd/worker`) are separate processes
- Docker Compose runs Postgres, Redis, API, and worker from a single `chronos:local` image
- Redis leader election: one API process ticks cron; lease renew is a single Lua GET+EXPIRE
- Cron enqueue is one Postgres transaction (CAS `last_enqueued_at` + insert job + mark runnable); `jobs.schedule_id` links cron-created jobs to the definition
- `GET /jobs?queue_id=&state=&limit=` — tenant-scoped list for the ops UI (`GET /jobs/{id}` still does detail)
- `GET /jobs/{id}/attempts` — attempt history (http status, error message, response snippet) for ops debugging
- Prometheus: `GET /metrics` on the API (`:8080`) and a metrics-only listener on the worker (`METRICS_ADDR`, default `:8081`)
- CI runs `./internal/cron/` unit tests (IsDue) with the other unit packages
- React ops UI under `web/` (Vite proxy → Go API): auth bootstrap, job list + state filter + runnable depth, create (+ optional Idempotency-Key), full job detail, cancel, DLQ replay, attempt history — not Grafana
- Load/failover validated on a dual-API Compose topology: **100+ job submissions/sec under k6**, and **leader failover under 30s** (scripted Redis lease flip, ~9–13s across runs) after killing the API that held the lease
- Worker chaos: `scripts/chaos/worker_reclaim.sh` — `SIGKILL` holder mid-job → reclaim → other worker claims (timeline on stderr)
- Leader failover benchmark: `scripts/chaos/leader_failover.sh` — `SIGKILL` lease holder → poll `chronos:leader` until flip → print `failover_ms`

## Still todo

- Kubernetes (or similar) deploy beyond local Compose

## Requirements

- Go 1.22+ (I've been on newer)
- Docker + Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate) on your PATH

## Run it locally

Two options: full Compose (API + worker in Docker), or Postgres in Docker and binaries on the host.

### Option A — Docker Compose (recommended)

From the repo root (`chronos/`, where `go.mod` lives):

```bash
# 1) Build the image once (both /bin/chronos and /bin/worker)
docker build -t chronos:local .

# 2) Start Postgres, Redis, API, worker
cd deploy/compose
docker compose up -d
cd ../..
```

Compose expects the pre-built `chronos:local` tag (see `deploy/compose/docker-compose.yml`). Rebuild the image after code changes, then `docker compose up -d` again.

Postgres is on `localhost:5432` (user/pass/db: `chronos` / `chronos` / `chronos`). API is on `localhost:8080`. Inside the Compose network, app containers talk to Postgres as host `postgres` (not `localhost`).

```bash
# 3) Migrate (against the published port on the host)
export DATABASE_URL=postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable
./scripts/migrate.sh up
```

Seed (step below), then smoke-test against `http://localhost:8080`.

Useful:

```bash
cd deploy/compose
docker compose ps
docker compose logs -f api worker
```

### Option B — binaries on the host

```bash
cd deploy/compose
docker compose up -d postgres redis
cd ../..

export JWT_SECRET=dev-secret-change-me
export QUEUE_ID=1
export DATABASE_URL=postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable
./scripts/migrate.sh up
```

Terminal 1 — API:

```bash
go run ./cmd/chronos
```

Terminal 2 — worker (metrics on `:8081` unless you set `METRICS_ADDR`):

```bash
go run ./cmd/worker
```

### Seed a tenant, queue, and cron

The API expects an existing tenant + queue. The cron row keeps the scheduler (and later the dashboard) from looking empty.

```bash
psql "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable" <<'SQL'
INSERT INTO tenants (name) VALUES ('demo') ON CONFLICT DO NOTHING;
INSERT INTO queues (tenant_id, name)
SELECT id, 'default' FROM tenants WHERE name = 'demo'
ON CONFLICT DO NOTHING;
INSERT INTO cron_definitions (
  tenant_id, queue_id, cron_expr, timezone,
  url, method, enabled, last_enqueued_at
)
SELECT t.id, q.id, '* * * * *', 'UTC',
       'https://example.com', 'GET', true, '1970-01-01'::timestamptz
FROM tenants t
JOIN queues q ON q.tenant_id = t.id
WHERE t.name = 'demo'
LIMIT 1;
SELECT t.id AS tenant_id, q.id AS queue_id
FROM tenants t
JOIN queues q ON q.tenant_id = t.id
WHERE t.name = 'demo';
SQL
```

Note the `queue_id`. The Compose worker defaults to `QUEUE_ID=1`; change the compose env (or host export) if your queue id differs.

`last_enqueued_at` is set in the past so the first leader tick can treat the schedule as due (`DEFAULT now()` would often skip the first fire).

### Smoke test

Get a token (tenant_id from seed):

```bash
curl -s -X POST http://localhost:8080/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":1}'
```

Create a job:

```bash
TOKEN='paste-token-here'

curl -s -X POST http://localhost:8080/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: smoke-1' \
  -d '{
    "queue_id": 1,
    "url": "https://example.com",
    "method": "GET",
    "timeout_ms": 5000,
    "max_attempts": 3
  }'
```

Check it:

```bash
curl -s http://localhost:8080/jobs/JOB_ID \
  -H "Authorization: Bearer $TOKEN"
```

List jobs for a queue (ops UI uses this):

```bash
curl -s "http://localhost:8080/jobs?queue_id=1&limit=10" \
  -H "Authorization: Bearer $TOKEN"

curl -s "http://localhost:8080/jobs?queue_id=1&state=dead_lettered" \
  -H "Authorization: Bearer $TOKEN"
```

`queue_id` is required. `state` and `limit` are optional (`limit` defaults to 50, max 200). Response is a JSON array of job objects (same shape as get, including `schedule_id`).

If `QUEUE_ID` matches the job's queue, you should see `state` flip to `succeeded` pretty quickly. Lock fields clear after completion and `attempt_count` bumps.

### Metrics

API and worker are separate processes, so they have **separate** Prometheus registries. Do not expect worker counters on `:8080`.

```bash
# API: leader gauge + cron enqueue (this process)
curl -s localhost:8080/metrics | grep chronos_

# Worker: claim / complete / fail (this process). Default METRICS_ADDR=:8081
curl -s localhost:8081/metrics | grep chronos_
```

| Series | Where it moves |
| --- | --- |
| `chronos_leader` | API only (`1` if this process holds the Redis lease) |
| `chronos_cron_enqueued_total` | API, after a cron fire is claimed and the job is marked runnable |
| `chronos_jobs_claimed_total` | Worker, after a successful claim |
| `chronos_jobs_completed_total` | Worker, after `CompleteJob` succeeds |
| `chronos_jobs_failed_total` | Worker, after `FailJob` succeeds |

Host `go run ./cmd/worker` listens on `:8081` by default. The Compose worker service does not publish `8081` yet.

Cancel a job that hasn't started yet (or is still `runnable`):

```bash
curl -s -X POST http://localhost:8080/jobs/JOB_ID/cancel \
  -H "Authorization: Bearer $TOKEN"
```

Replay something out of the DLQ (after it hit `dead_lettered`):

```bash
curl -s -X POST http://localhost:8080/jobs/JOB_ID/replay \
  -H "Authorization: Bearer $TOKEN"
```

Attempt history for a job (empty array until the worker has written rows):

```bash
curl -s http://localhost:8080/jobs/JOB_ID/attempts \
  -H "Authorization: Bearer $TOKEN"
```

## Ops UI (`web/`)

Vite + React + TypeScript. In dev, `/api` proxies to the Go API on `:8080`.

```bash
# API must already be running (Option A or B above)
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. Auth bootstraps with tenant `1` (same as seed).

Useful:

```bash
cd web
npm run typecheck
npm run smoke    # needs API on :8080; set CHRONOS_API_BASE if different
```

| Script | What it does |
| --- | --- |
| `npm run dev` | Vite UI on `:5173`, proxy `/api` → `:8080` |
| `npm run typecheck` | `tsc -b` |
| `npm run smoke` | TS client against live API (token, list, create, attempts, cancel, optional replay) |
| `npm run build` | Production bundle |

## Load, failover, and chaos

Day-to-day Compose stays single API + worker (`docker-compose.yml`). For the portfolio claim — **validated at 100+ submissions/sec under k6 with leader failover under 30s after killing the holding API process** — use the load topology. Worker reclaim after a hard kill is a separate Postgres lease path (below).

“Leader failover” means **Redis scheduler leadership** (which API may tick cron), not load-balanced zero-downtime HTTP. Worker reclaim uses `LEASE_TIMEOUT` on `jobs.locked_at` (daily default **60s**; load compose may use **15s** so chaos demos finish in tens of seconds — say which value you measured).

The `chronos:local` image installs **ca-certificates** so workers can dial public HTTPS targets (needed for delay-URL chaos jobs). Rebuild after Dockerfile changes: `docker build -t chronos:local .`

### Topology

`deploy/compose/docker-compose.load.yml`: Postgres, Redis, `api-1` (`:8080`), `api-2` (`:8082`), `worker-1`, `worker-2`. Each API/worker needs a distinct `WORKER_ID` (instance id for the lease / `locked_by`).

```bash
# from chronos/ — stop anything already bound to 8080/8082 first
docker build -t chronos:local .
cd deploy/compose
docker compose -f docker-compose.load.yml down --remove-orphans
docker compose -f docker-compose.load.yml up -d
docker compose -f docker-compose.load.yml ps -a
```

Both APIs should be **Up**. If `api-2` exited once on cold start, Postgres may not have been ready yet — `up -d api-2` again after healthy, or rely on `depends_on` + a retry.

```bash
curl -s localhost:8080/health
curl -s localhost:8082/health
docker compose -f docker-compose.load.yml logs api-1 api-2 2>&1 | grep -E 'became leader|lost leadership'
```

Exactly one current leader (latest `became leader` without a later `lost leadership` for that id). `chronos_leader` on `/metrics` can lag until the next scheduler tick (~30s) — prefer logs for timing.

### k6 create load (Docker)

Homebrew k6 is optional; run Grafana’s image. Script: `scripts/load/create_jobs.js` (JWT in `setup()`, then `POST /jobs` with unique `Idempotency-Key`).

```bash
# from chronos/, load stack already up
docker run --rm -i \
  -e BASE_URL=http://host.docker.internal:8080 \
  -v "$PWD/scripts/load:/scripts" \
  grafana/k6 run /scripts/create_jobs.js
```

**Measured (Docker Desktop, workers running):** ~**113** `http_reqs`/s sustained, **0%** HTTP failures, **100%** create checks at 201. Thresholds care about rate and errors; p95 was ~1.1s under concurrent workers on this laptop (median ~25ms) — not part of the portfolio sentence.

### Leader failover benchmark

Proves scheduler leadership moves after a hard kill of the holding API. Signal is Redis `GET chronos:leader` (not `/metrics` — `chronos_leader` can lag until the next scheduler tick).

Primary method — load stack with **both** `api-1` and `api-2` Up:

```bash
# from chronos/
./scripts/chaos/leader_failover.sh
```

Script: read lease holder → `SIGKILL` that compose service → poll until the key value changes → print `failover_ms` → restore the killed API → exit 0 if under 30s.

**Measured** (6 consecutive runs, lease TTL 10s, renew/acquire every 3s):

| Run | Killed | New leader | failover_ms |
| --- | --- | --- | --- |
| 1 | api-1 | api-2 | 11047 |
| 2 | api-2 | api-1 | 13260 |
| 3 | api-1 | api-2 | 11682 |
| 4 | api-2 | api-1 | 10284 |
| 5 | api-1 | api-2 | 12300 |
| 6 | api-2 | api-1 | 8907 |

min **8907** ms, max **13260** ms, all under 30s; both directions (`api-1` ↔ `api-2`). Spread is mostly remaining TTL at kill time plus the survivor’s next acquire tick. The killed host port goes down (no LB in front); leadership ≠ zero-downtime HTTP on that port.

### Worker reclaim chaos

Proves a `SIGKILL`’d worker does not strand a job forever: `ReclaimStaleJobs` clears the stale `running` lock, then another worker claims via `SKIP LOCKED`.

Primary method — load stack up, both workers with `LEASE_TIMEOUT=15`, API on `:8080`:

```bash
# from chronos/
./scripts/chaos/worker_reclaim.sh
```

Script: mint JWT → create `httpbin.org/delay/30` job → poll until claimed → `SIGKILL` that compose worker → poll until another `locked_by` → restore the killed worker → exit 0/1. Timeline lines are `T+…s` on stderr.

**Measured** (script run, job `#10569`, **15s** lease):

| T+ | Event |
| --- | --- |
| 1s | created |
| 2s | claimed — `locked_by=compose-worker-2` |
| 4s | `SIGKILL` `worker-2` |
| 20s | reclaimed — `runnable` (~**16s** after kill) |
| 35s | PASS — `locked_by=compose-worker-1` |

Earlier manual run (`#10566`) showed the same shape (~25s to `runnable`, then the other worker).

Caveat: reclaim is **at-least-once**. The dead worker’s in-flight HTTP may still complete; Chronos will run the job again after reclaim. This demo proves **recovery**, not exactly-once side effects.

Back to daily mode:

```bash
docker compose -f docker-compose.load.yml down
docker compose up -d
```

## Env vars

| Name | Default | Notes |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | API listen address (includes `/metrics`) |
| `METRICS_ADDR` | `:8081` | Worker metrics-only listen address |
| `DATABASE_URL` | local compose Postgres URL | pgx DSN |
| `REDIS_URL` | `redis://localhost:6379/0` | Leader lease |
| `JWT_SECRET` | `dev-secret-change-me` | change this if you expose the API |
| `QUEUE_ID` | `1` | queue the worker polls |
| `WORKER_ID` | `hostname-pid` | shows up in `locked_by` while running |
| `LEASE_TIMEOUT` | `60` (seconds) | how old a `running` lock must be before reclaim; load/chaos demos may use `15` |

## Tests

```bash
go test ./internal/execute/ ./internal/job/ ./internal/cron/ -v -count=1

# needs Postgres up and migrated
go test ./internal/store/ -v -count=1

# needs API on :8080 (and Postgres). From web/:
# npm run smoke
```

CI does the same idea on every push/PR (see `.github/workflows/ci.yml`): unit packages (including cron `IsDue`), `migrate up`, then store tests against a Postgres service container.

## Layout

```
cmd/chronos/          API server + leader + cron scheduler
cmd/worker/           claim / execute / reclaim loop + /metrics
internal/api/         HTTP handlers
internal/auth/        JWT helpers
internal/observe/     logs + Prometheus instruments
internal/leader/      Redis SETNX lease + Lua renew
internal/scheduler/   leader-only cron tick / enqueue
internal/cron/        IsDue helpers + unit tests
internal/store/       Postgres access
internal/worker/      claim -> execute -> complete/fail
internal/execute/     outbound HTTP + SSRF checks
internal/job/         domain types / states / retry backoff
web/                  Vite + React + TS ops UI (list / create / detail / cancel / replay / attempts)
migrations/           SQL
Dockerfile            multi-stage image (api + worker binaries)
deploy/compose/       Compose: daily stack + docker-compose.load.yml (2 API / 2 worker)
scripts/load/         k6 create-job script (run via grafana/k6 Docker image)
scripts/chaos/        worker_reclaim.sh, leader_failover.sh
.github/workflows/    CI
```

## Why Postgres owns claims

I keep job ownership in the database on purpose. Redis can die or flap without inventing a second source of truth for "who is running this attempt." The Redis lease only answers "which API process may tick cron." Duplicate fires are still prevented by the Postgres CAS on `last_enqueued_at`.
