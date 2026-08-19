package scheduler

import(
	"context"
	"time"
	"github.com/owenmoloney/chronos/internal/job"
)

func jobFromCron(def job.CronDefinition) job.Job{
	var j job.Job
    j.TenantId              = def.TenantID
    j.QueueID               = def.QueueID
    j.HTTP.URL              = def.URL
    j.HTTP.Method           = def.Method
    j.HTTP.Headers          = def.Headers
    j.HTTP.Body             = def.Body
    j.HTTP.Timeout          = def.Timeout
    j.Lifecycle.State       = job.StatePending
    j.Lifecycle.MaxAttempts = def.MaxAttempts
    j.Lifecycle.RunAt       = time.Now().UTC()
    return j
}
func (s *Scheduler) enqueueDue(ctx context.Context,  def job.CronDefinition, now time.Time) (err error){
    j := jobFromCron(def)

    created, err := s.store.CreateJob(ctx, j)
    if err != nil{
        return err
	}
    _, err = s.store.MarkRunnable(ctx, created.ID)
    if err != nil{
        return err
	}
    return s.store.UpdateCronLastEnqueued(ctx, def.ID, now)
}