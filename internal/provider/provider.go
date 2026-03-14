package provider

import (
	"context"
	"github.com/alexkovalov/gemini-chat/internal/models"
)

type StreamRequest struct {
	APIKey       string
	ModelID      string
	SystemPrompt string
	Messages     []models.Message
}

type ModelInfo struct {
	ID          string
	DisplayName string
}

type Provider interface {
	ID() string
	Name() string
	Stream(ctx context.Context, req StreamRequest) (<-chan string, <-chan error)
	ListModels(ctx context.Context, apiKey string) ([]ModelInfo, error)
	ValidateKey(ctx context.Context, apiKey string) error
}
