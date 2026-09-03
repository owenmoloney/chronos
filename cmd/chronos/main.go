package main

import (
	"fmt"
	"net/http"
	"log"
	"context"
	"github.com/owenmoloney/chronos/internal/config"
	"github.com/owenmoloney/chronos/internal/observe"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/owenmoloney/chronos/internal/api"
	"github.com/owenmoloney/chronos/internal/leader"
	"github.com/owenmoloney/chronos/internal/scheduler"

)

func health(w http.ResponseWriter, r *http.Request){
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"ok"}`)
}

func main(){

	fmt.Println("Starting")

	cfg := config.Load()

	logger := observe.NewLogger(cfg)

	ctx := context.Background()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	
	if err != nil{
		log.Fatal(err)
	}
	defer pool.Close()

	s := store.New(pool)
	
	h := &api.Handler{Store: s, JWTSecret: cfg.JWTSecret}
	
	logger.Info("chronos starting")

	http.HandleFunc("/health", health)
	http.Handle("/metrics", observe.MetricsHandler())
	http.HandleFunc("POST /jobs", h.CreateJob)
	http.HandleFunc("GET /jobs", h.ListJobs)
	http.HandleFunc("GET /jobs/{id}", h.GetJob)
	http.HandleFunc("POST /auth/token", h.IssueToken)
	http.HandleFunc("POST /jobs/{id}/replay", h.ReplayJob)
	http.HandleFunc("POST /jobs/{id}/cancel", h.CancelJob)
	http.HandleFunc("GET /jobs/{id}/attempts", h.ListJobAttempts)

	fmt.Println(cfg.HTTPAddr)

	elector, err := leader.New(cfg.RedisURL, cfg.WorkerID)
	if err != nil{
		log.Fatal(err)
	}

	go elector.Run(ctx)

	sched :=  scheduler.New(s, elector)
	go sched.Run(ctx)

	err = http.ListenAndServe(cfg.HTTPAddr, nil)

	if err != nil{
		log.Fatal(err)
	}
}