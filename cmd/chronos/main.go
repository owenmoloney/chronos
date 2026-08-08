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
	"github.com/owenmoloney/chronos/internal/worker"
	"time"
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
	http.HandleFunc("POST /jobs", h.CreateJob)
	http.HandleFunc("GET /jobs/{id}", h.GetJob)
	http.HandleFunc("POST /auth/token", h.IssueToken)
	fmt.Println(cfg.HTTPAddr)

	go func(){
		for{
			didWork, err:= worker.RunOnce(ctx, s, cfg.WorkerID, cfg.QueueID)
			if err != nil{
				logger.Info("Worker error")
			}

			if !didWork{
				time.Sleep(time.Second)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(10*time.Second)
		defer ticker.Stop()
		for range ticker.C{
			n, err:= s.ReclaimStaleJobs(ctx, cfg.LeaseTimeout)
			if err != nil{
				logger.Info("Reclaim error")
				continue
			}
			if n > 0 {
				logger.Info("Reclaimed Stale jobs")
			}
		}
	}()

	err = http.ListenAndServe(cfg.HTTPAddr, nil)

	if err != nil{
		log.Fatal(err)
	}
}