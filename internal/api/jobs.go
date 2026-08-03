package api
import(
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"github.com/owenmoloney/chronos/internal/job"
	"github.com/owenmoloney/chronos/internal/store"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
)

type Handler struct{
	Store  		*store.Store
	JWTSecret 	string

}

func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request){
	
	if r.Method != "POST"{
		http.Error(w, "method not allowed", 405)
		return
	}



	var req CreateJobRequest

	err:= json.NewDecoder(r.Body).Decode(&req)
	
	if err!= nil{
		http.Error(w, "Invalid JSON body", 400)
		return
	}

	tenantID, err := h.tenantIDFromRequest(r)

	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}

	key:= r.Header.Get("Idempotency-Key")

	bodyBytes, err := json.Marshal(req)

	if err!=nil{
		http.Error(w, "Failed ot hash request", 400)
		return 
	}

	sum := sha256.Sum256(bodyBytes)
	hash:= hex.EncodeToString(sum[:])

	var j job.Job

	j.TenantId = tenantID
	j.QueueID  = req.QueueId         

	j.HTTP.URL = req.Url
	j.HTTP.Method = req.Method
	j.HTTP.Headers = req.Headers
	j.HTTP.Body = []byte(req.Body)
	j.HTTP.Timeout = time.Duration(req.TimeoutMs) * time.Millisecond

	j.Lifecycle.State = job.StatePending
	j.Lifecycle.MaxAttempts = int(req.MaxAttempts)

	if !req.RunAt.IsZero(){
		j.Lifecycle.RunAt = req.RunAt
	}

	if key == ""{
		created, err := h.Store.CreateJob(r.Context(), j)
		if err != nil{
			http.Error(w, "Create Job failed", 500)
			return
		}

		var resp JobResponse

		resp.Id					= created.ID 						
		resp.TenantId			= created.TenantId			
		resp.QueueId			= created.QueueID
		resp.Url				= created.HTTP.URL
		resp.Method 			= created.HTTP.Method						
		resp.Headers			= created.HTTP.Headers		
		resp.Body				= json.RawMessage(created.HTTP.Body)
		resp.TimeoutMs			= created.HTTP.Timeout.Milliseconds()
		resp.State				= string(created.Lifecycle.State)
		resp.RunAt          	= created.Lifecycle.RunAt		
		resp.AttemptCount   	= int64(created.Lifecycle.AttemptCount)		
		resp.MaxAttempts    	= int64(created.Lifecycle.MaxAttempts)		
		resp.NextRunAt     		= created.Lifecycle.NextRunAt	
		resp.LockedBy       	= created.Claim.LockedBy		
		if created.Claim.LockedAt  != nil{
			resp.LockedAt = *created.Claim.LockedAt
		}     		
		resp.CancelRequested	= created.Cancel.CancelRequested			
		resp.IdempotencyKey     = created.Idempotency.IdempotencyKey
		resp.CreatedAt         	= created.Timestamps.CreatedAt	
		resp.UpdatedAt          = created.Timestamps.UpdatedAt
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		err = json.NewEncoder(w).Encode(resp)
		return 
	}
	rec, err := h.Store.GetIdempotency(r.Context(), tenantID, key)

	if err!=nil && errors.Is(err, pgx.ErrNoRows){
		j.Idempotency.IdempotencyKey = key

		created, err := h.Store.CreateJobWithIdempotency(r.Context(), j, key, hash)

		if err != nil{
			http.Error(w, "Create Job failed", 500)
			return
		}
		var resp JobResponse

		resp.Id					= created.ID 						
		resp.TenantId			= created.TenantId			
		resp.QueueId			= created.QueueID
		resp.Url				= created.HTTP.URL
		resp.Method 			= created.HTTP.Method						
		resp.Headers			= created.HTTP.Headers		
		resp.Body				= json.RawMessage(created.HTTP.Body)
		resp.TimeoutMs			= created.HTTP.Timeout.Milliseconds()
		resp.State				= string(created.Lifecycle.State)
		resp.RunAt          	= created.Lifecycle.RunAt		
		resp.AttemptCount   	= int64(created.Lifecycle.AttemptCount)		
		resp.MaxAttempts    	= int64(created.Lifecycle.MaxAttempts)		
		resp.NextRunAt     		= created.Lifecycle.NextRunAt	
		resp.LockedBy       	= created.Claim.LockedBy		
		if created.Claim.LockedAt  != nil{
			resp.LockedAt = *created.Claim.LockedAt
		}     		
		resp.CancelRequested	= created.Cancel.CancelRequested			
		resp.IdempotencyKey     = created.Idempotency.IdempotencyKey
		resp.CreatedAt         	= created.Timestamps.CreatedAt	
		resp.UpdatedAt          = created.Timestamps.UpdatedAt
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		err = json.NewEncoder(w).Encode(resp)
		return 
	}


	if err != nil{
		http.Error(w, "Not Found", 500)
		return
	}

	if rec.RequestHash != hash{
		http.Error(w, "Idempotency key conflict" ,409)
		return
	}

	got, err := h.Store.GetJob(r.Context(), rec.JobId)

	if err != nil {
		http.Error(w, "get job failed", 500)
		return
	}

	var resp JobResponse

	resp.Id					= got.ID 						
	resp.TenantId			= got.TenantId			
	resp.QueueId			= got.QueueID
	resp.Url				= got.HTTP.URL
	resp.Method 			= got.HTTP.Method						
	resp.Headers			= got.HTTP.Headers		
	resp.Body				= json.RawMessage(got.HTTP.Body)
	resp.TimeoutMs			= got.HTTP.Timeout.Milliseconds()
	resp.State				= string(got.Lifecycle.State)
	resp.RunAt          	= got.Lifecycle.RunAt		
	resp.AttemptCount   	= int64(got.Lifecycle.AttemptCount)		
	resp.MaxAttempts    	= int64(got.Lifecycle.MaxAttempts)		
	resp.NextRunAt     		= got.Lifecycle.NextRunAt	
	resp.LockedBy       	= got.Claim.LockedBy		
	if got.Claim.LockedAt  != nil{
		resp.LockedAt = *got.Claim.LockedAt
	}     		
	resp.CancelRequested	= got.Cancel.CancelRequested			
	resp.IdempotencyKey     = got.Idempotency.IdempotencyKey
	resp.CreatedAt         	= got.Timestamps.CreatedAt	
	resp.UpdatedAt          = got.Timestamps.UpdatedAt
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	err = json.NewEncoder(w).Encode(resp)
	return 

}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request){
	
	if r.Method != "GET"{
		http.Error(w, "method not allowed", 405)
		return
	}

	idStr := r.PathValue("id")

	if idStr == ""{
		http.Error(w, "Missing id", 400)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)

	if err != nil{
		http.Error(w, "invalid X-Tenant-ID", 400)
		return 
	}

	tenantID, err := h.tenantIDFromRequest(r)
	
	if err != nil {
	  http.Error(w, "unauthorized", 401)
	  return
	}

	got, err := h.Store.GetJob(r.Context(), id)

	if err != nil{
		http.Error(w, "job not found", 404)
		return
	}

	if got.TenantId != tenantID {
		http.Error(w, "Job not found", 404)
		return
	}

	var resp JobResponse

	resp.Id					= got.ID 						
	resp.TenantId			= got.TenantId			
	resp.QueueId			= got.QueueID
	resp.Url				= got.HTTP.URL
	resp.Method 			= got.HTTP.Method						
	resp.Headers			= got.HTTP.Headers		
	resp.Body				= json.RawMessage(got.HTTP.Body)
	resp.TimeoutMs			= got.HTTP.Timeout.Milliseconds()
	resp.State				= string(got.Lifecycle.State)
	resp.RunAt          	= got.Lifecycle.RunAt		
    resp.AttemptCount   	= int64(got.Lifecycle.AttemptCount)		
    resp.MaxAttempts    	= int64(got.Lifecycle.MaxAttempts)		
    resp.NextRunAt     		= got.Lifecycle.NextRunAt	
    resp.LockedBy       	= got.Claim.LockedBy		
    if got.Claim.LockedAt  != nil{
		resp.LockedAt = *got.Claim.LockedAt
	}     		
    resp.CancelRequested	= got.Cancel.CancelRequested			
    resp.IdempotencyKey     = got.Idempotency.IdempotencyKey
    resp.CreatedAt         	= got.Timestamps.CreatedAt	
    resp.UpdatedAt          = got.Timestamps.UpdatedAt
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	err = json.NewEncoder(w).Encode(resp)

	if err != nil{
		return
	}
}


