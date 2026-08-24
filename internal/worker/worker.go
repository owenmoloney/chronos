package worker

import (
	"context"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/owenmoloney/chronos/internal/execute"
	"github.com/owenmoloney/chronos/internal/observe"
	"time"
)

func RunOnce(ctx context.Context, s *store.Store, workerID string, queueID int64) (didWork bool, err error){
	j, ok, err:= s.ClaimJob(ctx, workerID, queueID)

	if err != nil{
		return false, err
	}

	if !ok {
		return false, nil
	}

	observe.JobsClaimed.Inc()

	j, err = s.GetJob(ctx, j.ID)
	if err != nil{
		return true, err
	}

	if j.Cancel.CancelRequested{
		if err := s.AcknowledgeCancel(ctx, j.ID, workerID); err != nil{
			return true, err
		}
		return true, nil
	}

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = s.Heartbeat(ctx, j.ID, workerID) // V1: log later; don't fail the job on one miss
			}
		}
	}()
	result := execute.ExecuteHTTP(ctx, j.HTTP)
	close(done)

	
	if result.Err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		errMsg := "non-2xx status"

		if result.Err != nil{
			errMsg = result.Err.Error()
		}

		if err:= s.FailJob(ctx, j.ID, workerID, result.StatusCode, result.Snippet, errMsg); err != nil{
			return true, err
		}

		observe.JobsFailed.Inc()
		return true, nil
	}

	if err := s.CompleteJob(ctx, j.ID, workerID, result.StatusCode, result.Snippet); err != nil {
		return true, err
	}

	observe.JobsCompleted.Inc()
	return true, nil
}


