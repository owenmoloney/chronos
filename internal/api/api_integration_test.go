package api_test


import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/owenmoloney/chronos/internal/api"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type apiFixture struct{
	t		*testing.T
	server *httptest.Server
	store  *store.Store
	pool   *pgxpool.Pool
	tenantID int64
	queueID  int64
	token    string 
}

func newAPIFixture(t *testing.T) *apiFixture{
	t.Helper()
	ctx:= context.Background()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable"
	}

	pool, err := store.NewPool(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	t.Cleanup(func() {pool.Close()})

	s := store.New(pool)
	suffix := time.Now().UnixNano()

	var tenantID, queueID int64
	err = pool.QueryRow(ctx,
		`INSERT INTO tenants (name) VALUES ($1) RETURNING id`,
		fmt.Sprintf("api-test-tenant-%d", suffix),
	).Scan(&tenantID)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO queues (tenant_id, name) VALUES ($1, $2) RETURNING id`,
		tenantID,
		fmt.Sprintf("api-test-queue-%d", suffix),
	).Scan(&queueID)
	if err != nil {
		t.Fatalf("insert queue: %v", err)
	}

	h := &api.Handler{Store: s, JWTSecret: "test-secret"}
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	f := &apiFixture{t: t, server: srv, store: s, pool: pool, tenantID: tenantID, queueID: queueID}

	body, _ := json.Marshal(map[string]int64{"tenant_id": f.tenantID})
	res, err := http.Post(f.server.URL+"/auth/token", "application/json", bytes.NewReader(body))


	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer res.Body.Close()
	
	if res.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d, want 200", res.StatusCode)
	}
	
	var tok api.TokenResponse 
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	f.token = tok.Token
	return f
}

func TestCreateJobUnauthorized(t *testing.T) {
	f := newAPIFixture(t)

	body := fmt.Sprintf(`{
		"queue_id": %d,
		"url": "https://example.com",
		"method": "GET",
		"max_attempts": 3,
		"timeout_ms": 5000
	}`, f.queueID)

	req, err := http.NewRequest(http.MethodPost, f.server.URL+"/jobs", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// no Authorization

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}

func (f *apiFixture) do(method, path string, body []byte, auth bool) *http.Response {
	f.t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, f.server.URL+path, rdr)
	if err != nil {
		f.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

func TestCreateGetListJob(t *testing.T) {
	f := newAPIFixture(t)

	createBody := fmt.Sprintf(`{
		"queue_id": %d,
		"url": "https://example.com/hook",
		"method": "POST",
		"max_attempts": 3,
		"timeout_ms": 5000
	}`, f.queueID)

	res := f.do(http.MethodPost, "/jobs", []byte(createBody), true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated { // 201
		t.Fatalf("create status = %d, want 201", res.StatusCode)
	}

	var created api.JobResponse
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Id == 0 || created.QueueId != f.queueID {
		t.Fatalf("created = %+v", created)
	}

	// GET /jobs/{id}
	res = f.do(http.MethodGet, fmt.Sprintf("/jobs/%d", created.Id), nil, true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", res.StatusCode)
	}
	var got api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&got)
	if got.Id != created.Id {
		t.Fatalf("get id = %d, want %d", got.Id, created.Id)
	}

	// GET /jobs?queue_id=&limit=10
	res = f.do(http.MethodGet, fmt.Sprintf("/jobs?queue_id=%d&limit=10", f.queueID), nil, true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", res.StatusCode)
	}
	var list []api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&list)
	found := false
	for _, j := range list {
		if j.Id == created.Id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list missing job %d", created.Id)
	}
}

func (f *apiFixture) doWithHeaders(method, path string, body []byte, headers map[string]string) *http.Response {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.token)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return res
}

func TestCreateJobIdempotency(t *testing.T) {
	f := newAPIFixture(t)
	key := fmt.Sprintf("idem-%d", time.Now().UnixNano())

	body := []byte(fmt.Sprintf(`{
		"queue_id": %d,
		"url": "https://example.com/a",
		"method": "POST",
		"max_attempts": 3,
		"timeout_ms": 5000
	}`, f.queueID))

	hdrs := map[string]string{"Idempotency-Key": key}

	res := f.doWithHeaders(http.MethodPost, "/jobs", body, hdrs)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("first create: %d", res.StatusCode)
	}
	var first api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&first)

	// replay — identical body
	res = f.doWithHeaders(http.MethodPost, "/jobs", body, hdrs)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("replay: %d, want 200", res.StatusCode)
	}
	var second api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&second)
	if second.Id != first.Id {
		t.Fatalf("replay id = %d, want %d", second.Id, first.Id)
	}

	// conflict — same key, different URL
	conflictBody := []byte(fmt.Sprintf(`{
		"queue_id": %d,
		"url": "https://example.com/b",
		"method": "POST",
		"max_attempts": 3,
		"timeout_ms": 5000
	}`, f.queueID))

	res = f.doWithHeaders(http.MethodPost, "/jobs", conflictBody, hdrs)
	defer res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("conflict: %d, want 409", res.StatusCode)
	}
}

func TestCancelAndAttempts(t *testing.T) {
	f := newAPIFixture(t)

	body := []byte(fmt.Sprintf(`{
		"queue_id": %d,
		"url": "https://example.com",
		"method": "GET",
		"max_attempts": 3,
		"timeout_ms": 5000
	}`, f.queueID))

	res := f.do(http.MethodPost, "/jobs", body, true) // or doWithHeaders without idem key
	defer res.Body.Close()
	var created api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&created)

	res = f.do(http.MethodGet, fmt.Sprintf("/jobs/%d/attempts", created.Id), nil, true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("attempts status = %d", res.StatusCode)
	}
	var attempts []api.AttemptResponse
	_ = json.NewDecoder(res.Body).Decode(&attempts)
	if len(attempts) != 0 {
		t.Fatalf("attempts = %d, want 0", len(attempts))
	}

	res = f.do(http.MethodPost, fmt.Sprintf("/jobs/%d/cancel", created.Id), nil, true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d", res.StatusCode)
	}
	var canceled api.JobResponse
	_ = json.NewDecoder(res.Body).Decode(&canceled)
	if canceled.State != "canceled" {
		t.Fatalf("state = %q, want canceled", canceled.State)
	}
}

func TestCronEnableDisable(t *testing.T) {
	f := newAPIFixture(t)

	body := []byte(fmt.Sprintf(`{
		"queue_id": %d,
		"cron_expr": "*/5 * * * *",
		"timezone": "UTC",
		"url": "https://example.com/cron",
		"method": "POST",
		"timeout_ms": 5000,
		"max_attempts": 3,
		"enabled": false
	}`, f.queueID))

	res := f.do(http.MethodPost, "/cron", body, true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create cron: %d", res.StatusCode)
	}
	var cron api.CronResponse
	_ = json.NewDecoder(res.Body).Decode(&cron)

	res = f.do(http.MethodPost, fmt.Sprintf("/cron/%d/enable", cron.Id), nil, true)
	defer res.Body.Close()
	_ = json.NewDecoder(res.Body).Decode(&cron)
	if !cron.Enabled {
		t.Fatal("want enabled")
	}

	res = f.do(http.MethodPost, fmt.Sprintf("/cron/%d/disable", cron.Id), nil, true)
	defer res.Body.Close()
	_ = json.NewDecoder(res.Body).Decode(&cron)
	if cron.Enabled {
		t.Fatal("want disabled")
	}
}

func TestBadInput(t *testing.T) {
	f := newAPIFixture(t)

	res := f.do(http.MethodPost, "/jobs", []byte(`{not-json`), true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad json: %d", res.StatusCode)
	}

	res = f.do(http.MethodPost, "/cron", []byte(fmt.Sprintf(
		`{"queue_id":%d,"cron_expr":"not a cron","url":"https://example.com","method":"GET"}`,
		f.queueID)), true)
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad cron_expr: %d", res.StatusCode)
	}
}