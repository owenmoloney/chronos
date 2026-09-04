package api 

import(
	
	"net/http"
)


func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /jobs", h.CreateJob)
	mux.HandleFunc("GET /jobs", h.ListJobs)
	mux.HandleFunc("GET /jobs/{id}", h.GetJob)
	mux.HandleFunc("POST /auth/token", h.IssueToken)
	mux.HandleFunc("POST /jobs/{id}/replay", h.ReplayJob)
	mux.HandleFunc("POST /jobs/{id}/cancel", h.CancelJob)
	mux.HandleFunc("GET /jobs/{id}/attempts", h.ListJobAttempts)
	mux.HandleFunc("GET /cron", h.ListCron)
	mux.HandleFunc("GET /cron/{id}", h.GetCron)
	mux.HandleFunc("POST /cron", h.CreateCron)
	mux.HandleFunc("POST /cron/{id}/enable", h.EnableCron)
	mux.HandleFunc("POST /cron/{id}/disable", h.DisableCron)
}