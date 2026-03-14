package store

import (
	"github.com/alexkovalov/gemini-chat/internal/models"
)

type Store interface {
	// --- Chats ---
	CreateChat(chat models.ChatSession) error
	ListChats() ([]models.ChatMeta, error)
	GetChat(id string) (models.ChatSession, error)
	UpdateChatTitle(id, title string) error
	DeleteChat(id string) error

	// --- Messages ---
	AddMessage(msg models.Message) error
	LoadMessages(chatID string) ([]models.Message, error)

	// --- Config ---
	GetConfig(key string) (string, error)
	SetConfig(key, value string) error

	// Close closes the database connection
	Close() error
}
