package store

import(
	"time"
	"context"
	"fmt"
	"encoding/json"
	"github.com/owenmoloney/chronos/internal/job"
	"errors"
	"github.com/jackc/pgx/v5" 

)
var ErrCronAlreadyClaimed = errors.New("cron already claimed")

func scanCronDefinition(r scannable) (job.CronDefinition, error){
	var(
		id 				int64
		tenantID 		int64
		queueID 		int64
		cron_expr		string
		time_zone		string
		url				string
		method 			string
		headersJSON		[]byte
		body 			[]byte
		timeout_ms		int64
		maxAttempts 	int
		enabled			bool
		last_enqueued_at	 time.Time
	)
	err := r.Scan(
		&id, &tenantID, &queueID, &cron_expr, &time_zone,
    	&url, &method, &headersJSON, &body, &timeout_ms,
    	&maxAttempts, &enabled, &last_enqueued_at,
	)

	if err != nil{
		return job.CronDefinition{}, err
	}

	headers := map[string]string{}

	if len(headersJSON) > 0{
		err = json.Unmarshal(headersJSON, &headers)
	}

	if err != nil{
		return job.CronDefinition{}, err
	}

	def := job.CronDefinition{}
	def.ID =							id
	def.TenantID = 					tenantID
	def.QueueID = 					queueID
	def.CronExpr =					cron_expr
	def.Timezone = 					time_zone
	def.URL = 					url
	def.Method = 				method
	def.Headers =				headers
	def.Body = 					body		
	def.Timeout = 				time.Duration(timeout_ms) * time.Millisecond
	def.MaxAttempts = 		maxAttempts 
	def.Enabled = 					enabled
	def.LastEnqueuedAt = 		 		last_enqueued_at
	

	return def, nil
}

func (s *Store) ListEnabledCronDefinitions(ctx context.Context) ([]job.CronDefinition, error){


	rows, err := s.pool.Query(ctx,`
		SELECT id, tenant_id, queue_id, cron_expr, timezone,
       		url, method, headers, body, timeout_ms, max_attempts,
       		enabled, last_enqueued_at
		FROM cron_definitions
		WHERE enabled = true`)

	if err != nil{
		return nil, err
	}

	defer rows.Close()

	var defs []job.CronDefinition

	for rows.Next(){
		def, err := scanCronDefinition(rows)

		if err != nil{
			return nil, err
		}
		defs = append(defs, def)
	}

	return defs, rows.Err()
}


func (s *Store) UpdateCronLastEnqueued(ctx context.Context, id int64, at time.Time, expectedLast time.Time) error{


	tag, err := s.pool.Exec(ctx, `
	UPDATE cron_definitions
	SET last_enqueued_at = $1, updated_at = now()
	WHERE id = $2 AND last_enqueued_at = $3
	`, at, id, expectedLast)

	if err !=nil{
		return err
	}

	if tag.RowsAffected() == 0{
		return fmt.Errorf("cron %d: %w", id, ErrCronAlreadyClaimed)
	}

	return err

}

func (s *Store) EnqueueCronJob(ctx context.Context, def job.CronDefinition, now time.Time, expectedLast time.Time) error{
	tx, err := s.pool.Begin(ctx)

	if err != nil{
		return err
	}

	
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE cron_definitions
		SET last_enqueued_at = $1, updated_at = now()
		WHERE id = $2 AND last_enqueued_at = $3
	`, now, def.ID, expectedLast)

	if err != nil{
		return err
	}

	if tag.RowsAffected()==0 {
		return fmt.Errorf("cron %d: %w", def.ID, ErrCronAlreadyClaimed)
	}

	var j job.Job
	j.TenantId = def.TenantID
	j.QueueID = def.QueueID
	j.HTTP.URL = def.URL
	j.HTTP.Method = def.Method
	j.HTTP.Headers = def.Headers
	j.HTTP.Body = def.Body
	j.HTTP.Timeout = def.Timeout
	j.Lifecycle.State = job.StatePending
	j.Lifecycle.MaxAttempts = def.MaxAttempts
	j.Lifecycle.RunAt = now
	j.ScheduleID = def.ID

	headersJSON, timeoutMs, state, err := flattenForInsert(j)
	if err != nil {
		return err
	}

	var next_run_at any
	if !j.Lifecycle.NextRunAt.IsZero() {
		next_run_at = j.Lifecycle.NextRunAt
	}
	var lockedBy any
	if j.Claim.LockedBy != "" {
		lockedBy = j.Claim.LockedBy
	}
	lockedAt := j.Claim.LockedAt
	var idemKey any
	if j.Idempotency.IdempotencyKey != "" {
		idemKey = j.Idempotency.IdempotencyKey
	}
	var scheduleID any
	if j.ScheduleID != 0 {
		scheduleID = j.ScheduleID
	}

	row := tx.QueryRow(ctx, `
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
		return err
	}

	tag, err = tx.Exec(ctx, `
		UPDATE jobs
		SET state = $1, updated_at = now()
		WHERE id = $2 AND state = $3
	`, string(job.StateRunnable), j.ID, string(job.StatePending))

	if err != nil{
		return err
	}

	if tag.RowsAffected()==0 {
		return fmt.Errorf("cron %d: job %d not marked runnable", def.ID, j.ID)
	}

	return tx.Commit(ctx)



}

func (s *Store) ListCronDefinitions(ctx context.Context, tenantID int64)([]job.CronDefinition, error){
	
	rows, err := s.pool.Query(ctx,`
		SELECT id, tenant_id, queue_id, cron_expr, timezone,
       		url, method, headers, body, timeout_ms, max_attempts,
       		enabled, last_enqueued_at
		FROM cron_definitions
		WHERE tenant_id =$1
		ORDER BY id DESC
	`, tenantID)

	if err != nil{
		return nil, err
	}

	defer rows.Close()

	var defs []job.CronDefinition

	for rows.Next(){
		def, err := scanCronDefinition(rows)

		if err != nil{
			return nil, err
		}
		defs = append(defs, def)
	}

	return defs, rows.Err()
}

func (s *Store) GetCronDefinition(ctx context.Context, tenantID int64, id int64)(job.CronDefinition, error){
	rows := s.pool.QueryRow(ctx,`
		SELECT id, tenant_id, queue_id, cron_expr, timezone,
       		url, method, headers, body, timeout_ms, max_attempts,
       		enabled, last_enqueued_at
		FROM cron_definitions
		WHERE id = $1 AND tenant_id = $2
		ORDER BY id DESC`, id, tenantID)

	
	def, err := scanCronDefinition(rows)

	if err!= nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job.CronDefinition{}, fmt.Errorf("cron %d not found: %w", id, err)
		}
		return job.CronDefinition{}, err
	}
	return def, nil
}

func (s *Store) CreateCronDefinition(ctx context.Context, def job.CronDefinition)(job.CronDefinition, error){
	if def.Timezone == ""{
		def.Timezone = "UTC"
	}

	if def.Method == ""{
		def.Method ="GET"
	}

	if def.Timeout == 0 {
		def.Timeout = 30 *time.Second
	}

	if def.MaxAttempts == 0{
		def.MaxAttempts = 3
	}

	if def.LastEnqueuedAt.IsZero(){
		def.LastEnqueuedAt = time.Unix(0, 0).UTC()
	}

	timeoutMs := int64(def.Timeout / time.Millisecond)

	if def.Headers == nil{
		def.Headers = make(map[string]string)
	}

	headersJSON, err := json.Marshal(def.Headers)

	if err!=nil{
		return job.CronDefinition{}, err
	}

	query :=`
			INSERT INTO cron_definitions (
				tenant_id, 
				queue_id, 
				cron_expr, 
				timezone,
				url, 
				method, 
				headers, 
				body, 
				timeout_ms, 
				max_attempts,
				enabled, 
				last_enqueued_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING 
				id,
				tenant_id, 
				queue_id, 
				cron_expr, 
				timezone,
				url, 
				method, 
				headers, 
				body, 
				timeout_ms, 
				max_attempts,
				enabled, 
				last_enqueued_at;
	`
	row := s.pool.QueryRow(
		ctx, 
		query,
		def.TenantID,
		def.QueueID,
		def.CronExpr,
		def.Timezone,
		def.URL,
		def.Method,
		headersJSON,
		def.Body, 
		timeoutMs,
		def.MaxAttempts,
		def.Enabled,
		def.LastEnqueuedAt,
	)

	var created job.CronDefinition
	created, err = scanCronDefinition(row)
	if err != nil {
		return job.CronDefinition{}, err
	}
	return created, nil
}

func (s *Store) SetCronEnabled(ctx context.Context, tenantID int64, id int64, enabled bool)(job.CronDefinition, error){
	tag, err := s.pool.Exec(ctx,`
		UPDATE cron_definitions
		SET enabled = $1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3`,enabled, id, tenantID)

		if err != nil{
			return job.CronDefinition{}, err
		}

		if tag.RowsAffected() == 0{
			return job.CronDefinition{}, fmt.Errorf("cron %d not found: %w", id, err)
		}

		return s.GetCronDefinition(ctx, tenantID, id)

}


