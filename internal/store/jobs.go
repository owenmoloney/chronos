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

	var scheduleID any
	if 	j.ScheduleID != 0{
		scheduleID = j.ScheduleID
	}

	row := s.pool.QueryRow(ctx,`
	INSERT INTO jobs (
	  tenant_id, queue_id,
      url, method, headers, body, timeout_ms,
      state, run_at, attempt_count, max_attempts, next_run_at,
      locked_by, locked_at,
      cancel_requested, idempotency_key, schedule_id
    ) VALUES (
	  $1, $2,
      $3, $4, $5, $6, $7,
      $8, $9, $10, $11, $12,
      $13, $14,
      $15, $16, $17
	)
	RETURNING id, created_at, updated_at
	`,
	  j.TenantId, j.QueueID,
      j.HTTP.URL, j.HTTP.Method, headersJSON, j.HTTP.Body, timeoutMs,
      state, j.Lifecycle.RunAt, j.Lifecycle.AttemptCount, j.Lifecycle.MaxAttempts,
      next_run_at,lockedBy,lockedAt,
      j.Cancel.CancelRequested, idemKey, scheduleID,
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

func (s *Store) MarkRunnable(ctx context.Context, id int64) (job.Job, error){
	j, err := s.GetJob(ctx, id)
	if err != nil{
		return job.Job{}, err
	}

	if !job.ValidTransition(j.Lifecycle.State, job.StateRunnable){
		return job.Job{}, fmt.Errorf("Cannot Mark job %d runnable from %s", id, j.Lifecycle.State)
	}

	tag, err := s.pool.Exec(ctx, `
	UPDATE jobs
	SET state = $1, updated_at = now()
	WHERE id = $2 AND state =$3
	`, string(job.StateRunnable), id, string(job.StatePending))

	if err !=nil{
		return job.Job{}, err
	}

	if tag.RowsAffected() ==0{
		return job.Job{}, fmt.Errorf("job %d not updated (not pending?)", id)
	}

	return s.GetJob(ctx, id)

}

func (s *Store) ReplayJob(ctx context.Context, id int64) (job.Job, error){
	j, err := s.GetJob(ctx, id)

	if err != nil{
		return job.Job{}, err
	}

	if !job.ValidTransition(j.Lifecycle.State, job.StateRunnable){
		return job.Job{}, fmt.Errorf("Cannot Replay job %d from %s", id, j.Lifecycle.State)
	}

	tag, err := s.pool.Exec(ctx, `
	UPDATE jobs
	SET state = $1, 
		attempt_count = 0,
    	locked_by = NULL,
    	locked_at = NULL,
    	run_at = now(),
		updated_at = now()
	WHERE id = $2 
		AND state =$3
	`, string(job.StateRunnable), id, string(job.StateDeadLettered))

	if err !=nil{
		return job.Job{}, err
	}

	if tag.RowsAffected() ==0{
		return job.Job{}, fmt.Errorf("job %d not replayed (not dead_lettered?)", id)
	}

	return s.GetJob(ctx, id)

}
	

func (s *Store) CancelJob(ctx context.Context, id int64) (job.Job, error){
	j, err := s.GetJob(ctx, id)
	if err != nil{
		return job.Job{}, err
	}

	switch j.Lifecycle.State{
	case job.StatePending, job.StateRunnable:
		tag, err :=  s.pool.Exec(ctx, `
		UPDATE jobs
		SET state = $1, 
			locked_by = NULL, 
			locked_at = NULL,
			updated_at = now()
		WHERE id = $2
			AND state = ANY($3)
		`, string(job.StateCanceled), id,
			[]string{string(job.StatePending), string(job.StateRunnable)})

	if err != nil {
		return job.Job{}, err
	}
	if tag.RowsAffected() == 0 {
		return job.Job{}, fmt.Errorf("job %d not canceled", id)
	}
	case job.StateRunning:
		tag, err := s.pool.Exec(ctx, `
			UPDATE jobs
			SET cancel_requested = true,
				updated_at = now()
			WHERE id = $1 AND state = $2
		`, id, string(job.StateRunning))
		if err != nil {
			return job.Job{}, err
		}
		if tag.RowsAffected() == 0 {
			return job.Job{}, fmt.Errorf("job %d not canceled", id)
		}
	default:
	return job.Job{}, fmt.Errorf("cannot cancel job %d from state %s", id, j.Lifecycle.State)
	}
	return s.GetJob(ctx, id) // return fresh row after UPDATE
}

func (s *Store) ClaimJob(ctx context.Context, workerID string, queueID int64)(job.Job, bool, error){
	if workerID == ""{
		return job.Job{}, false, fmt.Errorf("workedID required")
	}

	tx, err := s.pool.Begin(ctx)

	if err !=nil{
		return job.Job{}, false, err
	}

	defer tx.Rollback(ctx)

	var id int64

	err = tx.QueryRow(ctx, `
		SELECT id FROM jobs
		WHERE queue_id = $1
			AND state  = $2
			AND run_at <= now()
		ORDER BY run_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, queueID, string(job.StateRunnable)).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows){
		return job.Job{}, false, nil
	}

	if err != nil{
		return job.Job{}, false, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE jobs
		SET state = $1,
			locked_by =  $2,
			locked_at = now(),
			updated_at = now()
		WHERE id = $3
		`, string(job.StateRunning), workerID, id)

	if err != nil{
		return job.Job{}, false, err
	}

	err = tx.Commit(ctx)

	if err != nil{
		return job.Job{}, false, err
	}

	j, err := s.GetJob(ctx, id)

	if err != nil{
		return job.Job{}, false, err
	}

	return j, true, nil
	
}

func (s *Store) CompleteJob(ctx context.Context, jobID int64, workerID string, httpStatus int, snippet string) (error){


	tx, err := s.pool.Begin(ctx)

	if err !=nil{
		return err
	}

	defer tx.Rollback(ctx)

	var attemptCount int

	err = tx.QueryRow(ctx, `
		UPDATE jobs
		SET state = $1,                   
    		locked_by = NULL,
    		locked_at = NULL,
    		attempt_count = attempt_count + 1,
    		updated_at = now()
		WHERE id = $2
  			AND state = $3                   
  			AND locked_by = $4               
		RETURNING attempt_count
	`, string(job.StateSucceded), jobID, string(job.StateRunning),workerID,
	).Scan(&attemptCount)

	if errors.Is(err, pgx.ErrNoRows){
		return fmt.Errorf("complete job %d: not running or wrong worker", jobID)
	}

	if err != nil{
		return err
	}

	
	_, err = tx.Exec(ctx, `
		INSERT INTO job_attempts (
    		job_id,
    		attempt_number,
    		worker_id,
    		success,
    		http_status,
    		response_snippet,
    		finished_at
		)	VALUES ($1, $2, $3, $4, $5, $6, now())
		`, jobID, attemptCount, workerID, "true", fmt.Sprintf("%d", httpStatus), snippet,
		)

	if err != nil{
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) FailJob(ctx context.Context, jobID int64, workerID string, httpStatus int, snippet, errMsg string) (error){
	tx, err := s.pool.Begin(ctx)

	if err !=nil{
		return err
	}

	defer tx.Rollback(ctx)

	var attemptCount int
	var newState string

	err = tx.QueryRow(ctx, `
		UPDATE jobs
		SET 
			attempt_count = attempt_count +1,
    		locked_by = NULL,
    		locked_at = NULL,
			state = CASE
				WHEN attempt_count +1 >= max_attempts THEN $1
				ELSE $2
			END,
			run_At = CASE
				WHEN attempt_count +1 >= max_attempts THEN run_at
				ELSE now()
			END,
    		updated_at = now()
		WHERE id = $3
  			AND state = $4                   
  			AND locked_by = $5              
		RETURNING attempt_count, state
	`, string(job.StateDeadLettered), string(job.StateRunnable), jobID, string(job.StateRunning),workerID,
	).Scan(&attemptCount, &newState)


	if errors.Is(err, pgx.ErrNoRows){
		return fmt.Errorf("fail job %d: not running or wrong worker", jobID)
	}

	if err != nil{
		return err
	}
	
	_, err = tx.Exec(ctx, `
		INSERT INTO job_attempts (
    		job_id,
    		attempt_number,
    		worker_id,
    		success,
			http_status,
    		response_snippet,
    		error_message,
    		finished_at
		)	VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		`, jobID, attemptCount, workerID, "false", fmt.Sprintf("%d", httpStatus), snippet, errMsg,
		)

	if err != nil{
		return err
	}

	if newState == string(job.StateRunnable){
		runAt := time.Now().UTC().Add(job.RetryDelay(attemptCount))
		_, err = tx.Exec(ctx, 
			`UPDATE jobs 
			SET run_at = $1 
			WHERE id = $2
			`, runAt, jobID,
		)

		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Store) ReclaimStaleJobs(ctx context.Context, olderThan time.Duration) (int64, error){
	if olderThan <= 0 {
        return 0, fmt.Errorf("olderThan must be positive")
    }

	tag, err := s.pool.Exec(ctx, `
		UPDATE jobs
		SET state = $1,
			locked_by = NULL,
			locked_at = NULL,
			updated_at = now()
		WHERE state = $2
			AND locked_at IS NOT NULL
			AND locked_at < now() - ($3 * interval '1 second')
			`, string(job.StateRunnable), string(job.StateRunning), int64(olderThan.Seconds()))

	if err != nil{
		return 0, err
	}
	return tag.RowsAffected(), nil

}

func (s *Store) AcknowledgeCancel(ctx context.Context, jobID int64, workerID string) (error){
	
	tag, err := s.pool.Exec(ctx,`
	UPDATE jobs
	SET state = $1,
    	locked_by = NULL,
    	locked_at = NULL,
    	updated_at = now()
	WHERE id = $2
  		AND state = $3
  		AND locked_by = $4
  		AND cancel_requested = true
	`,string(job.StateCanceled), jobID, string(job.StateRunning), workerID)


	if err != nil {
		return err
	}


	if tag.RowsAffected() == 0{
		return fmt.Errorf("acknowledge cancel job %d: not running, wrong worker, or not requested", jobID)
	}
	return nil
}

func (s *Store) Heartbeat(ctx context.Context, jobID int64, workerID string) (error){
	
	tag, err := s.pool.Exec(ctx,`
	UPDATE jobs
	SET locked_at = now(),
    	updated_at = now()
	WHERE id = $1
  		AND state = $2
  		AND locked_by = $3
	`,jobID, string(job.StateRunning), workerID)


	if err != nil {
		return err
	}


	if tag.RowsAffected() == 0{
		return fmt.Errorf("heartbeat job %d: ot running, wrong worker, or not requested", jobID)
	}
	return nil
}