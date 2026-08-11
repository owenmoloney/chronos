package store_test

import (
	"testing"
	"context"
	"fmt"
	"time"
	"os"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/owenmoloney/chronos/internal/job"
)

func TestJobCreateGetList(t *testing.T){
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
	
	j.HTTP				= 		job.HTTP{
		URL:  	  		"https://example.com/hook",
		Method:	 		"POST",
		Headers: 		map[string]string{"Content-Type": "application/json"},
		Body:	 		[]byte(`{"ok":true}`),
		Timeout:  		5 * time.Second,
	}

	j.Lifecycle			=		job.Lifecycle{
		State:	 job.StatePending,
	}

	created, err := s.CreateJob(ctx, j)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if created.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	got, err:= s.GetJob(ctx, created.ID)
	if(err != nil){
		t.Fatalf("GetJob: %v", err)
	}
//9 Asserts

	if got.ID != created.ID {
		t.Errorf("ID: got %v want %v", got.ID,created.ID ) 
	}

	if got.TenantId != tenantID {
		t.Errorf("TenantID: got  %v want %v", got.TenantId, tenantID) 
	}

	if got.QueueID != queueID  {
		t.Errorf("QueuedID: got %v want %v", got.QueueID,  queueID) 
	}

	if got.HTTP.URL != "https://example.com/hook"{
		t.Errorf("URL: got %q want %q", got.HTTP.URL, "https://example.com/hook")
	}

	if got.HTTP.Method !=  "POST" {
		t.Errorf("Method: got %q want %q", got.HTTP.Method, "POST")
	}

	if got.HTTP.Headers["Content-Type"] != "application/json"  {
		t.Errorf("Content-Type: got %q want %q", got.HTTP.Headers["Content-Type"], "application/json")
	}

	if string(got.HTTP.Body) != `{"ok":true}` {
		t.Errorf("Body: got %q want %q", string(got.HTTP.Body), `{"ok":true}`)
	}

	if got.HTTP.Timeout != 5 * time.Second {
		t.Errorf("Timeout: got %v want %v", got.HTTP.Timeout, 5*time.Second)
	}

	if got.Lifecycle.State != job.StatePending  {
		t.Errorf("Lifecycle State: got %v want %v", got.Lifecycle.State, job.StatePending ) 
	}

	list, err := s.ListJobsByQueue(ctx, queueID)
	if err != nil{
		t.Fatalf("ListJobsByQueue: %v", err)
	}

	found := false
	for _, item := range list {
		if item.ID == created.ID{
			found = true
			break
		}
	}

	if found == false{
		t.Fatalf("created job %d not in ListJobsByQueue result (len=%d)", created.ID, len(list))
	}
}

func TestClaimJobSkipLocked(t *testing.T){
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
	
	j.HTTP				= 		job.HTTP{
		URL:  	  		"https://example.com/hook",
		Method:	 		"POST",
		Headers: 		map[string]string{"Content-Type": "application/json"},
		Body:	 		[]byte(`{"ok":true}`),
		Timeout:  		5 * time.Second,
	}

	j.Lifecycle			=		job.Lifecycle{
		State:	 job.StatePending,
	}

	a, err:= s.CreateJob(ctx, j)

	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}


	b, err:= s.CreateJob(ctx, j)

	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if a.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	if b.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	_, err = s.MarkRunnable(ctx, a.ID)

	if err != nil {
		t.Fatalf("MarkRunnable a: %v", err) 
	}

 	_, err = s.MarkRunnable(ctx, b.ID)

	if err != nil { 
		t.Fatalf("MarkRunnable b: %v", err)
	}


	j1, ok1, err := s.ClaimJob(ctx, "worker-1", queueID)

	if err != nil || !ok1{
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	j2, ok2, err := s.ClaimJob(ctx, "worker-2", queueID)
	
	if err != nil || !ok2{
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}

	if j1.ID == j2.ID{
		t.Errorf("Both workers got job %d", j1.ID)
	}

	if j1.Lifecycle.State != job.StateRunning {
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	} 

	if j2.Lifecycle.State != job.StateRunning {
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}   	
	
	if j1.Claim.LockedBy != "worker-1" {
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}
	if j2.Claim.LockedBy != "worker-2" {
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}

	
}

func TestCompleteAndFailJob(t *testing.T){
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
	
	j.HTTP				= 		job.HTTP{
		URL:  	  		"https://example.com/hook",
		Method:	 		"POST",
		Headers: 		map[string]string{"Content-Type": "application/json"},
		Body:	 		[]byte(`{"ok":true}`),
		Timeout:  		5 * time.Second,
	}

	j.Lifecycle			=		job.Lifecycle{
		State:	 job.StatePending,
	}

	a, err:= s.CreateJob(ctx, j)

	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}


	b, err:= s.CreateJob(ctx, j)

	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	if a.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	if b.ID == 0{
		t.Fatal("CreateJob returned id 0")
	}

	_, err = s.MarkRunnable(ctx, a.ID)

	if err != nil {
		t.Fatalf("MarkRunnable a: %v", err) 
	}

 	_, err = s.MarkRunnable(ctx, b.ID)

	if err != nil { 
		t.Fatalf("MarkRunnable b: %v", err)
	}


	j1, ok1, err := s.ClaimJob(ctx, "worker-1", queueID)

	if err != nil || !ok1{
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	j2, ok2, err := s.ClaimJob(ctx, "worker-2", queueID)
	
	if err != nil || !ok2{
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}

	if j1.ID == j2.ID{
		t.Errorf("Both workers got job %d", j1.ID)
	}

	if j1.Lifecycle.State != job.StateRunning {
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	} 

	if j2.Lifecycle.State != job.StateRunning {
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}   	
	
	if j1.Claim.LockedBy != "worker-1" {
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}
	if j2.Claim.LockedBy != "worker-2" {
		t.Fatalf("claim worker-2: ok=%v err=%v", ok2, err)
	}
	err = s.CompleteJob(ctx, j1.ID, "worker-1", 200, "ok")

	if err != nil{
		t.Fatalf("CompleteJob: %v", err)
	}

	got, err := s.GetJob(ctx, j1.ID)

	if err != nil {
		t.Fatalf("GetJob after complete: %v", err)
	}

	if got.Lifecycle.State != job.StateSucceded{
		t.Fatalf("LockedBy = %q, want empty", got.Lifecycle.State)
	}

	if got.Claim.LockedBy != "" {
		t.Fatalf("LockedBy = %q, want empty", got.Claim.LockedBy)
	}

	if got.Lifecycle.AttemptCount != 1{
		t.Fatalf("AttemptCount = %d, want 1", got.Lifecycle.AttemptCount)
	}

	var attemptNumber int
	var success, httpStatus, snippet string

	err = pool.QueryRow(ctx, `
		SELECT attempt_number, success, http_status, response_snippet
		FROM job_attempts
		WHERE job_id = $1
	`, j1.ID).Scan(&attemptNumber, &success, &httpStatus, &snippet)

	if err != nil{
		t.Fatalf("job_attemps: %v", err)
	}

	if attemptNumber != 1 || success != "true" || httpStatus != "200" || snippet != "ok"{
		t.Fatalf("attempt = #%d success=%q status=%q snippet=%q",
			attemptNumber, success, httpStatus, snippet)
	}

	err = s.FailJob(ctx, j2.ID, "worker-2", 500, "boom", "http failed")
	if err != nil {
 		t.Fatalf("FailJob: %v", err)
	}

	got2, err := s.GetJob(ctx, j2.ID)

	if err != nil {
		t.Fatalf("GetJob after fail: %v", err)
	}

	var attemptNumber2 int
	var success2, httpStatus2, snippet2, errMsg string
	err = pool.QueryRow(ctx, `
 		SELECT attempt_number, success, http_status, response_snippet, error_message
    	FROM job_attempts
    	WHERE job_id = $1
	`, j2.ID).Scan(&attemptNumber2, &success2, &httpStatus2, &snippet2, &errMsg)

	if err != nil {
		t.Fatalf("GetJob after fail: %v", err)
	}
	if got2.Lifecycle.State != job.StateRunnable {
		t.Fatalf("state = %q, want failed_retrying", got2.Lifecycle.State)
	}
	if got2.Claim.LockedBy != "" {
		t.Fatalf("LockedBy = %q, want empty", got2.Claim.LockedBy)
	}
	if got2.Lifecycle.AttemptCount != 1 {
		t.Fatalf("AttemptCount = %d, want 1", got2.Lifecycle.AttemptCount)
	}

	if err != nil {
		t.Fatalf("job_attempts fail: %v", err)
	}
	if attemptNumber2 != 1 || success2 != "false" || httpStatus2 != "500" ||
		snippet2 != "boom" || errMsg != "http failed" {
		t.Fatalf("fail attempt = #%d success=%q status=%q snippet=%q err=%q",
			attemptNumber2, success2, httpStatus2, snippet2, errMsg)
	}
}

func TestFailJobDeadLetters(t *testing.T){
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
		URL:     "https://example.com/hook",
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"ok":true}`),
		Timeout: 5 * time.Second,
	}
	j.Lifecycle = job.Lifecycle{
		State:       job.StatePending,
		MaxAttempts: 1, // must be here
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

	if err != nil || !ok1{
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	err = s.FailJob(ctx, j1.ID, "worker-1", 500, "boom", "http failed")

	if err != nil{
		t.Fatalf("FailJob: %v", err)
	}

	got, err := s.GetJob(ctx, j1.ID)

	if err != nil {
		t.Fatalf("GetJob after complete: %v", err)
	}

	if got.Lifecycle.State != job.StateDeadLettered{
		t.Fatalf("State = %q, want dead_lettered", got.Lifecycle.State)
	}

	if got.Claim.LockedBy != ""{
		t.Fatalf("LockedBy = %q, want empty", got.Claim.LockedBy)
	}

	if got.Lifecycle.AttemptCount != 1{
		t.Fatalf("AttemptCount = %d, want 1", got.Lifecycle.AttemptCount)
	}

}

func TestReplayJob(t *testing.T){
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
		URL:     "https://example.com/hook",
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"ok":true}`),
		Timeout: 5 * time.Second,
	}
	j.Lifecycle = job.Lifecycle{
		State:       job.StatePending,
		MaxAttempts: 1, // must be here
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

	if err != nil || !ok1{
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	err = s.FailJob(ctx, j1.ID, "worker-1", 500, "boom", "http failed")

	if err != nil{
		t.Fatalf("FailJob: %v", err)
	}

	got, err := s.ReplayJob(ctx, j1.ID)

	if err != nil {
		t.Fatalf("ReplayJob after complete: %v", err)
	}

	if got.Lifecycle.State != job.StateRunnable{
		t.Fatalf("State = %q, want dead_lettered", got.Lifecycle.State)
	}

	if got.Claim.LockedBy != ""{
		t.Fatalf("LockedBy = %q, want empty", got.Claim.LockedBy)
	}

	if got.Lifecycle.AttemptCount != 0{
		t.Fatalf("AttemptCount = %d, want 0", got.Lifecycle.AttemptCount)
	}

}


func TestReclaimStaleJobs(t *testing.T){
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
		URL:     "https://example.com/hook",
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"ok":true}`),
		Timeout: 5 * time.Second,
	}
	j.Lifecycle = job.Lifecycle{
		State:       job.StatePending,
		MaxAttempts: 1, // must be here
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

	if err != nil || !ok1{
		t.Fatalf("claim worker-1: ok=%v err=%v", ok1, err)
	}

	_, err = pool.Exec(ctx, `
		UPDATE jobs SET locked_at = now() - interval '2 minutes' WHERE id = $1
		`, j1.ID)

	if err != nil{
		t.Fatalf("Exec Fails : %v", err)
	}
	n, err := s.ReclaimStaleJobs(ctx, time.Minute)

	if err != nil || n < 1 {
		t.Fatalf("claim worker-1: n=%v err=%v", n, err)
	}

	got, err := s.GetJob(ctx, j1.ID)

	if err != nil {
		t.Fatalf("GetJob after complete: %v", err)
	}

	if got.Lifecycle.State != job.StateRunnable{
		t.Fatalf("State = %q, want state_runnable", got.Lifecycle.State)
	}

	if got.Claim.LockedBy != ""{
		t.Fatalf("LockedBy = %q, want empty", got.Claim.LockedBy)
	}

	if got.Lifecycle.AttemptCount != 0{
		t.Fatalf("AttemptCount = %d, want 0", got.Lifecycle.AttemptCount)
	}

}

func TestCancelJob(t *testing.T){
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
		URL:     "https://example.com/hook",
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"ok":true}`),
		Timeout: 5 * time.Second,
	}
	j.Lifecycle = job.Lifecycle{
		State:       job.StatePending,
		MaxAttempts: 1, // must be here
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

	got, err := s.CancelJob(ctx, a.ID)

	if err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	if got.Lifecycle.State != job.StateCanceled {
		t.Fatalf("state = %q, want canceled", got.Lifecycle.State)
	}
	if got.Claim.LockedBy != "" {
		t.Fatalf("LockedBy = %q, want empty", got.Claim.LockedBy)
	}

}
