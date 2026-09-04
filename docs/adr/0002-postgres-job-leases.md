# ADR 0002: Postgres owns job claims (not a Redis work queue)

- Status: Accepted
- Date: 2026-09-04

## Context

Workers need exclusive claim of a job attempt, crash recovery when a process dies mid-execution, and a single durable history for state, attempts, and DLQ. Redis is already in the stack for scheduler leadership (ADR 0001).

## Decision

Keep the runnable queue and claim lease in Postgres:

- Claim via `FOR UPDATE SKIP LOCKED` on runnable rows
- Ownership columns `locked_by` / `locked_at` with `LEASE_TIMEOUT`; `ReclaimStaleJobs` clears stale `running` locks
- Redis is **not** the work queue — only the leader lease key

## Alternatives considered

| Option | Why not (V1) |
| --- | --- |
| Redis list / Streams as the queue | Second source of truth for “who owns this attempt”; harder crash story; duplicates Postgres job rows |
| No reclaim (trust workers forever) | A `SIGKILL`’d worker strands `running` jobs indefinitely |
| Exactly-once execution after reclaim | Requires distributed transactions or external idempotency of the HTTP target; V1 proves **recovery** (at-least-once), not exactly-once side effects |

## Consequences

**Positive**

- One durable SoT for job state, attempts, cancel, DLQ
- Redis flap does not strand or double-book claims in a second store
- Reclaim behavior is demonstrable with a repeatable chaos script

**Negative / measured cost**

- After hard kill, the job stays locked until `LEASE_TIMEOUT` ages out, then another worker claims
- Chaos run (`scripts/chaos/worker_reclaim.sh`), job `#10569`, load-stack `LEASE_TIMEOUT=15`: claimed by `compose-worker-2` → `SIGKILL` → `runnable` ~**16s** after kill → claimed by `compose-worker-1` (~35s wall from script start)
- In-flight HTTP from the dead worker may still complete; Chronos may run the job again (at-least-once)

## References

- `internal/store` / worker reclaim path; env `LEASE_TIMEOUT`
- `scripts/chaos/worker_reclaim.sh`
- README — [Worker reclaim chaos](../../README.md#worker-reclaim-chaos)
