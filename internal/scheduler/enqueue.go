package scheduler

import(
	"context"
	"time"
	"github.com/owenmoloney/chronos/internal/job"
    "errors"
    "github.com/owenmoloney/chronos/internal/store"
    "github.com/owenmoloney/chronos/internal/observe"
)

func (s *Scheduler) enqueueDue(ctx context.Context,  def job.CronDefinition, now time.Time, expectedLast time.Time) (err error){
    err = s.store.EnqueueCronJob(ctx, def, now, expectedLast)
    if errors.Is(err, store.ErrCronAlreadyClaimed) {
        return nil
    }
    if err != nil {
        return err
    }
    observe.CronEnqueued.Inc()
    return nil
}