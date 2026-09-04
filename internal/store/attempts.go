
package store


import (
	"context"
	"time"
)

type JobAttempt struct {
	ID		int64
	JobID 	int64
	AttemptNumber	int 
	WorkerID string
	StartedAt	time.Time
	FinishedAt 		time.Time
	Success		string
	HTTPStatus 	string 
	ErrorMessage	 string
	ResponseSnippet string
}

func (s *Store) ListJobAttempts(ctx context.Context, JobID int64)([]JobAttempt, error){

	rows, err := s.pool.Query(ctx,`
	SELECT
		id, job_id, attempt_number, worker_id,
    	started_at, finished_at, success,
    	http_status, COALESCE(error_message, '') AS error_message, response_snippet
    FROM job_attempts
    WHERE job_id = $1
	ORDER BY attempt_number ASC

	`, JobID)
	
	if err != nil{
		return nil, err
	}
	defer rows.Close()

	var attempts []JobAttempt
	for rows.Next(){
		var a JobAttempt
		err := rows.Scan(&a.ID, &a.JobID, &a.AttemptNumber, &a.WorkerID,
			&a.StartedAt, &a.FinishedAt, &a.Success,
			&a.HTTPStatus, &a.ErrorMessage, &a.ResponseSnippet,
		)
		if err != nil{
			return nil, err
		}
		attempts = append(attempts, a)
	}

	return attempts, rows.Err()
}