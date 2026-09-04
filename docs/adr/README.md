# Architecture Decision Records

Short decisions that shape Chronos. Prefer linking measured evidence over re-stating it.

| ADR | Title | Status |
| --- | --- | --- |
| [0001](0001-redis-leader-lease.md) | Redis SETNX lease for scheduler leadership | Accepted |
| [0002](0002-postgres-job-leases.md) | Postgres owns job claims (not a Redis work queue) | Accepted |
| [0003](0003-at-least-once-execution.md) | At-least-once execution with outbound idempotency keys | Accepted |
