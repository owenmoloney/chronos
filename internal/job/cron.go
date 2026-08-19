package job

import(
	"time"
)

type CronDefinition struct{
	ID		int64
	TenantID int64
	QueueID 	int64
	CronExpr	string
	Timezone	string
	URL string  			
	Method string  		    	
	Headers map[string]string	
	Body []byte
	Timeout time.Duration
	MaxAttempts	int
	Enabled bool
	LastEnqueuedAt	time.Time
}

