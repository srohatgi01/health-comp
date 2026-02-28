package config

import (
	"os"
	"strings"

	"go.uber.org/zap"
)

// NewLogger creates a zap logger based on APP_ENV.
// APP_ENV=development enables a human-readable logger.
func NewLogger() (*zap.Logger, error) {
	if strings.EqualFold(os.Getenv("APP_ENV"), "development") {
		return zap.NewDevelopment()
	}

	return zap.NewProduction()
}
