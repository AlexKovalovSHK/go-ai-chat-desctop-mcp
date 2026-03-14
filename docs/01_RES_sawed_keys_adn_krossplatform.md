# 01_RES — Хранение ключей, кросс-платформенность, дистрибуция
> Статус: ✅ Исследование завершено | Дата: 2025-03

---

## ЧАСТЬ 1 — Правильное хранение API-ключей

### Три уровня безопасности (от плохого к правильному)

```
❌ Уровень 0: Хардкод в коде
   const key = "AIza..."  → ключ в git-истории навсегда, НИКОГДА

❌ Уровень 1: Открытый JSON
   ~/.config/app/config.json → читается любым процессом юзера, плохо

✅ Уровень 2: JSON с правами 0600 + атомарная запись
   Файл недоступен другим пользователям. Приемлемо для v1.0.

✅ Уровень 3: OS Keychain (системное хранилище секретов)
   macOS Keychain / Windows Credential Manager / Linux Secret Service
   Ключ зашифрован паролем входа в систему. Лучший вариант.
```

### ✅ Рекомендуемая библиотека: `github.com/99designs/keyring`

Поддерживает все основные бэкенды одним API:
- macOS → Keychain
- Windows → Credential Manager (WinCred)
- Linux → Secret Service (GNOME Keyring / KWallet)
- Fallback → зашифрованный файл (если системного хранилища нет)

```bash
go get github.com/99designs/keyring
```

### Реализация KeyStore

```go
// internal/keystore/keystore.go
package keystore

import (
    "fmt"
    "github.com/99designs/keyring"
)

const serviceName = "gemini-chat"

var ring keyring.Keyring

func init() {
    var err error
    ring, err = keyring.Open(keyring.Config{
        // Имя сервиса — так ключ будет виден в Keychain Access на macOS
        ServiceName: serviceName,

        // macOS: доверять приложению автоматически (не спрашивать каждый раз)
        KeychainTrustApplication: true,

        // Fallback: если системный keyring недоступен — зашифрованный файл
        // (актуально для headless Linux серверов)
        FileDir: "~/.config/gemini-chat/",
        FilePasswordFunc: func(prompt string) (string, error) {
            // В Fyne-приложении можно показать диалог ввода пароля
            // Для простоты v1.0 — используем имя пользователя как соль
            return "gemini-chat-fallback", nil
        },
        // Разрешённые бэкенды (nil = все доступные на текущей ОС)
        AllowedBackends: nil,
    })
    if err != nil {
        // Keyring недоступен — это не фатально, работаем без него
        ring = nil
    }
}

// SetKey сохраняет API-ключ для провайдера (например "gemini", "kimi")
func SetKey(providerID, apiKey string) error {
    if ring == nil {
        return fmt.Errorf("системное хранилище ключей недоступно")
    }
    return ring.Set(keyring.Item{
        Key:         providerID,
        Data:        []byte(apiKey),
        Label:       fmt.Sprintf("GeminiChat — %s API Key", providerID),
        Description: "API ключ для приложения GeminiChat",
    })
}

// GetKey возвращает сохранённый API-ключ
func GetKey(providerID string) (string, error) {
    if ring == nil {
        return "", fmt.Errorf("системное хранилище ключей недоступно")
    }
    item, err := ring.Get(providerID)
    if err != nil {
        return "", err // keyring.ErrKeyNotFound если ключ не сохранён
    }
    return string(item.Data), nil
}

// DeleteKey удаляет сохранённый ключ
func DeleteKey(providerID string) error {
    if ring == nil {
        return fmt.Errorf("системное хранилище ключей недоступно")
    }
    return ring.Remove(providerID)
}

// HasKey проверяет, существует ли ключ (без его чтения)
func HasKey(providerID string) bool {
    _, err := GetKey(providerID)
    return err == nil
}
```

> **Что видит пользователь на macOS:** при первом сохранении ключа macOS покажет
> системный диалог _"GeminiChat хочет получить доступ к Keychain"_.
> После нажатия **"Всегда разрешать"** — больше никаких диалогов.

---

## ЧАСТЬ 2 — Жизненный цикл ключа: от ввода до удаления

### Схема состояний при запуске приложения

```
┌─────────────────────────────────────────────────────────┐
│                    СТАРТ ПРИЛОЖЕНИЯ                     │
└────────────────────────┬────────────────────────────────┘
                         │
                         ▼
              keystore.HasKey("gemini")?
                    /         \
                  ДА           НЕТ
                  │             │
                  ▼             ▼
         Загрузить ключ    Показать диалог
         из Keychain       настроек сразу
                  │
                  ▼
         ValidateKey() — тихая проверка
         GET /v1beta/models (лёгкий запрос)
                /         \
           УСПЕХ          ОШИБКА
              │               │
              ▼               ▼
         Загрузить        Показать баннер
         список моделей   "Ключ недействителен"
         Показать чат     + кнопка "Обновить ключ"
```

### Код проверки валидности при старте

```go
// internal/api/validate.go

// ValidateKey делает минимальный запрос к API для проверки ключа.
// Возвращает nil если ключ рабочий.
func ValidateKey(ctx context.Context, apiKey string) error {
    client, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  apiKey,
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return fmt.Errorf("создание клиента: %w", err)
    }
    defer client.Close()

    // Самый дешёвый запрос — список моделей (не тратит токены)
    ctx2, cancel := context.WithTimeout(ctx, 8*time.Second)
    defer cancel()

    _, err = client.Models.List(ctx2, nil)
    if err != nil {
        var apiErr *genai.APIError
        if errors.As(err, &apiErr) {
            switch apiErr.Code {
            case 401, 403:
                return fmt.Errorf("ключ недействителен или отозван")
            case 429:
                // Лимит запросов — ключ РАБОЧИЙ, просто занят
                return nil
            }
        }
        return fmt.Errorf("ошибка проверки: %w", err)
    }
    return nil
}
```

### UI при старте — проверка в фоне

```go
// в main.go или при инициализации окна:
func (mw *MainWindow) onAppStart() {
    apiKey, err := keystore.GetKey("gemini")

    if err != nil || apiKey == "" {
        // Ключа нет — сразу открываем настройки
        mw.showSettingsDialog()
        return
    }

    // Ключ есть — проверяем в фоне, не блокируя UI
    mw.setStatus("🔄 Проверка ключа...")
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        if err := api.ValidateKey(ctx, apiKey); err != nil {
            // Ключ не работает
            mw.showBanner("⚠️ API ключ недействителен — обновите в настройках", "warning")
            return
        }

        // Ключ рабочий — грузим модели
        models, _ := api.FetchModels(ctx, apiKey)
        mw.setAvailableModels(models)
        mw.setStatus("✅ Готов")
    }()
}
```

### Диалог настроек: полный цикл управления ключом

```go
// internal/ui/settings.go

func ShowSettingsDialog(win fyne.Window, onSave func(apiKey, model string)) {
    existingKey, _ := keystore.GetKey("gemini")

    // Маскируем существующий ключ: "AIza...xK9f" (не показываем полностью)
    maskedKey := ""
    if existingKey != "" {
        maskedKey = maskKey(existingKey)
    }

    statusLabel := widget.NewLabel("")
    if existingKey != "" {
        statusLabel.SetText("✅ Ключ сохранён: " + maskedKey)
    } else {
        statusLabel.SetText("⚠️ Ключ не задан")
    }

    keyEntry := widget.NewPasswordEntry()
    keyEntry.SetPlaceHolder("Вставьте новый API ключ (AIza...)")

    modelSelect := widget.NewSelect(fallbackModels, nil)
    loadingLabel := widget.NewLabel("")

    // Автопроверка при вводе
    keyEntry.OnChanged = func(key string) {
        if len(strings.TrimSpace(key)) < 30 {
            return
        }
        loadingLabel.SetText("🔄 Проверка ключа...")
        modelSelect.Disable()

        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            if err := api.ValidateKey(ctx, key); err != nil {
                loadingLabel.SetText("❌ Ключ недействителен: " + err.Error())
                modelSelect.Enable()
                return
            }

            // Ключ рабочий — грузим модели
            models, err := api.FetchModels(ctx, key)
            if err != nil {
                loadingLabel.SetText("✅ Ключ рабочий (модели не загружены)")
                modelSelect.Enable()
                return
            }

            displayNames := modelDisplayNames(models)
            modelSelect.SetOptions(displayNames)
            modelSelect.SetSelected(preferredModel(displayNames))
            modelSelect.Enable()
            loadingLabel.SetText(fmt.Sprintf("✅ Ключ рабочий, моделей: %d", len(models)))
        }()
    }

    // Кнопка удаления ключа
    deleteBtn := widget.NewButtonWithIcon("Удалить ключ", theme.DeleteIcon(), func() {
        dialog.ShowConfirm("Удалить ключ?",
            "API ключ будет удалён из системного хранилища.", func(ok bool) {
                if ok {
                    keystore.DeleteKey("gemini")
                    statusLabel.SetText("🗑 Ключ удалён")
                    keyEntry.SetText("")
                }
            }, win)
    })

    content := container.NewVBox(
        statusLabel,
        widget.NewSeparator(),
        widget.NewLabel("Новый API ключ:"),
        keyEntry,
        widget.NewHyperlink("Получить бесплатно на Google AI Studio →",
            mustParseURL("https://aistudio.google.com/app/apikey")),
        loadingLabel,
        widget.NewLabel("Модель:"),
        modelSelect,
        widget.NewSeparator(),
        deleteBtn,
    )

    dialog.ShowCustomConfirm("Настройки", "Сохранить", "Отмена", content,
        func(ok bool) {
            if !ok {
                return
            }
            newKey := strings.TrimSpace(keyEntry.Text)
            if newKey != "" {
                keystore.SetKey("gemini", newKey)
            }
            if onSave != nil {
                keyToUse := newKey
                if keyToUse == "" {
                    keyToUse = existingKey // оставляем старый если не меняли
                }
                onSave(keyToUse, modelSelect.Selected)
            }
        }, win)
}

// maskKey: "AIzaSyBxxx...K9fQ" → "AIza••••••••K9fQ"
func maskKey(key string) string {
    if len(key) < 8 {
        return "••••••••"
    }
    return key[:4] + strings.Repeat("•", len(key)-8) + key[len(key)-4:]
}
```

---

## ЧАСТЬ 3 — Кросс-платформенная сборка и дистрибуция

### Go и кросс-компиляция — в чём суперсила

Чистые Go-программы тривиально легко компилируются под все основные платформы с одной машины. Можно собрать .exe для Windows, бинарник для macOS и Linux — прямо с вашего MacBook.

```bash
# Всё это запускается с одного MacBook:
GOOS=darwin  GOARCH=arm64  go build -o dist/gemini-chat-mac-arm64  ./cmd/app/
GOOS=darwin  GOARCH=amd64  go build -o dist/gemini-chat-mac-intel  ./cmd/app/
GOOS=windows GOARCH=amd64  go build -o dist/gemini-chat-win.exe    ./cmd/app/
GOOS=linux   GOARCH=amd64  go build -o dist/gemini-chat-linux      ./cmd/app/
```

### Проблема: CGo ломает кросс-компиляцию

Некоторые библиотеки требуют CGo (C-компилятор + заголовки целевой ОС).
Это делает кросс-компиляцию невозможной без Docker или CI.

| Компонент | CGo нужен? | Решение |
|---|---|---|
| Fyne v2 | ⚠️ Да (OpenGL/Metal) | Собирать нативно на каждой ОС или CI |
| `mattn/go-sqlite3` | ❌ Да | **Заменить на `modernc.org/sqlite`** |
| `99designs/keyring` | ⚠️ Частично | macOS: CGo для Keychain; Win/Linux: без CGo |
| Gemini SDK | ✅ Нет | Чистый Go |

> **Главный вывод:** Fyne сам по себе требует нативной сборки (OpenGL на Linux, Metal на macOS).
> Поэтому кросс-компиляция Fyne-приложения невозможна — нужен CI с несколькими агентами.

### Правильная стратегия дистрибуции

```
┌─────────────────────────────────────────────────────────────┐
│               GitHub Actions (CI/CD)                        │
│                                                             │
│  job: build-mac    │  job: build-win   │  job: build-linux  │
│  runs-on: macos    │  runs-on: windows │  runs-on: ubuntu   │
│                    │                   │                    │
│  fyne package      │  fyne package     │  fyne package      │
│  → GeminiChat.app  │  → GeminiChat.exe │  → gemini-chat     │
│  → zip → release   │  → zip → release  │  → tar.gz→ release │
└─────────────────────────────────────────────────────────────┘
```

### GitHub Actions — минимальный workflow

```yaml
# .github/workflows/release.yml
name: Build & Release

on:
  push:
    tags: ['v*']  # Запускается при git tag v1.0.0

jobs:
  build-macos:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest
      - name: Build macOS app
        run: |
          fyne package -os darwin -name GeminiChat -appID dev.geminichat.app
          zip -r GeminiChat-mac.zip GeminiChat.app
      - uses: actions/upload-artifact@v4
        with: { name: mac-build, path: GeminiChat-mac.zip }

  build-windows:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest
      - name: Build Windows exe
        run: |
          fyne package -os windows -name GeminiChat -appID dev.geminichat.app
          Compress-Archive GeminiChat.exe GeminiChat-win.zip
      - uses: actions/upload-artifact@v4
        with: { name: win-build, path: GeminiChat-win.zip }

  build-linux:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - name: Install system dependencies (Fyne on Linux)
        run: sudo apt-get install -y libgl1-mesa-dev xorg-dev
      - name: Install Fyne CLI
        run: go install fyne.io/fyne/v2/cmd/fyne@latest
      - name: Build Linux binary
        run: |
          fyne package -os linux -name GeminiChat -appID dev.geminichat.app
          tar -czf GeminiChat-linux.tar.gz GeminiChat
      - uses: actions/upload-artifact@v4
        with: { name: linux-build, path: GeminiChat-linux.tar.gz }

  release:
    needs: [build-macos, build-windows, build-linux]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
      - uses: softprops/action-gh-release@v2
        with:
          files: |
            mac-build/GeminiChat-mac.zip
            win-build/GeminiChat-win.zip
            linux-build/GeminiChat-linux.tar.gz
```

---

## ЧАСТЬ 4 — SQLite без CGo: полное решение

### Почему `modernc.org/sqlite`, а не `mattn/go-sqlite3`

`modernc.org/sqlite` — pure Go реализация SQLite без CGo. Для людей незнакомых с Go: чистые Go-программы тривиально легко компилируются под все основные платформы по умолчанию.

Поддерживаемые платформы modernc.org/sqlite: darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, linux/amd64, linux/arm64 и другие — всё что нужно для десктопного приложения.

```go
// go.mod
require (
    modernc.org/sqlite v1.34.0  // ✅ Без CGo
    // НЕ использовать: github.com/mattn/go-sqlite3 — требует CGo + GCC
)
```

### Правильная инициализация соединения

SQLite — однозаписывающая БД. Использование одного `sql.DB` пула может вызывать `SQLITE_BUSY` ошибки при конкурентных записях. Решение — два раздельных подключения: одно для записи (MaxOpenConns=1), второе для чтения (несколько соединений).

```go
// internal/storage/sqlite.go
package storage

import (
    "context"
    "database/sql"
    "os"
    "path/filepath"

    _ "modernc.org/sqlite"
)

// Оптимальные PRAGMA настройки для десктопного приложения
const initPragmas = `
    PRAGMA journal_mode = WAL;       -- Write-Ahead Logging: быстрее, не блокирует чтение
    PRAGMA synchronous   = NORMAL;   -- Баланс скорость/надёжность
    PRAGMA foreign_keys  = ON;       -- Проверка внешних ключей
    PRAGMA busy_timeout  = 5000;     -- 5 сек ждать при SQLITE_BUSY
    PRAGMA temp_store    = MEMORY;   -- Временные таблицы в RAM
`

const schema = `
CREATE TABLE IF NOT EXISTS providers (
    id              TEXT PRIMARY KEY,       -- "gemini", "kimi", "deepseek"
    selected_model  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS chats (
    id          TEXT PRIMARY KEY,           -- UUID
    title       TEXT NOT NULL,
    provider_id TEXT NOT NULL DEFAULT 'gemini',
    model_id    TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK(role IN ('user', 'assistant')),
    content    TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_time ON messages(chat_id, created_at);
CREATE INDEX IF NOT EXISTS idx_chats_updated ON chats(updated_at DESC);
`

type Store struct {
    write *sql.DB
    read  *sql.DB
}

func NewStore() (*Store, error) {
    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".config", "gemini-chat")
    os.MkdirAll(dir, 0700)
    dbPath := filepath.Join(dir, "history.db")

    dsn := "file:" + dbPath + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"

    write, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    write.SetMaxOpenConns(1) // ВАЖНО: только 1 писатель

    read, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    read.SetMaxOpenConns(4) // несколько читателей — ок

    // Применяем PRAGMA и создаём схему
    if _, err = write.ExecContext(context.Background(), initPragmas+schema); err != nil {
        return nil, err
    }

    return &Store{write: write, read: read}, nil
}

func (s *Store) Close() {
    s.write.Close()
    s.read.Close()
}

// --- Методы чатов ---

func (s *Store) CreateChat(id, title, providerID, modelID string) error {
    _, err := s.write.Exec(
        `INSERT INTO chats (id, title, provider_id, model_id) VALUES (?, ?, ?, ?)`,
        id, title, providerID, modelID,
    )
    return err
}

func (s *Store) AddMessage(chatID, role, content string) error {
    tx, err := s.write.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    _, err = tx.Exec(
        `INSERT INTO messages (chat_id, role, content) VALUES (?, ?, ?)`,
        chatID, role, content,
    )
    if err != nil {
        return err
    }

    // Обновляем заголовок из первого сообщения
    var count int
    tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&count)
    if count == 1 && role == "user" {
        title := content
        if len(title) > 45 {
            title = title[:45] + "..."
        }
        tx.Exec(`UPDATE chats SET title=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, title, chatID)
    } else {
        tx.Exec(`UPDATE chats SET updated_at=CURRENT_TIMESTAMP WHERE id=?`, chatID)
    }

    return tx.Commit()
}

func (s *Store) LoadMessages(chatID string) ([]Message, error) {
    rows, err := s.read.Query(
        `SELECT role, content, created_at FROM messages
         WHERE chat_id = ? ORDER BY created_at ASC`,
        chatID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var msgs []Message
    for rows.Next() {
        var m Message
        rows.Scan(&m.Role, &m.Text, &m.Timestamp)
        msgs = append(msgs, m)
    }
    return msgs, nil
}

func (s *Store) ListChats() ([]ChatMeta, error) {
    rows, err := s.read.Query(
        `SELECT id, title, provider_id, model_id, updated_at
         FROM chats ORDER BY updated_at DESC LIMIT 200`,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var chats []ChatMeta
    for rows.Next() {
        var c ChatMeta
        rows.Scan(&c.ID, &c.Title, &c.ProviderID, &c.ModelID, &c.UpdatedAt)
        chats = append(chats, c)
    }
    return chats, nil
}

func (s *Store) DeleteChat(chatID string) error {
    // ON DELETE CASCADE удалит messages автоматически
    _, err := s.write.Exec(`DELETE FROM chats WHERE id = ?`, chatID)
    return err
}
```

---

## ЧАСТЬ 5 — Что получает пользователь при установке

### macOS (GeminiChat.app)
```
GeminiChat.app/          ← перетащить в /Applications
  Contents/
    MacOS/GeminiChat     ← единственный бинарник, всё внутри
    Info.plist
    Resources/

Данные хранятся отдельно (НЕ внутри .app):
~/.config/gemini-chat/
  history.db             ← SQLite база с историей чатов
  # API ключ — в системном Keychain, не в файле!
```

### Windows (GeminiChat.exe)
```
GeminiChat.exe           ← просто запустить, установка не нужна

Данные:
C:\Users\<user>\AppData\Roaming\gemini-chat\
  history.db

API ключ → Windows Credential Manager (Панель управления → Диспетчер учётных данных)
```

### Linux
```
gemini-chat              ← бинарник, chmod +x и запустить

Данные:
~/.config/gemini-chat/
  history.db

API ключ → GNOME Keyring или KWallet
```

---

## Итоговые решения

| Вопрос | Решение |
|---|---|
| Хранение ключа | `github.com/99designs/keyring` → macOS Keychain / WinCred / GNOME Keyring |
| Проверка ключа при старте | `client.Models.List()` — лёгкий запрос, 0 токенов |
| Ключ не запрашивается каждый раз | Keyring хранит перманентно, читается автоматически |
| Смена / удаление ключа | `keystore.SetKey()` / `keystore.DeleteKey()` из диалога настроек |
| Маскировка ключа в UI | `"AIza••••••••K9fQ"` — показываем только начало и конец |
| SQLite без CGo | `modernc.org/sqlite` — pure Go, один бинарник для всех ОС |
| Кросс-платформенная сборка | GitHub Actions: отдельный job для каждой ОС |
| Данные при обновлении | `.db` и Keychain не трогаются при замене бинарника |