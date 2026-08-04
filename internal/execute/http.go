package execute

import(
	"context"
	"github.com/owenmoloney/chronos/internal/job"
	"net/http"
	"bytes"
	"io"
	"time"
)

type Result struct{
	StatusCode 	int
	Snippet		string
	Err 		error
}

func ExecuteHTTP(ctx context.Context, h job.HTTP) Result{
	err := SafeURL(h.URL)
	
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

	resp, err := client.Do(req)

	if err !=nil{
		return Result{Err: err}
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	return Result{StatusCode: resp.StatusCode, Snippet: string(body)}
}
