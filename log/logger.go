package log

import (
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	logger  *zap.Logger
	initErr error
	once    sync.Once
)

// Init initializes the app logger once based on APP_ENV.
// APP_ENV=development enables a human-readable logger.
func Init() error {
	once.Do(func() {
		if strings.EqualFold(os.Getenv("APP_ENV"), "development") {
			logger, initErr = zap.NewDevelopment()
			return
		}

		logger, initErr = zap.NewProduction()
	})

	return initErr
}

// L returns the initialized zap logger.
func L() *zap.Logger {
	if logger == nil {
		panic("logger is not initialized: call log.Init() first")
	}

	return logger
}

// Sync flushes buffered logs.
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	L().Fatal(msg, fields...)
}
