package gemini

import (
	"context"

	"github.com/srohatgi/health-comp/log"
	"go.uber.org/zap"
	genai "google.golang.org/genai"
)

func Init(ctx *context.Context) *genai.Client {
	client, err := genai.NewClient(*ctx, nil)
	if err != nil {
		log.Fatal("failed to initialize gemini client", zap.Error(err))
	}

	return client
}
