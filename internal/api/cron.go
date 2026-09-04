package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"github.com/owenmoloney/chronos/internal/job"
	"time"
	"github.com/robfig/cron/v3"
)

func (h *Handler) CreateCron(w http.ResponseWriter, r *http.Request){
	
	if r.Method != "POST"{
		http.Error(w, "method not allowed", 405)
		return
	}

	
	tenantID, err := h.tenantIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}

	var req CreateCronRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", 400)
		return
	}

	if req.QueueId < 0{
		http.Error(w, "queue_id required", 400)
		return
	}

	if req.CronExpr == "" {
		http.Error(w, "cron_expr required", 400)
		return
	}
	if req.Url == "" {
		http.Error(w, "url required", 400)
		return
	}
	if _, err := cron.ParseStandard(req.CronExpr); err != nil {
		http.Error(w, "invalid cron_expr", 400)
		return
	}

	def := job.CronDefinition{
		TenantID:    tenantID,
		QueueID:     req.QueueId,
		CronExpr:    req.CronExpr,
		Timezone:    req.Timezone,  
		URL:         req.Url,
		Method:      req.Method,
		Headers:     req.Headers,
		Body:        []byte(req.Body), // ok if nil/empty RawMessage
		Timeout:     time.Duration(req.TimeoutMs) * time.Millisecond,
		MaxAttempts: int(req.MaxAttempts),
	}
	if req.Enabled != nil {
		def.Enabled = *req.Enabled
	}

	created, err := h.Store.CreateCronDefinition(r.Context(), def)
	if err != nil {
		http.Error(w, "Create cron failed", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(cronToResponse(created))


}
func cronToResponse(def job.CronDefinition) CronResponse{
	return CronResponse{
		Id:					 def.ID, 						
		TenantId:			 def.TenantID,		
		QueueId:			 def.QueueID,
		CronExpr:			 def.CronExpr,
		Timezone:			 def.Timezone,
		Url:				 def.URL,
		Method: 			 def.Method,						
		Headers:			 def.Headers,		
		Body:				 json.RawMessage(def.Body),
		TimeoutMs:			 def.Timeout.Milliseconds(),
    	MaxAttempts:    	 def.MaxAttempts,		
    	Enabled:			 def.Enabled,
		LastEnqueuedAt:		 def.LastEnqueuedAt,
	}
}

func (h *Handler) ListCron(w http.ResponseWriter, r *http.Request) {
	tenantID, err := h.tenantIDFromRequest(r)
	
	if err != nil {
	  http.Error(w, "unauthorized", 401)
	  return
	}

	cron, err := h.Store.ListCronDefinitions(r.Context(), tenantID)


	if err != nil{
		http.Error(w, "List Cron failed", 500)
		return
	}

	out := make([]CronResponse, 0, len(cron))
	for _, j := range cron {
		out = append(out, cronToResponse(j))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(out)	
}

func (h *Handler) GetCron(w http.ResponseWriter, r *http.Request){
	
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
		http.Error(w, "invalid X-Cron-ID", 400)
		return 
	}

	tenantID, err := h.tenantIDFromRequest(r)
	
	if err != nil {
	  http.Error(w, "unauthorized", 401)
	  return
	}

	got, err := h.Store.GetCronDefinition(r.Context(), tenantID, id)

	if err != nil{
		http.Error(w, "Cron not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(cronToResponse(got))
}


func (h *Handler) setCronEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Missing id", 400)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	tenantID, err := h.tenantIDFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	got, err := h.Store.SetCronEnabled(r.Context(), tenantID, id, enabled)
	if err != nil {
		http.Error(w, "Cron not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_ = json.NewEncoder(w).Encode(cronToResponse(got))
}
func (h *Handler) EnableCron(w http.ResponseWriter, r *http.Request) {
	h.setCronEnabled(w, r, true)
}

func (h *Handler) DisableCron(w http.ResponseWriter, r *http.Request) {
	h.setCronEnabled(w, r, false)
}