package store

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"  
	"errors" 
	"github.com/owenmoloney/chronos/internal/job"
	"time"
)

type IdempotencyRecord struct {
	TenantID 		int64
	Key 			string 
	JobId			int64
	RequestHash 	string 
}

func (s *Store) GetIdempotency(ctx context.Context, tenantID int64, key string) (IdempotencyRecord, error){
	rows := s.pool.QueryRow(ctx, `
	SELECT
		tenant_id, key, job_id, request_hash
	FROM idempotency_keys
	WHERE tenant_id = $1 AND key = $2
	`, tenantID, key )

	var rec IdempotencyRecord

	err:= rows.Scan(&rec.TenantID, &rec.Key, &rec.JobId, &rec.RequestHash)

	if err != nil{
		if errors.Is(err, pgx.ErrNoRows) {
			return IdempotencyRecord{}, fmt.Errorf("idempotency key %q not found: %w", key, err)
		}
		return IdempotencyRecord{}, err
	}
	return rec, nil
}

func (s *Store) SaveIdempotency(ctx context.Context, rec IdempotencyRecord)	(error){
	_, err := s.pool.Exec(ctx, `
	INSERT INTO idempotency_keys (tenant_id, key, job_id, request_hash)
	VALUES($1, $2, $3, $4)
	`, rec.TenantID, rec.Key, rec.JobId, rec.RequestHash)

	return err
}



func(s *Store) CreateJobWithIdempotency(ctx context.Context, j job.Job, key string, requestHash string) (job.Job, error){
	tx, err := s.pool.Begin(ctx)
	
	if err!=nil{
		return job.Job{}, err
	}

	defer tx.Rollback(ctx)

	if j.Lifecycle.State == ""{
		j.Lifecycle.State = job.StatePending
	}

	if j.Lifecycle.MaxAttempts == 0{
		j.Lifecycle.MaxAttempts = 3
	}

	if j.Lifecycle.RunAt.IsZero(){
		j.Lifecycle.RunAt = time.Now().UTC()
	}

	headersJSON, timeoutMs, state, err := flattenForInsert(j)

	if(err !=nil){
		return job.Job{}, err
	}
	var next_run_at any 

	if !j.Lifecycle.NextRunAt.IsZero(){
		next_run_at = j.Lifecycle.NextRunAt
	}

	var lockedBy any

	if j.Claim.LockedBy != ""{
		lockedBy = j.Claim.LockedBy
	}

	lockedAt := j.Claim.LockedAt

	var idemKey any

	j.Idempotency.IdempotencyKey = key

	if j.Idempotency.IdempotencyKey != ""{
		idemKey = j.Idempotency.IdempotencyKey
	}

	row := tx.QueryRow(ctx,`
	INSERT INTO jobs (
	  tenant_id, queue_id,
      url, method, headers, body, timeout_ms,
      state, run_at, attempt_count, max_attempts, next_run_at,
      locked_by, locked_at,
      cancel_requested, idempotency_key
    ) VALUES (
	  $1, $2,
      $3, $4, $5, $6, $7,
      $8, $9, $10, $11, $12,
      $13, $14,
      $15, $16 
	)
	RETURNING id, created_at, updated_at
	`,
	  j.TenantId, j.QueueID,
      j.HTTP.URL, j.HTTP.Method, headersJSON, j.HTTP.Body, timeoutMs,
      state, j.Lifecycle.RunAt, j.Lifecycle.AttemptCount, j.Lifecycle.MaxAttempts,
      next_run_at,lockedBy,lockedAt,
      j.Cancel.CancelRequested, idemKey,
   )
    err = row.Scan(&j.ID, &j.Timestamps.CreatedAt, &j.Timestamps.UpdatedAt)

	if err != nil{
		return job.Job{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (tenant_id, key, job_id, request_hash)
		VALUES ($1, $2, $3, $4)
  		`, j.TenantId, key, j.ID, requestHash)

	if err != nil{
		return job.Job{}, err
	}

	err = tx.Commit(ctx)

	if err != nil{
		return job.Job{}, err
	}



	j.Lifecycle.State = job.State(state)

	return j, nil
}