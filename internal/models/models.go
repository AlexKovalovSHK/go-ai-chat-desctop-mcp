package models

import "time"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	ID        int64     `json:"id"`
	ChatID    string    `json:"chat_id"`
	Role      Role      `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatSession struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	ProviderID string    `json:"provider_id"`
	ModelID    string    `json:"model_id"`
	Messages   []Message `json:"messages,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ChatMeta struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	ProviderID   string    `json:"provider_id"`
	ModelID      string    `json:"model_id"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}
