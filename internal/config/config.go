package config
import (
	"os"
	"fmt"
	"strconv"
	"time"
)
type Config struct{
	HTTPAddr string
	DatabaseURL string
	RedisURL string
	Role string
	JWTSecret string
	WorkerID string
	QueueID int64
	LeaseTimeout time.Duration
}

func Load() Config {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "chronos"
	}

	pid := os.Getpid()
	
	cfg := Config {
		HTTPAddr : ":8080",
		DatabaseURL: "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable",
		JWTSecret: "dev-secret-change-me",
		WorkerID: fmt.Sprintf("%s-%d", host, pid),
		QueueID: 1,
		LeaseTimeout: 60 * time.Second,
		RedisURL: "redis://localhost:6379/0",
	}

	if envAddr := os.Getenv("JWT_SECRET"); envAddr != "" {
		cfg.JWTSecret = envAddr
	}

	if envAddr := os.Getenv("HTTP_ADDR"); envAddr != "" {
		cfg.HTTPAddr = envAddr
	}

	if envAddr := os.Getenv("DATABASE_URL"); envAddr != "" {
		cfg.DatabaseURL = envAddr
	}

	if envAddr := os.Getenv("WORKER_ID"); envAddr != "" {
		cfg.WorkerID = envAddr
	}

	if envAddr := os.Getenv("REDIS_URL"); envAddr != "" {
		cfg.RedisURL = envAddr
	}

	if v := os.Getenv("QUEUE_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			cfg.QueueID = id
		}
	}

	if v := os.Getenv("LEASE_TIMEOUT"); v != "" {
		sec, err := strconv.ParseInt(v, 10, 64)
    	if err == nil && sec > 0 {
        	cfg.LeaseTimeout = time.Duration(sec) * time.Second
    	}
	}

	return cfg
}