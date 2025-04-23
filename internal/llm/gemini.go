package gemini

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	defaultModelName = "gemini-2.0-flash"
)

type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewGeminiClient(ctx context.Context, apiKey string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := client.GenerativeModel(defaultModelName)

	return &GeminiClient{
		client: client,
		model:  model,
	}, nil
}

func (g *GeminiClient) Generate(ctx context.Context, prompt string, data ...string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt cannot be empty")
	}

	fullPrompt := []genai.Part{
		genai.Text(prompt),
	}

	for _, d := range data {
		fullPrompt = append(fullPrompt, genai.Text(d))
	}

	resp, err := g.model.GenerateContent(ctx, fullPrompt...)
	if err != nil {
		return "", fmt.Errorf("generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no content in response")
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		sb.WriteString(fmt.Sprintf("%v", part))
	}

	return sb.String(), nil
}

func (g *GeminiClient) Close() error {
	return g.client.Close()
}

func (g *GeminiClient) IsSpam(ctx context.Context, message string) (bool, error) {
	if strings.TrimSpace(message) == "" {
		return false, fmt.Errorf("сообщение не может быть пустым")
	}

	prompt := `Ты эксперт по обнаружению спама в Telegram. Проанализируй следующее сообщение пользователя и определи, является ли оно спамом.

Спам обычно содержит:
- Нежелательные ссылки на внешние ресурсы
- Предложения заработка/инвестиций
- Рекламу
- Массовую рассылку
- Попытки фишинга
- Любые вредоносные или мошеннические намерения

Спамом не являются:
- Сообщение с четырехзначным кодом (например "ngzi")
- Обычные сообщения пользователей

Ответь только "YES", если сообщение является спамом, или "NO", если это нормальное сообщение.

Сообщение: ` + message

	resp, err := g.Generate(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке спама: %w", err)
	}

	resp = strings.TrimSpace(resp)
	resp = strings.ToUpper(resp)

	return strings.Contains(resp, "YES"), nil
}
