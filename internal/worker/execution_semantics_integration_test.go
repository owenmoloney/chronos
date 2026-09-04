package worker_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
	"github.com/owenmoloney/chronos/internal/execute"
	"github.com/owenmoloney/chronos/internal/job"
	"github.com/owenmoloney/chronos/internal/store"
)

type recordedReq struct{
	IdempotencyKey string
}

var (
	mu sync.Mutex
	hits []recordedReq
)

func TestAtLeastOnceExecutionAfterCrashBeforeComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		mu.Lock()
		hits = append(hits, recordedReq{IdempotencyKey: r.Header.Get("Idempotency-Key")})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _  = w.Write([]byte(`ok`)) 
	}))
	defer srv.Close()

	restore := execute.SetURLValidatorForTest(func(string) error { return nil })
	defer restore()

	ctx:= context.Background()

	databaseURL:= os.Getenv("DATABASE_URL")

	if databaseURL == ""{
		databaseURL = "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable"
	}

	pool, err := store.NewPool(ctx, databaseURL)

	if err != nil{
		t.Skipf("postgres not available: %v", err)
	}

	defer pool.Close()

	s := store.New(pool)

	suffix := time.Now().UnixNano()


	tenantName := fmt.Sprintf("test-tenant-%d", suffix)
	queueName := fmt.Sprintf("test-queue-%d", suffix)
	var tenantID int64

	err = pool.QueryRow(ctx,
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`,
		tenantName,
		).Scan(&tenantID)
	
	if err != nil{
		t.Fatalf("insert tenant: %v", err)
	}

	var queueID  int64

	err = pool.QueryRow(ctx,
		 `INSERT INTO queues (tenant_id, name) VALUES($1, $2) RETURNING id`,
		tenantID,
		queueName,
		).Scan( &queueID )

	if err != nil{
		t.Fatalf("insert queue: %v", err)
	}

	j := job.Job{}
	j.TenantId 		=		tenantID
	j.QueueID		= 		queueID
	
	j.HTTP = job.HTTP{
		URL:     srv.URL, 
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"ok":true}`),
		Timeout: 5 * time.Second,
	}
	j.Lifecycle = job.Lifecycle{
		State:       job.StatePending,
		MaxAttempts: 3,
	}

	a, err:= s.CreateJob(ctx, j)

	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if a.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	_, err = s.MarkRunnable(ctx, a.ID)

	if err != nil {
		t.Fatalf("MarkRunnable a: %v", err) 
	}


	j1, ok1, err := s.ClaimJob(ctx, "worker-1", queueID)
	if err != nil || !ok1 {
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	got1, err := s.GetJob(ctx, j1.ID)
	if err != nil {
		t.Fatalf("GetJob worker-1: %v", err)
	}

	r1 := execute.ExecuteHTTP(ctx, got1.HTTP, got1.ID, got1.Lifecycle.AttemptCount)
	if r1.Err != nil || r1.StatusCode != 200 {
		t.Fatalf("first execute: status=%d err=%v", r1.StatusCode, r1.Err)
	}
	// crash: no CompleteJob / FailJob

	_, err = pool.Exec(ctx, `
		UPDATE jobs SET locked_at = now() - interval '2 minutes' WHERE id = $1
	`, j1.ID)
	if err != nil {
		t.Fatalf("age lock: %v", err)
	}

	n, err := s.ReclaimStaleJobs(ctx, time.Minute)
	if err != nil || n < 1 {
		t.Fatalf("reclaim: n=%d err=%v", n, err)
	}

	j2, ok2, err := s.ClaimJob(ctx, "worker-2", queueID)
	if err != nil || !ok2 {
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}

	got2, err := s.GetJob(ctx, j2.ID)
	if err != nil {
		t.Fatalf("GetJob worker-2: %v", err)
	}

	r2 := execute.ExecuteHTTP(ctx, got2.HTTP, got2.ID, got2.Lifecycle.AttemptCount)
	if r2.Err != nil || r2.StatusCode != 200 {
		t.Fatalf("second execute: status=%d err=%v", r2.StatusCode, r2.Err)
	}

	if err := s.CompleteJob(ctx, j2.ID, "worker-2", r2.StatusCode, r2.Snippet); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	wantKey := fmt.Sprintf("chronos-%d-%d", a.ID, 0)
	mu.Lock()
	if len(hits) != 2 {
		mu.Unlock()
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0].IdempotencyKey != wantKey || hits[1].IdempotencyKey != wantKey {
		mu.Unlock()
		t.Fatalf("keys = %q, %q want both %q", hits[0].IdempotencyKey, hits[1].IdempotencyKey, wantKey)
	}
	mu.Unlock()
	attempts, err := s.ListJobAttempts(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListJobAttempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(attempts))
	}
}