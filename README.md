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
- Prometheus: `GET /metrics` on the API (`:8080`) and a metrics-only listener on the worker (`METRICS_ADDR`, default `:8081`)
- CI runs `./internal/cron/` unit tests (IsDue) with the other unit packages

## Still todo

- Thin React ops dashboard under `web/` (list / detail / replay / queue depth) — not Grafana
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
| `LEASE_TIMEOUT` | `60` (seconds) | how old a `running` lock must be before reclaim |

## Tests

```bash
go test ./internal/execute/ ./internal/job/ ./internal/cron/ -v -count=1

# needs Postgres up and migrated
go test ./internal/store/ -v -count=1
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
web/                  Vite + React + TS ops UI (in progress)
migrations/           SQL
Dockerfile            multi-stage image (api + worker binaries)
deploy/compose/       Compose: Postgres, Redis, api, worker
.github/workflows/    CI
```

## Why Postgres owns claims

I keep job ownership in the database on purpose. Redis can die or flap without inventing a second source of truth for "who is running this attempt." The Redis lease only answers "which API process may tick cron." Duplicate fires are still prevented by the Postgres CAS on `last_enqueued_at`.
