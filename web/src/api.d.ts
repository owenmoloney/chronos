import { type JobAttempt } from './types.js';
import type { Job, ListJobsParams, TokenResponse, CreateJobRequest } from './types.js';
export declare function getToken(tenantId: number): Promise<TokenResponse>;
export declare function listJobs(params: ListJobsParams, token: string): Promise<Job[]>;
export declare function getJob(token: string, id: number): Promise<Job>;
export declare function replayJob(token: string, id: number): Promise<Job>;
export declare function cancelJob(token: string, id: number): Promise<Job>;
export declare function createJob(token: string, body: CreateJobRequest, idempotencyKey?: string): Promise<Job>;
export declare function listJobAttempts(token: string, jobId: number): Promise<JobAttempt[]>;
