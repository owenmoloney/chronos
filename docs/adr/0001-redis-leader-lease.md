# ADR 0001: Redis SETNX lease for scheduler leadership

- Status: Accepted
- Date: 2026-09-04

## Context

Chronos can run more than one API process. Cron enqueue must not race: only one process should tick schedules. Election has to survive hard kills (`SIGKILL`) and stay cheap to renew under normal load.

## Decision

Use a Redis key `chronos:leader` as a lease:

- Acquire with `SETNX` + TTL (10s) keyed by instance id (`WORKER_ID`, e.g. `api-1`)
- Renew with a single Lua script: GET must match this instance, then EXPIRE
- Only the holder runs the scheduler tick loop (`internal/leader`, `internal/scheduler`)

Leadership answers only “who may tick cron.” It is not HTTP high availability and not job ownership.

## Alternatives considered

| Option | Why not (V1) |
| --- | --- |
| Postgres advisory locks | Couples election to the job DB; session/TTL semantics under crash are awkward; cron load would compete with claim/reclaim traffic |
| etcd / Consul | Correct for large orgs; too much ops surface for a portfolio MVP already running Postgres + Redis |
| Every API ticks cron | Relies entirely on `last_enqueued_at` CAS; works for idempotency but wastes work and muddies “who scheduled this” under failure drills |

## Consequences

**Positive**

- Clear single ticker; renew is one round-trip
- Redis outage does not invent a second source of truth for running jobs (Postgres still owns claims — see ADR 0002)
- Duplicate cron fires remain guarded by the Postgres CAS on `last_enqueued_at`

**Negative / measured cost**

- Failover is bounded by lease TTL + acquire loop (~3s), not instant
- Scripted kill of the holder (`scripts/chaos/leader_failover.sh`), 6 consecutive runs: **avg ~11247 ms (~11.2s)**, min **8907** ms, max **13260** ms — all under the 30s portfolio bar; both `api-1` ↔ `api-2`
- The killed API’s host port goes down (no load balancer in front); leadership ≠ zero-downtime HTTP on that port

## References

- `internal/leader/leader.go` — `ChronosKey`, `ConfigLeaseTTL = 10s`
- `scripts/chaos/leader_failover.sh`
- README — [Leader failover benchmark](../../README.md#leader-failover-benchmark) (per-run table)
