# ADR 0003: At-least-once execution with outbound idempotency keys

- Status: Accepted
- Date: 2026-09-04

## Context

Postgres claim leases (ADR 0002) give **at most one active worker claim** on a job row. That is not the same as exactly-once side effects.

Classic failure window:

1. Worker claims the job
2. Worker executes the outbound HTTP request (remote already received it)
3. Worker crashes before `CompleteJob` / `FailJob`
4. Lease goes stale → `ReclaimStaleJobs` → another worker claims
5. HTTP runs again

`attempt_count` advances only on Complete/Fail, not on claim. After crash-before-ack, the second delivery still sees the same attempt number.

Create-path `Idempotency-Key` on `POST /jobs` only dedupes **job creation**. It does not make outbound execution exactly-once.

## Decision

Name the contract explicitly:

- **At-most-one active claim** — `FOR UPDATE SKIP LOCKED` + `locked_by` / `locked_at` (ADR 0002)
- **At-least-once execution** — reclaim can re-run HTTP after a crash in the ack window
- **Outbound idempotency support** — if the job payload did not set `Idempotency-Key`, execute sets  
  `Idempotency-Key: chronos-<jobId>-<attemptCount>`  
  so the **same** key is reused across reclaim-without-ack; a **new** key is used after Complete/Fail advances `attempt_count` (recorded failure then retry)

Exactly-once side effects remain the responsibility of the HTTP target (honor the key). Chronos does not claim exactly-once delivery.

## Alternatives considered

| Option | Why not (V1) |
| --- | --- |
| Call the system “exactly-once” because of Postgres locks | Confuses exclusive claim with exclusive side effect; fails the crash-after-HTTP interview question |
| Two-phase commit / transactional outbox with the target | Correct for some orgs; too much protocol surface for a portfolio MVP; target still must cooperate |
| Dedupe store of response bodies in Chronos | Stops duplicate *records*, not duplicate *side effects* at the remote |
| Docs-only (no outbound key, no test) | Weaker evidence; key + integration test make the contract measurable |

## Consequences

**Positive**

- Honest interview answer: exclusive claim ≠ exactly-once HTTP
- Targets that honor `Idempotency-Key` can coalesce crash-before-ack duplicates
- Create idempotency and execution idempotency stay clearly separated

**Negative / measured cost**

- Without an idempotent target, duplicate side effects are possible after reclaim
- Integration evidence: `TestAtLeastOnceExecutionAfterCrashBeforeComplete` — fake HTTP server records **two** hits with the **same** `chronos-<id>-0` key after claim → execute → skip Complete → reclaim → claim → execute → one `CompleteJob` → **one** `job_attempts` success row
- Ops chaos (`scripts/chaos/worker_reclaim.sh`) still demonstrates reclaim timing; it does not by itself prove double HTTP (see the integration test for that)

## References

- `internal/execute/http.go` — outbound `Idempotency-Key`
- `internal/worker/execution_semantics_integration_test.go`
- `internal/store` — `ClaimJob`, `CompleteJob`, `FailJob`, `ReclaimStaleJobs`
- ADR 0002 — claim leases
- README — [Execution semantics](../../README.md#execution-semantics)
