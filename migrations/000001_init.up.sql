--- 1. TENANTS FRIST
CREATE TABLE tenants (
    id         bigserial PRIMARY KEY, 
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);


-- Table 2 depends on tenants
CREATE TABLE queues (
    id         bigserial PRIMARY KEY, 
    tenant_id  bigint NOT NULL REFERENCES tenants(id),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

-- 3. Jobs(depends on tenants + queues)

CREATE TABLE jobs (
    id                  bigserial PRIMARY KEY,
    tenant_id           bigint NOT NULL REFERENCES tenants(id),
    queue_id            bigint NOT NULL REFERENCES queues(id),
    url                 text NOT NULL,
    method              text NOT NULL,
    headers             jsonb NOT NULL DEFAULT '{}',
    body                BYTEA,
    timeout_ms          integer NOT NULL DEFAULT 30000,
    state               text NOT NULL DEFAULT 'pending',
    run_at              timestamptz NOT NULL DEFAULT now(),
    attempt_count       integer NOT NULL DEFAULT 0,
    max_attempts        integer NOT NULL DEFAULT 3,
    next_run_at         timestamptz,
    locked_by           text,
    locked_at           timestamptz,
    cancel_requested    boolean NOT NULL DEFAULT FALSE,
    idempotency_key     text,
    schedule_id         bigint DEFAULT NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_jobs_queue_state_run ON jobs (queue_id, state, run_at);
CREATE INDEX idx_tenant_queue_state_run ON jobs (tenant_id, created_at);


--4 JOB ATTEMPS
CREATE TABLE job_attempts(
    id                  bigserial PRIMARY KEY,
    job_id              bigint NOT NULL REFERENCES jobs(id),
    attempt_number      integer NOT NULL DEFAULT 0,
    worker_id           text DEFAULT NULL,
    started_at          timestamptz NOT NULL DEFAULT now(),
    finished_at         timestamptz NOT NULL DEFAULT now(),
    success             text,
    http_status         text,
    error_message       text,
    response_snippet    text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE              (job_id, attempt_number)
);

--5 Cron_definitions 
CREATE TABLE cron_definitions(
    id                  bigserial PRIMARY KEY,
    tenant_id           bigint NOT NULL REFERENCES tenants(id),
    queue_id            bigint NOT NULL REFERENCES queues(id),
    cron_expr           text NOT NULL,
    timezone            text DEFAULT 'EST',
    url                 text NOT NULL,
    method              text NOT NULL,
    headers             jsonb NOT NULL DEFAULT '{}',
    body                BYTEA,
    timeout_ms          integer NOT NULL DEFAULT 30000,
    max_attempts        integer NOT NULL DEFAULT 3,
    enabled             boolean NOT NULL DEFAULT FALSE,
    last_enqueued_at     timestamptz NOT NULL DEFAULT now(),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

-- 6. Idempotency_keys 
CREATE TABLE idempotency_keys(
    id                  bigserial PRIMARY KEY,
    tenant_id           bigint NOT NULL REFERENCES tenants(id),
    key                 text NOT NULL,
    job_id              bigint NOT NULL REFERENCES jobs(id),
    request_hash        text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE              (tenant_id, key)
);