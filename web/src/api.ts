import { request } from './request.js';
import { type JobAttempt } from './types.js';

import type {Job, ListJobsParams, TokenResponse, CreateJobRequest} from './types.js';

export async function getToken(tenantId: number): Promise<TokenResponse>{

    const path = '/auth/token';
    const method = 'POST';

    return request<TokenResponse>(path, {
        method, 
        body: { tenant_id: tenantId } 
    });
}

export async function listJobs(params: ListJobsParams, token: string, ): Promise<Job[]>{
    const search = new URLSearchParams();
    search.append('queue_id',String(params.queue_id));

    if (params.state !== undefined){
        search.append('state',String(params.state));
    };

    if (params.limit !== undefined){
        search.append('limit',String(params.limit));
    };

    const path = `/jobs?${search.toString()}`;

    return request<Job[]>(path, {
        token
    });
}

export async function getJob(token: string, id: number): Promise<Job>{
    const path = `/jobs/${id}`;
    return request<Job>(path, {token});
}

export async function replayJob(token: string, id: number): Promise<Job>{
    const path = `/jobs/${id}/replay`;

    const method ='POST';

    return request<Job>(path, { method: 'POST', token });
}

export async function cancelJob(token: string, id: number): Promise<Job>{
    const path = `/jobs/${id}/cancel`;
    return request<Job>(path, {method: 'POST', token});
}

export async function createJob(
    token: string,
    body: CreateJobRequest,
    idempotencyKey?: string,
  ): Promise<Job> {
    return request<Job>('/jobs', {
      method: 'POST',
      token,
      body,
      ...(idempotencyKey
        ? { headers: { 'Idempotency-Key': idempotencyKey } }
        : {}),
    });
  }

export async function listJobAttempts(
    token: string,
    jobId: number,
): Promise<JobAttempt[]>{
    return request<JobAttempt[]>(`/jobs/${jobId}/attempts`, {token});
}