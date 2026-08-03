package config
import "os"

type Config struct{
	HTTPAddr string
	DatabaseURL string
	RedisURL string
	Role string
	JWTSecret string
}

func Load() Config {
	cfg := Config {
		HTTPAddr : ":8080",
		DatabaseURL: "postgres://chronos:chronos@localhost:5432/chronos?sslmode=disable",
		JWTSecret: "dev-secret-change-me",
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

	return cfg
}