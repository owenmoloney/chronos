package observe

import (
	"go.uber.org/zap"
	"github.com/owenmoloney/chronos/internal/config"
)

func NewLogger(cfg config.Config) *zap.Logger{
	logger, err := zap.NewProduction()
	if err != nil { 
		panic(err)
	}


	return logger
}