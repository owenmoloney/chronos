package worker

import (
	"context"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/owenmoloney/chronos/internal/execute"
)

func RunOnce(ctx context.Context, s *store.Store, workerID string, queueID int64) (didWork bool, err error){
	j, ok, err:= s.ClaimJob(ctx, workerID, queueID)

	if err != nil{
		return false, err
	}

	if !ok {
		return false, nil
	}

	result := execute.ExecuteHTTP(ctx, j.HTTP)

	if result.Err != nil || result.StatusCode < 200 || result.StatusCode >= 300 {
		errMsg := "non-2xx status"

		if result.Err != nil{
			errMsg = result.Err.Error()
		}

		if err:= s.FailJob(ctx, j.ID, workerID, result.StatusCode, result.Snippet, errMsg); err != nil{
			return true, err
		}

		return true, nil
	}

	if err := s.CompleteJob(ctx, j.ID, workerID, result.StatusCode, result.Snippet); err != nil {
		return true, err
	}
	return true, nil
}

