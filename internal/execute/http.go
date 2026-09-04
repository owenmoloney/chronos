package execute

import(
	"context"
	"github.com/owenmoloney/chronos/internal/job"
	"net/http"
	"bytes"
	"io"
	"fmt"
	"time"
)

type Result struct{
	StatusCode 	int
	Snippet		string
	Err 		error
}

var validateURL = SafeURL


func ExecuteHTTP(ctx context.Context, h job.HTTP, jobID int64, attemptCount int) Result{
	err := validateURL(h.URL)
	
	if err !=nil{
		return Result{Err: err}
	}

	timeout := h.Timeout

	if timeout == 0{
		timeout = 30 * time.Second
	}

	client := &http.Client{Timeout: timeout}

	method:= h.Method

	if method == ""{
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, h.URL, bytes.NewReader(h.Body))

	if err !=nil{
		return Result{Err: err}
	}

	for k, v := range h.Headers{
		req.Header.Set(k, v)
	}

	if req.Header.Get("Idempotency-Key") ==""{
		req.Header.Set("Idempotency-Key", fmt.Sprintf("chronos-%d-%d", jobID, attemptCount))
	}

	resp, err := client.Do(req)

	if err !=nil{
		return Result{Err: err}
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return Result{StatusCode: resp.StatusCode, Snippet: string(body)}
}


func SetURLValidatorForTest(fn func(string) error) (restore func()){
	prev := validateURL
	validateURL = fn
	return func() { validateURL = prev }
}