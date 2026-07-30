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
