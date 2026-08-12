# Chronos

Chronos is a job scheduler I'm building in Go. You submit an HTTP task (URL, method, headers, body, timeout, retries), it gets stored in Postgres, and a worker claims it and actually hits that URL.

Postgres is the source of truth for jobs and claims. Redis is in the compose file for later (leader election / heartbeats). It is not used as a job queue.

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

## Still todo

- Redis leader election, cron schedules
- Prometheus metrics and a small React dashboard
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

Terminal 2 — worker:

```bash
go run ./cmd/worker
```

### Seed a tenant and queue

The API expects an existing tenant + queue. Quick seed:

```bash
psql "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable" <<'SQL'
INSERT INTO tenants (name) VALUES ('demo') ON CONFLICT DO NOTHING;
INSERT INTO queues (tenant_id, name)
SELECT id, 'default' FROM tenants WHERE name = 'demo'
ON CONFLICT DO NOTHING;
SELECT t.id AS tenant_id, q.id AS queue_id
FROM tenants t
JOIN queues q ON q.tenant_id = t.id
WHERE t.name = 'demo';
SQL
```

Note the `queue_id`. The Compose worker defaults to `QUEUE_ID=1`; change the compose env (or host export) if your queue id differs.

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

If `QUEUE_ID` matches the job's queue, you should see `state` flip to `succeeded` pretty quickly. Lock fields clear after completion and `attempt_count` bumps.

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
| `HTTP_ADDR` | `:8080` | API listen address |
| `DATABASE_URL` | local compose Postgres URL | pgx DSN |
| `JWT_SECRET` | `dev-secret-change-me` | change this if you expose the API |
| `QUEUE_ID` | `1` | queue the worker polls |
| `WORKER_ID` | `hostname-pid` | shows up in `locked_by` while running |
| `LEASE_TIMEOUT` | `60` (seconds) | how old a `running` lock must be before reclaim |

## Tests

```bash
go test ./internal/execute/ -v
go test ./internal/job/ -v

# needs Postgres up and migrated
go test ./internal/store/ -v -count=1
```

CI does the same idea on every push/PR (see `.github/workflows/ci.yml`): unit packages, `migrate up`, then store tests against a Postgres service container.

## Layout

```
cmd/chronos/          API server
cmd/worker/           claim / execute / reclaim loop
internal/api/         HTTP handlers
internal/auth/        JWT helpers
internal/store/       Postgres access
internal/worker/      claim -> execute -> complete/fail
internal/execute/     outbound HTTP + SSRF checks
internal/job/         domain types / states / retry backoff
migrations/           SQL
Dockerfile            multi-stage image (api + worker binaries)
deploy/compose/       Compose: Postgres, Redis, api, worker
.github/workflows/    CI
```

## Why Postgres owns claims

I keep job ownership in the database on purpose. Redis can die or flap without inventing a second source of truth for "who is running this attempt." Coordination stuff (leader, presence) can live in Redis later without becoming the queue.
