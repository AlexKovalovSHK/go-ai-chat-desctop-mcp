package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexkovalov/gemini-chat/internal/models"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Apply Pragmals
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("failed to set pragma %q: %v", p, err)
		}
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS chats (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL DEFAULT 'Новый чат',
		provider_id TEXT NOT NULL DEFAULT 'gemini',
		model_id    TEXT NOT NULL,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id     TEXT    NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		role        TEXT    NOT NULL CHECK(role IN ('user','assistant')),
		content     TEXT    NOT NULL,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS config (
		key         TEXT PRIMARY KEY,
		value       TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id, created_at ASC);
	CREATE INDEX IF NOT EXISTS idx_chats_updated ON chats(updated_at DESC);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) CreateChat(chat models.ChatSession) error {
	query := `INSERT INTO chats (id, title, provider_id, model_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, chat.ID, chat.Title, chat.ProviderID, chat.ModelID, chat.CreatedAt, chat.UpdatedAt)
	return err
}

func (s *SQLiteStore) ListChats() ([]models.ChatMeta, error) {
	query := `
		SELECT c.id, c.title, c.provider_id, c.model_id, c.updated_at, COUNT(m.id) as msg_count
		FROM chats c
		LEFT JOIN messages m ON c.id = m.chat_id
		GROUP BY c.id
		ORDER BY c.updated_at DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []models.ChatMeta
	for rows.Next() {
		var c models.ChatMeta
		if err := rows.Scan(&c.ID, &c.Title, &c.ProviderID, &c.ModelID, &c.UpdatedAt, &c.MessageCount); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}
	return chats, nil
}

func (s *SQLiteStore) GetChat(id string) (models.ChatSession, error) {
	var c models.ChatSession
	query := `SELECT id, title, provider_id, model_id, created_at, updated_at FROM chats WHERE id = ?`
	err := s.db.QueryRow(query, id).Scan(&c.ID, &c.Title, &c.ProviderID, &c.ModelID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (s *SQLiteStore) UpdateChatTitle(id, title string) error {
	_, err := s.db.Exec(`UPDATE chats SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, id)
	return err
}

func (s *SQLiteStore) DeleteChat(id string) error {
	_, err := s.db.Exec(`DELETE FROM chats WHERE id = ?`, id)
	return err
}

func (s *SQLiteStore) AddMessage(msg models.Message) error {
	query := `INSERT INTO messages (chat_id, role, content, created_at) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, msg.ChatID, msg.Role, msg.Content, msg.CreatedAt)
	if err != nil {
		return err
	}
	// Update chat updated_at
	_, err = s.db.Exec(`UPDATE chats SET updated_at = ? WHERE id = ?`, msg.CreatedAt, msg.ChatID)
	return err
}

func (s *SQLiteStore) LoadMessages(chatID string) ([]models.Message, error) {
	query := `SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = ? ORDER BY created_at ASC`
	rows, err := s.db.Query(query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (s *SQLiteStore) GetConfig(key string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

func (s *SQLiteStore) SetConfig(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`, key, value, value)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
