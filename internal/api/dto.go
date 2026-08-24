package api 

import(
	"time"
    "encoding/json"
)

type CreateJobRequest struct{
	QueueId 		int64					`json:"queue_id"`
	Url				string					`json:"url"`
	Method 			string					`json:"method"`
	Headers			map[string]string		`json:"headers"`
	Body			json.RawMessage			`json:"body"`
	TimeoutMs		int64					`json:"timeout_ms"`
	MaxAttempts 	int64					`json:"max_attempts"`
	RunAt			time.Time				`json:"run_at"`
}
type JobResponse struct{
	Id 						int64					`json:"id"`
	TenantId				int64					`json:"tenant_id"`
	QueueId 				int64					`json:"queue_id"`
	Url						string					`json:"url"`
	Headers					map[string]string		`json:"headers"`
	Body					json.RawMessage			`json:"body"`
	TimeoutMs				int64					`json:"timeout_ms"`
	Method 					string					`json:"method"`
	State					string					`json:"state"`
	RunAt          			time.Time				`json:"run_at"`
    AttemptCount   			int64					`json:"attempt_count"`
    MaxAttempts    			int64					`json:"max_attempts"`
    NextRunAt     			time.Time				`json:"next_run_at"`
    LockedBy       			string					`json:"locked_by"`
    LockedAt       			time.Time				`json:"locked_at"`
    CancelRequested			bool	 				`json:"cancel_requested"`
    IdempotencyKey     		string					`json:"idempotency_key"`
    ScheduleId         		int64					`json:"schedule_id"`
    CreatedAt         		time.Time				`json:"created_at"`
    UpdatedAt          		time.Time				`json:"updated_at"`
	ScheduleID				int64					`json:schedule_id`
}

type TokenRequest struct{
	TenantId		int64	`json:"tenant_id"`
}

type TokenResponse struct{
	Token			string		`json:"token"`
	ExpiresAt		time.Time	`json:"expires_at"`
}