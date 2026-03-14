# 02_DES — Data Models (Go Structs)
> Этап: Design | Статус: ✅ | Дата: 2025-03

---

## Обзор: карта всех структур

```
┌─────────────────┐     ┌─────────────────┐     ┌──────────────────┐
│   AppConfig     │     │   ChatSession   │     │    Message       │
│─────────────────│     │─────────────────│     │──────────────────│
│ ActiveProvider  │     │ ID              │     │ ID               │
│ Providers map   │     │ Title           │     │ ChatID           │
│                 │     │ ProviderID      │     │ Role             │
└─────────────────┘     │ ModelID         │     │ Content          │
                        │ CreatedAt       │     │ Timestamp        │
┌─────────────────┐     │ UpdatedAt       │     └──────────────────┘
│ ProviderConfig  │     └─────────────────┘
│─────────────────│
│ SelectedModel   │     ┌─────────────────┐     ┌──────────────────┐
│ (no API key!)   │     │   ModelInfo     │     │   ChatMeta       │
└─────────────────┘     │─────────────────│     │──────────────────│
                        │ ID              │     │ ID               │
  API ключи хранятся    │ DisplayName     │     │ Title            │
  ТОЛЬКО в OS Keychain  │ CtxWindow       │     │ ProviderID       │
                        │ IsFree          │     │ ModelID          │
                        └─────────────────┘     │ UpdatedAt        │
                                                │ MessageCount     │
                                                └──────────────────┘
```

---

## 1. Message — единица диалога

```go
// internal/store/models.go

// Role определяет сторону диалога.
// Используем "user" / "assistant" (не "model") —
// это унифицированный формат для всех провайдеров (OpenAI-совместимый).
// Gemini SDK внутри конвертирует "assistant" → "model" при необходимости.
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    // RoleSystem зарезервирован для system prompt (не хранится в истории)
)

type Message struct {
    ID        int64     // автоинкремент из SQLite (для пагинации)
    ChatID    string    // UUID родительского чата
    Role      Role      // "user" | "assistant"
    Content   string    // текст сообщения (может содержать Markdown)
    CreatedAt time.Time // время создания
}

// ToProviderMessage конвертирует для передачи в Provider.Stream()
func (m Message) ToProviderMessage() provider.Message {
    return provider.Message{
        Role:    string(m.Role),
        Content: m.Content,
    }
}
```

---

## 2. ChatSession — сессия диалога

```go
// ChatSession — полная сессия включая сообщения.
// Используется при загрузке чата в UI.
type ChatSession struct {
    ID         string    // UUID, генерируется при создании: uuid.New().String()
    Title      string    // первые 45 символов первого сообщения пользователя
    ProviderID string    // "gemini" | "kimi" | "deepseek"
    ModelID    string    // "gemini-2.0-flash" | "moonshot-v1-8k" | ...
    Messages   []Message // загружается лениво (только при открытии чата)
    CreatedAt  time.Time
    UpdatedAt  time.Time // обновляется при каждом новом сообщении
}

// ChatMeta — облегчённая версия для отображения в sidebar (без сообщений).
// Загружается при старте для всего списка чатов.
type ChatMeta struct {
    ID           string
    Title        string
    ProviderID   string
    ModelID      string
    UpdatedAt    time.Time
    MessageCount int       // количество сообщений (из COUNT(*) запроса)
}

// ProviderIcon возвращает эмодзи-иконку провайдера для sidebar
func (c ChatMeta) ProviderIcon() string {
    switch c.ProviderID {
    case "gemini":
        return "✦" // или загрузить из assets
    case "kimi":
        return "🌙"
    case "deepseek":
        return "🔍"
    default:
        return "🤖"
    }
}

// ShortModel возвращает сокращённое имя модели для отображения в sidebar
func (c ChatMeta) ShortModel() string {
    // "gemini-2.0-flash" → "2.0 Flash"
    name := c.ModelID
    name = strings.TrimPrefix(name, "gemini-")
    name = strings.TrimPrefix(name, "moonshot-")
    name = strings.TrimPrefix(name, "deepseek-")
    // Capitalize first letter
    if len(name) > 0 {
        return strings.ToUpper(name[:1]) + name[1:]
    }
    return name
}
```

---

## 3. AppConfig — конфигурация приложения

```go
// internal/store/config.go

// AppConfig хранится в SQLite таблице `config` (key-value)
// ИЛИ в отдельном JSON-файле ~/.config/gemini-chat/config.json.
// API ключи здесь НЕ хранятся — только в OS Keychain.
type AppConfig struct {
    // Активный провайдер по умолчанию
    ActiveProvider string `json:"active_provider"` // "gemini"

    // Настройки по провайдерам
    Providers map[string]ProviderConfig `json:"providers"`

    // UI настройки
    Theme     ThemeMode `json:"theme"`      // "system" | "dark" | "light"
    FontSize  float32   `json:"font_size"`  // 14.0 (default)
    SidebarWidth float32 `json:"sidebar_width"` // 0.25 (25%)
}

// ProviderConfig — настройки конкретного провайдера.
// API ключ НЕ включён — хранится в OS Keychain под ключом провiderID.
type ProviderConfig struct {
    SelectedModel string `json:"selected_model"` // "gemini-2.0-flash"
    // В будущих версиях: BaseURL для кастомных endpoint
    // BaseURL string `json:"base_url,omitempty"`
}

type ThemeMode string
const (
    ThemeSystem ThemeMode = "system"
    ThemeDark   ThemeMode = "dark"
    ThemeLight  ThemeMode = "light"
)

// DefaultConfig возвращает конфиг по умолчанию для первого запуска
func DefaultConfig() AppConfig {
    return AppConfig{
        ActiveProvider: "gemini",
        Providers: map[string]ProviderConfig{
            "gemini":   {SelectedModel: "gemini-2.0-flash"},
            "kimi":     {SelectedModel: "moonshot-v1-8k"},
            "deepseek": {SelectedModel: "deepseek-chat"},
        },
        Theme:        ThemeSystem,
        FontSize:     14.0,
        SidebarWidth: 0.25,
    }
}
```

---

## 4. Provider types — типы для работы с LLM API

```go
// internal/provider/provider.go

// Provider — единый интерфейс для любого LLM провайдера
type Provider interface {
    ID() string   // "gemini" | "kimi" | "deepseek"
    Name() string // "Google Gemini" | "Kimi (Moonshot AI)" | "DeepSeek"

    // FetchModels получает список моделей. Для провайдеров без динамического
    // списка (Kimi, DeepSeek) — возвращает статический список.
    FetchModels(ctx context.Context, apiKey string) ([]ModelInfo, error)

    // ValidateKey проверяет валидность ключа.
    // Возвращает nil если ключ рабочий.
    ValidateKey(ctx context.Context, apiKey string) error

    // Stream отправляет сообщения и стримит ответ.
    Stream(ctx context.Context, req StreamRequest) (<-chan string, <-chan error)
}

// ModelInfo — метаданные модели
type ModelInfo struct {
    ID          string // "gemini-2.0-flash" — используется в API-вызовах
    DisplayName string // "Gemini 2.0 Flash" — для отображения в UI
    CtxWindow   int    // Размер контекстного окна (токены)
    IsFree      bool   // Бесплатный тир доступен?
}

// StreamRequest — запрос к API (одинаков для всех провайдеров)
type StreamRequest struct {
    APIKey       string    // ключ для этого запроса (из Keychain)
    ModelID      string    // "gemini-2.0-flash"
    SystemPrompt string    // системная инструкция (может быть пустой)
    Messages     []Message // полная история, последнее = новый запрос пользователя
}

// Message — унифицированный формат сообщения для провайдеров
type Message struct {
    Role    string // "user" | "assistant"
    Content string
}
```

---

## 5. KeyStore interface

```go
// internal/keystore/keystore.go

type KeyStore interface {
    // Set сохраняет API ключ для провайдера
    Set(providerID, apiKey string) error

    // Get возвращает сохранённый ключ.
    // Возвращает ("", ErrNotFound) если ключ не сохранён.
    Get(providerID string) (string, error)

    // Delete удаляет ключ из хранилища.
    Delete(providerID string) error

    // Has проверяет существование ключа без его чтения
    Has(providerID string) bool
}

var ErrNotFound = errors.New("API ключ не найден")
```

---

## 6. Store interface — хранилище данных

```go
// internal/store/store.go

type Store interface {
    // --- Конфигурация ---
    LoadConfig() (AppConfig, error)
    SaveConfig(AppConfig) error

    // --- Чаты ---
    CreateChat(chat ChatSession) error
    ListChats() ([]ChatMeta, error)
    GetChat(id string) (ChatSession, error)
    UpdateChatTitle(id, title string) error
    DeleteChat(id string) error // ON DELETE CASCADE удалит messages

    // --- Сообщения ---
    AddMessage(msg Message) error
    LoadMessages(chatID string) ([]Message, error)

    // --- Очистка ---
    ClearAllChats() error

    // Close закрывает соединение с БД
    Close() error
}
```

---

## 7. Полная схема SQLite

```sql
-- Таблица чатов
CREATE TABLE chats (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL DEFAULT 'Новый чат',
    provider_id TEXT NOT NULL DEFAULT 'gemini',
    model_id    TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Таблица сообщений
CREATE TABLE messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id     TEXT    NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL CHECK(role IN ('user','assistant')),
    content     TEXT    NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Таблица конфигурации (key-value)
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Индексы
CREATE INDEX idx_messages_chat    ON messages(chat_id, created_at ASC);
CREATE INDEX idx_chats_updated    ON chats(updated_at DESC);
```

---

## 8. Схема инициализации при первом запуске

```go
// cmd/app/main.go — wire-up

func main() {
    // 1. Открыть/создать SQLite
    db, err := store.NewSQLiteStore()
    // создаст ~/.config/gemini-chat/history.db при первом запуске

    // 2. Загрузить или создать дефолтный конфиг
    cfg, err := db.LoadConfig()
    if errors.Is(err, store.ErrNotFound) {
        cfg = store.DefaultConfig()
        db.SaveConfig(cfg)
    }

    // 3. Открыть Keychain (не блокирует — ошибка обрабатывается мягко)
    keys := keystore.NewKeyringStore()

    // 4. Инициализировать реестр провайдеров
    registry := provider.NewRegistry()
    registry.Register("gemini", gemini.New())

    // 5. Создать менеджер чата
    manager := chat.NewManager(db, keys, registry, cfg)

    // 6. Запустить UI
    fyneApp := fyne.NewFyneApp(manager)
    fyneApp.Run()
}
```

> **Принцип dependency injection:** каждый слой получает зависимости через конструктор,
> а не создаёт их сам. Это делает тестирование тривиальным — подставляем `store.MemoryStore{}`.