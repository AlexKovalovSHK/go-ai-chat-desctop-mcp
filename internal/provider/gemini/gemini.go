package gemini

import (
	"context"
	"fmt"

	"strings"

	"github.com/alexkovalov/gemini-chat/internal/provider"
	"google.golang.org/genai"
)

type GeminiProvider struct{}

func New() *GeminiProvider {
	return &GeminiProvider{}
}

func (p *GeminiProvider) ID() string   { return "gemini" }
func (p *GeminiProvider) Name() string { return "Google Gemini" }

func (p *GeminiProvider) Stream(ctx context.Context, req provider.StreamRequest) (<-chan string, <-chan error) {
	textCh := make(chan string)
	errCh := make(chan error, 1)

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  req.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		errCh <- p.mapError(err)
		close(textCh)
		close(errCh)
		return textCh, errCh
	}

	go func() {
		defer close(textCh)
		defer close(errCh)

		var contents []*genai.Content
		for _, msg := range req.Messages {
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: []*genai.Part{{Text: msg.Content}},
			})
		}

		config := &genai.GenerateContentConfig{}
		if req.SystemPrompt != "" {
			config.SystemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: req.SystemPrompt}},
			}
		}

		iter := client.Models.GenerateContentStream(ctx, req.ModelID, contents, config)
		for resp, err := range iter {
			if err != nil {
				errCh <- p.mapError(err)
				return
			}
			if text := resp.Text(); text != "" {
				textCh <- text
			}
		}
	}()

	return textCh, errCh
}

func (p *GeminiProvider) ListModels(ctx context.Context, apiKey string) ([]provider.ModelInfo, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	var models []provider.ModelInfo
	page, err := client.Models.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	for {
		for _, m := range page.Items {
			supportsGenerate := false
			for _, op := range m.SupportedActions {
				if op == "generateContent" || op == "generate_content" {
					supportsGenerate = true
					break
				}
			}
			if supportsGenerate {
				// ✅ Убираем префикс "models/" — API принимает только короткое имя
				id := strings.TrimPrefix(m.Name, "models/")
				models = append(models, provider.ModelInfo{
					ID:          id,
					DisplayName: m.DisplayName,
				})
			}
		}

		if page.NextPageToken == "" {
			break
		}
		page, err = page.Next(ctx)
		if err != nil {
			return nil, err
		}
	}

	return models, nil
}

func (p *GeminiProvider) ValidateKey(ctx context.Context, apiKey string) error {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return err
	}

	// Try to list models with PageSize 1 as a validation step
	_, err = client.Models.List(ctx, &genai.ListModelsConfig{PageSize: 1})
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			return fmt.Errorf("превышен лимит запросов (Rate Limit). Пожалуйста, подождите немного")
		}
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("невалидный API ключ или нет доступа к API")
		}
		return fmt.Errorf("ошибка проверки ключа: %v", err)
	}
	return nil
}

func (p *GeminiProvider) mapError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "429"):
		return fmt.Errorf("превышен лимит запросов (Rate Limit). Попробуйте позже")
	case strings.Contains(msg, "400") && strings.Contains(msg, "safety"):
		return fmt.Errorf("запрос заблокирован фильтром безопасности Google")
	case strings.Contains(msg, "context_window_exceeded"):
		return fmt.Errorf("превышено окно контекста модели")
	case strings.Contains(msg, "403"):
		return fmt.Errorf("доступ запрещен (проверьте настройки API ключа)")
	}
	return err
}
