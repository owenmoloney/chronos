export type JobState = 
    | "pending"
    | "runnable"
    | "running"
    | "succeeded"
    | "failed_retrying"
    | "dead_lettered"
    | "canceled"

export interface Job{ 
    id: number;
    tenant_id: number;
    queue_id: number;
    url: string;
    method: string;
    headers: Record<string,string>;
    body: unknown;
    timeout_ms: number;
    state: JobState;
    run_at: string;
    attempt_count: number;
    max_attempts: number;
    next_run_at: string;
    locked_by: string;
    locked_at: string;
    cancel_requested: boolean;
    idempotency_key: string;
    schedule_id: number;
    created_at: string;
    updated_at: string;
}
export interface CreateJobRequest {
    queue_id: number;
    url: string;
    method: string;
    headers?: Record<string, string>;
    body?: unknown;
    timeout_ms?: number;
    max_attempts: number;
    run_at?: string;  // skip in form for now
}
export interface TokenRequest{
    tenant_id: number;
}
export interface TokenResponse{
    token: string;
    expires_at: string;
}
export interface ListJobsParams{
    queue_id: number;
    state?: JobState;
    limit?: number;
}
export interface JobAttempt {
    attempt_number: number;
    worker_id: string;
    started_at: string;
    finished_at: string;
    success: string;       
    http_status: string;
    error_message: string;
    response_snippet: string;
  }



