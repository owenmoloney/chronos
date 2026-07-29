package main

import (
	"fmt"
	"net/http"
	"log"
	"github.com/owenmoloney/chronos/internal/config"
	"github.com/owenmoloney/chronos/internal/observe"
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

	logger.Info("chronos starting")

	http.HandleFunc("/health", health)

	fmt.Println(cfg.HTTPAddr)

	err := http.ListenAndServe(cfg.HTTPAddr, nil)

	if err != nil{
		log.Fatal(err)
	}
}