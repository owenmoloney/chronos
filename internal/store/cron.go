package store

import(
	"time"
	"context"
	"fmt"
	"encoding/json"
	"github.com/owenmoloney/chronos/internal/job"
)
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


func (s *Store) UpdateCronLastEnqueued(ctx context.Context, id int64, at time.Time) error{


	tag, err := s.pool.Exec(ctx, `
	UPDATE cron_definitions
	SET last_enqueued_at = $1, updated_at = now()
	WHERE id = $2 
	`, at, id)

	if err !=nil{
		return err
	}

	if tag.RowsAffected() == 0{
		return fmt.Errorf("Cron %d nto found)", id)
	}

	return err

}