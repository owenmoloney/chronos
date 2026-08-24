package job
import "time"

type Job struct {

	//Identity
	ID int64
	TenantId int64
	QueueID int64

	HTTP HTTP

	Lifecycle Lifecycle

	Claim Claim

	Cancel Cancel

	Idempotency Idempotency

	Timestamps Timestamps

	ScheduleID int64
}
	//HTTP task
type HTTP struct{
	URL string  			
	Method string  		    	
	Headers map[string]string	
	Body []byte
	Timeout time.Duration
}

type Lifecycle struct{
	State	State
	RunAt	time.Time
	AttemptCount int 
	MaxAttempts	int
	NextRunAt  time.Time
}

type Claim struct{
	LockedBy  string
	LockedAt  *time.Time
}

type Cancel struct{
	CancelRequested bool
}
type Idempotency struct {
	IdempotencyKey string
}
type Timestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}
