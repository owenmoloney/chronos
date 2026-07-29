package store
import (
	"encoding/json"
	"github.com/owenmoloney/chronos/internal/job"
	"time"
	"context"
	"errors"   
	"github.com/jackc/pgx/v5"
	"fmt"
)

func flattenForInsert(j job.Job) (headersJSON []byte, timeoutMs int64, state string, err error){
	
	headers := j.HTTP.Headers

	if  headers == nil{
		headers = map[string]string{}
	}

	//headersJSON, err := json.Marshal(headers)
	headersJSON, err = json.Marshal(headers)

	if err != nil{
		return nil, 0, "", err
	}	

	timeoutMs = j.HTTP.Timeout.Milliseconds()

	state = string(j.Lifecycle.State)
	if state == "" {
		state = "pending"
	}

	return headersJSON, timeoutMs, state, nil

}
type scannable interface{
	Scan(dest ...any) error
}

func scanJob(r scannable) (job.Job, error){
	var(
		id 				int64
		tenantID 		int64
		queueID 		int64
		url				string
		method 			string
		stateStr		string
		headersJSON		[]byte
		body 			[]byte
		timeoutMs		int64
		runAt 			time.Time
		createdAt 		time.Time
		updatedAt 		time.Time
		attemptCount	int
		maxAttempts 	int
		nextRunAt 		*time.Time
		lockedBy 		*string
		lockedAt 		*time.Time
		cancelRequested bool
		idempotencyKey	*string
	)

	err := r.Scan(
		&id, &tenantID, &queueID,
    	&url, &method, &headersJSON, &body, &timeoutMs,
    	&stateStr, &runAt, &attemptCount, &maxAttempts, &nextRunAt,
    	&lockedBy, &lockedAt,
    	&cancelRequested, &idempotencyKey,
    	&createdAt, &updatedAt,
	)

	if err != nil{
		return job.Job{}, err
	}

	headers := map[string]string{}

	if len(headersJSON) > 0{
		err = json.Unmarshal(headersJSON, &headers)
	}

	if err != nil{
		return job.Job{}, err
	}

	j := job.Job{}
	j.ID =							id
	j.TenantId = 					tenantID
	j.QueueID = 					queueID
	j.HTTP.URL = 					url
	j.HTTP.Method = 				method
	j.HTTP.Headers =				headers
	j.HTTP.Body = 					body		
	j.HTTP.Timeout = 				time.Duration(timeoutMs) *time.Millisecond
	j.Lifecycle.State = 			job.State(stateStr)
	j.Lifecycle.RunAt =  			runAt 			
	j.Lifecycle.AttemptCount = 		attemptCount 		
	j.Lifecycle.MaxAttempts =   	maxAttempts		
	j.Claim.LockedAt = 				lockedAt

	if nextRunAt != nil{
		j.Lifecycle.NextRunAt = *nextRunAt
	}

	if lockedBy != nil{
		j.Claim.LockedBy = *lockedBy
	}

	j.Cancel.CancelRequested = cancelRequested

	if idempotencyKey != nil{
		j.Idempotency.IdempotencyKey = *idempotencyKey
	}

	j.Timestamps.CreatedAt = createdAt
	j.Timestamps.UpdatedAt = updatedAt


	return j, nil
}

func (s *Store) GetJob(ctx context.Context, id int64) (job.Job, error){
	row := s.pool.QueryRow(ctx,`
	SELECT
		id, tenant_id, queue_id,
    	url, method, headers, body, timeout_ms,
    	state, run_at, attempt_count, max_attempts, next_run_at,
    	locked_by, locked_at,
    	cancel_requested, idempotency_key,
    	created_at, updated_at
    FROM jobs
    WHERE id = $1
	`, id)

	j, err := scanJob(row)

	if err != nil{
		if errors.Is(err,pgx.ErrNoRows){
			return job.Job{}, fmt.Errorf("Job %d not found: %w", id, err)
		}
	return job.Job{}, err
	}
	return j,nil
}

func (s *Store) CreateJob(ctx context.Context, j job.Job) (job.Job, error){

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

	if j.Idempotency.IdempotencyKey != ""{
		idemKey = j.Idempotency.IdempotencyKey
	}

	row := s.pool.QueryRow(ctx,`
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

	j.Lifecycle.State = job.State(state)

	return j, nil
}

func (s *Store) ListJobsByQueue(ctx context.Context, queueID int64)([]job.Job, error){
	rows, err := s.pool.Query(ctx,`
		SELECT
        id, tenant_id, queue_id,
        url, method, headers, body, timeout_ms,
        state, run_at, attempt_count, max_attempts, next_run_at,
        locked_by, locked_at,
        cancel_requested, idempotency_key,
        created_at, updated_at
      FROM jobs
      WHERE queue_id = $1
      ORDER BY created_at DESC
  	`, queueID)

	if err != nil{
		return nil, err
	}

	defer rows.Close()

	var jobs []job.Job

	for rows.Next(){
		j, err:= scanJob(rows)

		if err != nil{
			return nil, err
		}
		jobs = append(jobs, j)
	}

	if err := rows.Err(); err !=nil{
		return nil, err
	}

	return jobs, nil

}