# 01_RES — Gemini API Go SDK
> Статус: ✅ Исследование завершено | SDK: google.golang.org/genai v1.x | Дата: 2025-03

---

## 1. Важно: два разных SDK — не перепутайте!

| SDK | Модуль | Статус |
|-----|--------|--------|
| **Новый (наш выбор)** | `google.golang.org/genai` | ✅ GA, активно развивается |
| Старый (legacy) | `github.com/google/generative-ai-go/genai` | ❌ Deprecated с конца 2024 |

Начиная с Gemini 2.0 (конец 2024) Google перешёл на новый unified SDK.
Старый SDK больше не получает новых функций, поддержка прекращена.

```go
// ✅ Новый SDK — используем его
import "google.golang.org/genai"

// ❌ Старый SDK — НЕ использовать
import "github.com/google/generative-ai-go/genai"
```

---

## 2. Установка

```bash
go get google.golang.org/genai
```

**go.mod:**
```
require google.golang.org/genai v1.0.0  // или актуальная версия
```

---

## 3. Создание клиента

### Gemini Developer API (наш случай — бесплатный ключ из AI Studio)

```go
import (
    "context"
    "google.golang.org/genai"
)

ctx := context.Background()

client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:  "AIza...",              // ключ из aistudio.google.com
    Backend: genai.BackendGeminiAPI, // явно указываем, не Vertex AI
})
if err != nil {
    return fmt.Errorf("create client: %w", err)
}
defer client.Close()
```

### Через переменные окружения (альтернатива)
```bash
export GEMINI_API_KEY="AIza..."
```
```go
// Тогда APIKey можно не передавать — SDK подхватит из env
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    Backend: genai.BackendGeminiAPI,
})
```

---

## 4. Доступные модели (Free Tier, актуально на 2025-03)

| Модель | RPM | TPM | Input | Output |
|--------|-----|-----|-------|--------|
| `gemini-2.5-flash` | 10 | 250 000 | 1M токенов | 8 192 |
| `gemini-2.0-flash` | 15 | 1 000 000 | 1M токенов | 8 192 |
| `gemini-1.5-flash` | 15 | 1 000 000 | 1M токенов | 8 192 |
| `gemini-1.5-flash-8b` | 15 | 1 000 000 | 1M токенов | 8 192 |
| `gemini-1.5-pro` | 2 | 32 000 | 2M токенов | 8 192 |

RPM = requests per minute, TPM = tokens per minute (бесплатный тир)

**Рекомендация для разработки:** `gemini-2.0-flash` — быстрый, щедрые лимиты, поддерживает streaming.

---

## 5. Структура запроса

### Базовый запрос (без истории)

```go
result, err := client.Models.GenerateContent(
    ctx,
    "gemini-2.0-flash",
    genai.Text("Привет! Как дела?"),
    nil, // config — nil для дефолтных настроек
)
if err != nil {
    return err
}
fmt.Println(result.Text()) // удобный хелпер
```

### Структура Contents (история чата)

API принимает массив `[]*genai.Content`, каждый элемент — один "ход":

```go
contents := []*genai.Content{
    {
        Role:  "user",
        Parts: []*genai.Part{{Text: "Расскажи о Go"}},
    },
    {
        Role:  "model",
        Parts: []*genai.Part{{Text: "Go — это компилируемый язык..."}},
    },
    {
        Role:  "user",
        Parts: []*genai.Part{{Text: "А что такое горутины?"}},
    },
}

result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", contents, nil)
```

> **Ключевое правило:** история должна чередоваться `user → model → user → model...`
> и всегда заканчиваться на `user`. Нарушение этого порядка вызывает ошибку API.

### System Instruction

System Instruction — инструкция для модели, которая не видна пользователю и устанавливает "личность" или контекст:

```go
config := &genai.GenerateContentConfig{
    SystemInstruction: &genai.Content{
        Parts: []*genai.Part{
            {Text: "Ты — полезный AI-ассистент. Отвечай кратко и по делу. Используй Markdown для форматирования кода."},
        },
    },
    Temperature:     genai.Ptr[float32](0.9),
    MaxOutputTokens: 2048,
}

result, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", contents, config)
```

> **Важно:** System Instruction передаётся в `config`, не в `contents`. Она не добавляется в историю и не считается "ходом".

---

## 6. Chat API (высокоуровневый)

Новый SDK предоставляет `client.Chats` — объект, который автоматически ведёт историю:

```go
// Создание чат-сессии
chat, err := client.Chats.Create(ctx, "gemini-2.0-flash",
    &genai.GenerateContentConfig{
        SystemInstruction: &genai.Content{
            Parts: []*genai.Part{{Text: "Ты полезный ассистент"}},
        },
    },
    nil, // начальная история — nil (или передать []*genai.Content)
)

// Отправка сообщения (история ведётся автоматически)
result, err := chat.SendMessage(ctx, genai.Part{Text: "Привет!"})
fmt.Println(result.Text())

// Следующее сообщение — история уже включает предыдущий обмен
result, err = chat.SendMessage(ctx, genai.Part{Text: "Что ты умеешь?"})

// Получить текущую историю
history := chat.History(false) // false = comprehensive history
```

### ⚠️ Проблема Chat API с нашей архитектурой

Chat API хранит историю в памяти внутри объекта `*Chat`.
При перезапуске приложения история теряется.

**Решение:** не использовать `client.Chats` для хранения истории.
Вместо этого сохранять историю самостоятельно (в JSON) и передавать через `GenerateContent` с полным массивом `contents`.

```go
// Наш подход: каждый раз передаём всю историю явно
func (c *Client) SendWithHistory(ctx context.Context, messages []storage.Message, sysPrompt string) (*genai.GenerateContentResponse, error) {
    contents := make([]*genai.Content, 0, len(messages))
    for _, m := range messages {
        contents = append(contents, &genai.Content{
            Role:  m.Role,
            Parts: []*genai.Part{{Text: m.Text}},
        })
    }

    config := &genai.GenerateContentConfig{}
    if sysPrompt != "" {
        config.SystemInstruction = &genai.Content{
            Parts: []*genai.Part{{Text: sysPrompt}},
        }
    }

    return c.inner.Models.GenerateContent(ctx, c.model, contents, config)
}
```

---

## 7. Streaming (потоковые ответы)

### Базовый стриминг

```go
// GenerateContentStream возвращает итератор
iter := client.Models.GenerateContentStream(ctx, "gemini-2.0-flash", contents, nil)

for result, err := range iter { // Go 1.23+ range over iterator
    if err != nil {
        return fmt.Errorf("stream: %w", err)
    }
    fmt.Print(result.Text()) // печатаем чанк без \n
}
```

### Совместимость с Go < 1.23 (через Next())

```go
for {
    result, err := iter.Next()
    if err == iterator.Done {
        break
    }
    if err != nil {
        return fmt.Errorf("stream: %w", err)
    }
    // Извлечь текст из ответа
    for _, cand := range result.Candidates {
        if cand.Content != nil {
            for _, part := range cand.Content.Parts {
                if part.Text != "" {
                    fmt.Print(part.Text)
                }
            }
        }
    }
}
```

### Стриминг в горутину с каналом (паттерн для нашего UI)

```go
func (c *Client) StreamMessage(
    ctx context.Context,
    messages []storage.Message,
) (<-chan string, <-chan error) {
    textCh := make(chan string, 64)  // буферизованный
    errCh  := make(chan error, 1)

    go func() {
        defer close(textCh)
        defer close(errCh)

        // Собираем contents из истории
        contents := buildContents(messages)

        config := &genai.GenerateContentConfig{
            SystemInstruction: &genai.Content{
                Parts: []*genai.Part{{Text: c.systemPrompt}},
            },
            Temperature: genai.Ptr[float32](0.9),
        }

        iter := c.inner.Models.GenerateContentStream(ctx, c.model, contents, config)

        for result, err := range iter {
            if err != nil {
                errCh <- err
                return
            }
            chunk := result.Text()
            if chunk != "" {
                textCh <- chunk
            }
        }
    }()

    return textCh, errCh
}

// Использование в UI:
textCh, errCh := client.StreamMessage(ctx, chatHistory)
for chunk := range textCh {
    buf.WriteString(chunk)
    richText.ParseMarkdown(buf.String())
    richText.Refresh()
}
if err := <-errCh; err != nil {
    showError(err)
}
```

---

## 8. GenerateContentConfig — все параметры

```go
config := &genai.GenerateContentConfig{
    // Системная инструкция (личность бота)
    SystemInstruction: &genai.Content{
        Parts: []*genai.Part{{Text: "Ты полезный ассистент"}},
    },

    // Параметры генерации
    Temperature:     genai.Ptr[float32](0.9),  // 0.0 = детерминированно, 2.0 = максимум случайности
    TopP:            genai.Ptr[float32](0.95),
    TopK:            genai.Ptr[int32](40),
    MaxOutputTokens: 4096,

    // Формат ответа (по умолчанию text/plain, модель сама использует MD)
    // ResponseMIMEType: "text/plain",
}
```

> **Совет:** Temperature 0.7–0.9 хорошо работает для чата. Для технических ответов — 0.3–0.5.

---

## 9. Обработка ошибок

```go
result, err := client.Models.GenerateContent(ctx, model, contents, config)
if err != nil {
    // Проверяем тип ошибки
    var apiErr *genai.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.Code {
        case 400:
            // Bad request — неверный формат contents (нарушена очерёдность ролей)
        case 401:
            // Invalid API key
        case 429:
            // Rate limit exceeded — нужен exponential backoff
        case 503:
            // Service unavailable — временная проблема на стороне Google
        }
        return fmt.Errorf("API error %d: %s", apiErr.Code, apiErr.Message)
    }
    // Другие ошибки (сеть, таймаут)
    if errors.Is(err, context.DeadlineExceeded) {
        return fmt.Errorf("request timeout")
    }
    return fmt.Errorf("unexpected error: %w", err)
}
```

### Проверка FinishReason

```go
for _, cand := range result.Candidates {
    switch cand.FinishReason {
    case genai.FinishReasonStop:
        // Нормальное завершение
    case genai.FinishReasonMaxTokens:
        // Ответ обрезан — предупредить пользователя
    case genai.FinishReasonSafety:
        // Заблокировано safety filters
        return "", fmt.Errorf("ответ заблокирован фильтром безопасности")
    }
}
```

---

## 10. Полный пример клиента для нашего приложения

```go
// internal/api/gemini.go
package api

import (
    "context"
    "errors"
    "fmt"

    "gemini-chat/internal/storage"
    "google.golang.org/genai"
)

const defaultSystemPrompt = `Ты полезный AI-ассистент.
Отвечай на языке пользователя.
Для кода используй markdown блоки с указанием языка.
Будь краток, но информативен.`

var AvailableModels = []string{
    "gemini-2.0-flash",
    "gemini-2.5-flash",
    "gemini-1.5-flash",
    "gemini-1.5-flash-8b",
    "gemini-1.5-pro",
}

type Client struct {
    inner        *genai.Client
    model        string
    systemPrompt string
}

func New(apiKey, model string) (*Client, error) {
    if apiKey == "" {
        return nil, errors.New("API key не задан")
    }
    ctx := context.Background()
    c, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  apiKey,
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return nil, fmt.Errorf("создание клиента Gemini: %w", err)
    }
    return &Client{
        inner:        c,
        model:        model,
        systemPrompt: defaultSystemPrompt,
    }, nil
}

func (c *Client) Close() { c.inner.Close() }

// Stream отправляет историю сообщений и возвращает каналы для чтения ответа.
// Последнее сообщение в messages должно быть от "user".
func (c *Client) Stream(ctx context.Context, messages []storage.Message) (<-chan string, <-chan error) {
    textCh := make(chan string, 64)
    errCh  := make(chan error, 1)

    go func() {
        defer close(textCh)
        defer close(errCh)

        // Конвертация истории
        contents := make([]*genai.Content, 0, len(messages))
        for _, m := range messages {
            contents = append(contents, &genai.Content{
                Role:  m.Role, // "user" или "model"
                Parts: []*genai.Part{{Text: m.Text}},
            })
        }

        config := &genai.GenerateContentConfig{
            SystemInstruction: &genai.Content{
                Parts: []*genai.Part{{Text: c.systemPrompt}},
            },
            Temperature: genai.Ptr[float32](0.9),
            MaxOutputTokens: 4096,
        }

        iter := c.inner.Models.GenerateContentStream(ctx, c.model, contents, config)

        for result, err := range iter {
            if err != nil {
                errCh <- fmt.Errorf("stream: %w", err)
                return
            }
            if chunk := result.Text(); chunk != "" {
                textCh <- chunk
            }
        }
    }()

    return textCh, errCh
}
```

---

## 11. Миграция со старого SDK (справка)

Если в проекте есть старый код с `github.com/google/generative-ai-go`:

| Старый API | Новый API |
|---|---|
| `genai.NewClient(ctx, option.WithAPIKey(key))` | `genai.NewClient(ctx, &genai.ClientConfig{APIKey: key})` |
| `client.GenerativeModel("model")` | удалён; используйте `client.Models.GenerateContent(ctx, "model", ...)` |
| `model.StartChat()` | `client.Chats.Create(ctx, "model", config, history)` |
| `cs.SendMessageStream(ctx, genai.Text(s))` | `client.Models.GenerateContentStream(ctx, "model", contents, config)` |
| `genai.Text("...")` | `genai.Part{Text: "..."}` |
| `option.WithAPIKey(key)` | `&genai.ClientConfig{APIKey: key}` |

---

## Итог для реализации

1. Используем `google.golang.org/genai` (новый SDK)
2. Авторизация через `ClientConfig{APIKey: ..., Backend: BackendGeminiAPI}`
3. История хранится в нашем `storage.Config` (JSON), передаётся явно при каждом запросе
4. Стриминг через `GenerateContentStream` → канал `chan string` → `ParseMarkdown` в UI
5. System Instruction задаётся через `GenerateContentConfig.SystemInstruction`
6. Ошибки проверяем через `*genai.APIError` с кодом 429 (rate limit) и 401 (неверный ключ)

=======================================

# 01_RES — Gemini API Go SDK (v2)
> Статус: ✅ Обновлено | SDK: google.golang.org/genai v1.x | Дата: 2025-03
> Изменения: +динамический список моделей, +архитектура мульти-провайдера, +сравнение хранилищ

---

## ЧАСТЬ 1 — Динамический список моделей по API-ключу

### Проблема с хардкодом
Хардкодить список моделей — плохая практика:
- Google регулярно добавляет новые модели (gemini-2.5-flash появился в 2025)
- Платные аккаунты видят модели, которых нет в бесплатном тире
- Fine-tuned модели у каждого пользователя свои

### ✅ client.Models.List — официальный метод

```go
// Получить все модели, поддерживающие generateContent
func FetchAvailableModels(ctx context.Context, apiKey string) ([]string, error) {
    client, err := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey:  apiKey,
        Backend: genai.BackendGeminiAPI,
    })
    if err != nil {
        return nil, fmt.Errorf("создание клиента: %w", err)
    }
    defer client.Close()

    page, err := client.Models.List(ctx, &genai.ListModelsConfig{})
    if err != nil {
        return nil, fmt.Errorf("список моделей: %w", err)
    }

    var names []string
    for _, m := range page.Items {
        // Фильтруем: только модели с поддержкой чата
        for _, action := range m.SupportedActions {
            if action == "generateContent" {
                // m.Name = "models/gemini-2.0-flash" → нам нужен "gemini-2.0-flash"
                name := strings.TrimPrefix(m.Name, "models/")
                names = append(names, name)
                break
            }
        }
    }

    // Сортировка: сначала флагманские (2.x), потом 1.5
    sort.Slice(names, func(i, j int) bool {
        return names[i] > names[j] // обратная алфавитная = новые сверху
    })

    return names, nil
}
```

### Паттерн в UI — "Загрузить модели" при вводе ключа

```go
// В диалоге настроек:
keyEntry.OnChanged = func(key string) {
    if len(key) < 30 { // AIza... минимум 39 символов
        return
    }
    modelSelect.Disable()
    modelSelect.SetOptions([]string{"Загрузка..."})

    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        models, err := FetchAvailableModels(ctx, key)
        if err != nil {
            // Показываем fallback список — не оставляем пользователя без выбора
            modelSelect.SetOptions(fallbackModels)
            modelSelect.SetSelected(fallbackModels[0])
            modelSelect.Enable()
            return
        }
        modelSelect.SetOptions(models)
        if len(models) > 0 {
            // Предпочитаем gemini-2.0-flash как дефолт
            preferred := preferredModel(models)
            modelSelect.SetSelected(preferred)
        }
        modelSelect.Enable()
    }()
}

// Fallback если API недоступен или ключ ещё вводится
var fallbackModels = []string{
    "gemini-2.0-flash",
    "gemini-1.5-flash",
    "gemini-1.5-flash-8b",
    "gemini-1.5-pro",
}

func preferredModel(models []string) string {
    preferred := []string{"gemini-2.0-flash", "gemini-1.5-flash", "gemini-2.5-flash"}
    for _, p := range preferred {
        for _, m := range models {
            if m == p {
                return m
            }
        }
    }
    return models[0]
}
```

### Структура объекта Model (что возвращает API)

```go
type Model struct {
    Name             string   // "models/gemini-2.0-flash"
    DisplayName      string   // "Gemini 2.0 Flash"
    Description      string   // Описание модели
    SupportedActions []string // ["generateContent", "embedContent", ...]
    InputTokenLimit  int32    // Максимум входных токенов
    OutputTokenLimit int32    // Максимум выходных токенов
}
```

> **Полезно для UI:** `m.DisplayName` даёт красивые имена — "Gemini 2.0 Flash" вместо "gemini-2.0-flash".
> `m.InputTokenLimit` можно показывать в тултипе.

---

## ЧАСТЬ 2 — Архитектура мульти-провайдера (Gemini + Kimi + DeepSeek)

### Принцип: Provider Interface

Правильная архитектура для поддержки нескольких LLM-провайдеров — интерфейс.
Это позволит добавить Kimi/DeepSeek в будущем без переписывания UI и storage.

```go
// internal/provider/provider.go

// Provider — единый интерфейс для любого LLM
type Provider interface {
    // Name возвращает человекочитаемое имя провайдера
    Name() string

    // FetchModels получает список доступных моделей по ключу (или возвращает статический список)
    FetchModels(ctx context.Context, apiKey string) ([]Model, error)

    // Stream отправляет сообщения и стримит ответ в канал
    Stream(ctx context.Context, req StreamRequest) (<-chan string, <-chan error)

    // ValidateKey проверяет, что ключ рабочий (простой тестовый запрос)
    ValidateKey(ctx context.Context, apiKey string) error
}

type Model struct {
    ID          string // "gemini-2.0-flash" / "moonshot-v1-8k" / "deepseek-chat"
    DisplayName string // "Gemini 2.0 Flash" / "Moonshot v1 8K" / "DeepSeek Chat"
    CtxWindow   int    // Размер контекстного окна в токенах
}

type StreamRequest struct {
    APIKey       string
    ModelID      string
    SystemPrompt string
    Messages     []Message // История чата
}

type Message struct {
    Role string // "user" | "assistant" (унифицировано для всех провайдеров)
    Text string
}
```

### Реализация для Gemini

```go
// internal/provider/gemini/gemini.go
type GeminiProvider struct{}

func (g *GeminiProvider) Name() string { return "Google Gemini" }

func (g *GeminiProvider) FetchModels(ctx context.Context, apiKey string) ([]provider.Model, error) {
    client, _ := genai.NewClient(ctx, &genai.ClientConfig{
        APIKey: apiKey, Backend: genai.BackendGeminiAPI,
    })
    defer client.Close()

    page, err := client.Models.List(ctx, nil)
    if err != nil {
        return nil, err
    }

    var models []provider.Model
    for _, m := range page.Items {
        for _, a := range m.SupportedActions {
            if a == "generateContent" {
                models = append(models, provider.Model{
                    ID:          strings.TrimPrefix(m.Name, "models/"),
                    DisplayName: m.DisplayName,
                    CtxWindow:   int(m.InputTokenLimit),
                })
                break
            }
        }
    }
    return models, nil
}

func (g *GeminiProvider) Stream(ctx context.Context, req provider.StreamRequest) (<-chan string, <-chan error) {
    // ... реализация через genai SDK (как в предыдущем документе)
}
```

### Заготовка для Kimi (Moonshot) — будущий релиз

```go
// internal/provider/kimi/kimi.go
// Kimi использует OpenAI-совместимый API — никакого специального SDK не нужно!
type KimiProvider struct{}

func (k *KimiProvider) Name() string { return "Kimi (Moonshot AI)" }

func (k *KimiProvider) FetchModels(_ context.Context, _ string) ([]provider.Model, error) {
    // Kimi не имеет динамического list endpoint — статический список
    return []provider.Model{
        {ID: "moonshot-v1-8k",   DisplayName: "Moonshot v1 8K",   CtxWindow: 8000},
        {ID: "moonshot-v1-32k",  DisplayName: "Moonshot v1 32K",  CtxWindow: 32000},
        {ID: "moonshot-v1-128k", DisplayName: "Moonshot v1 128K", CtxWindow: 128000},
    }, nil
}

// Stream будет использовать OpenAI-compatible endpoint:
// POST https://api.moonshot.cn/v1/chat/completions
// Authorization: Bearer {api_key}
// Реализуется через github.com/sashabaranov/go-openai
```

### Заготовка для DeepSeek — будущий релиз

```go
// internal/provider/deepseek/deepseek.go
// DeepSeek также OpenAI-совместим!
type DeepSeekProvider struct{}

func (d *DeepSeekProvider) Name() string { return "DeepSeek" }

// Endpoint: https://api.deepseek.com
// Модели: deepseek-chat, deepseek-reasoner
// Авторизация: Bearer token — идентичен OpenAI SDK
```

### Реестр провайдеров

```go
// internal/provider/registry.go

var Registry = map[string]Provider{
    "gemini":   &gemini.GeminiProvider{},
    // В будущих релизах:
    // "kimi":     &kimi.KimiProvider{},
    // "deepseek": &deepseek.DeepSeekProvider{},
}

// GetProvider возвращает провайдер по ID из конфига
func GetProvider(id string) (Provider, error) {
    p, ok := Registry[id]
    if !ok {
        return nil, fmt.Errorf("неизвестный провайдер: %s", id)
    }
    return p, nil
}
```

> **Вывод по мульти-провайдеру:** Kimi и DeepSeek используют OpenAI-совместимый API.
> Для них достаточно одной библиотеки `github.com/sashabaranov/go-openai` с разными `BaseURL`.
> Никакого дополнительного SDK не понадобится.

---

## ЧАСТЬ 3 — Хранение истории чатов: JSON vs SQLite

### Сравнительная таблица

| Критерий | JSON-файл | SQLite |
|---|---|---|
| Сложность | ✅ Минимальная | ⚠️ Нужна схема + миграции |
| Зависимости | ✅ Ноль (encoding/json в stdlib) | ⚠️ Нужен драйвер |
| CGo | ✅ Не нужен | ⚠️ mattn/go-sqlite3 требует CGo |
| Без CGo (macOS/Win/Linux) | ✅ | ✅ modernc.org/sqlite (pure Go) |
| Читаемость | ✅ Открыть в TextEdit | ❌ Бинарный формат |
| Поиск по чатам | ❌ Только в памяти | ✅ SQL LIKE / FTS5 |
| Скорость при 1000+ чатах | ⚠️ Медленная загрузка | ✅ Быстро по индексам |
| Транзакции / целостность | ❌ | ✅ ACID |
| Резервная копия | ✅ Просто скопировать файл | ✅ Тоже один файл .db |
| Подходит для v1.0 | ✅ Идеально | ⚠️ Избыточно |

### Вывод: JSON для v1.0, SQLite начиная с v2.0

**Рекомендация:**
- **v1.0** → JSON-файл (`~/.config/gemini-chat/history.json`)
  Нулевые зависимости, просто, работает. Достаточно для < 200 чатов.
- **v2.0** → SQLite с `modernc.org/sqlite` (pure Go, без CGo!)
  Когда нужен поиск, пагинация, теги чатов, несколько аккаунтов.

---

## ЧАСТЬ 4 — Реализация хранилища

### Вариант A: JSON (рекомендован для v1.0)

```go
// internal/storage/json_store.go
package storage

import (
    "encoding/json"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type Message struct {
    Role      string    `json:"role"`       // "user" | "assistant"
    Text      string    `json:"text"`
    Timestamp time.Time `json:"ts"`
}

type Chat struct {
    ID         string    `json:"id"`
    Title      string    `json:"title"`
    ProviderID string    `json:"provider"`   // "gemini" | "kimi" | "deepseek"
    ModelID    string    `json:"model"`
    Messages   []Message `json:"messages"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

type AppConfig struct {
    // Настройки по провайдерам
    Providers map[string]ProviderConfig `json:"providers"`
    // Последний активный провайдер
    ActiveProvider string `json:"active_provider"`
    // История чатов
    Chats []Chat `json:"chats"`
}

type ProviderConfig struct {
    APIKey       string `json:"api_key"`
    SelectedModel string `json:"selected_model"`
}

type JSONStore struct {
    mu   sync.RWMutex
    path string
    cfg  *AppConfig
}

func NewJSONStore() (*JSONStore, error) {
    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".config", "gemini-chat")
    if err := os.MkdirAll(dir, 0700); err != nil {
        return nil, err
    }
    s := &JSONStore{
        path: filepath.Join(dir, "data.json"),
        cfg:  &AppConfig{
            Providers:      make(map[string]ProviderConfig),
            ActiveProvider: "gemini",
            Chats:          []Chat{},
        },
    }
    _ = s.load() // игнорируем ошибку — файл может не существовать
    return s, nil
}

func (s *JSONStore) load() error {
    data, err := os.ReadFile(s.path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, s.cfg)
}

func (s *JSONStore) Save() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    data, err := json.MarshalIndent(s.cfg, "", "  ")
    if err != nil {
        return err
    }
    // Атомарная запись: сначала во временный файл, затем rename
    tmp := s.path + ".tmp"
    if err := os.WriteFile(tmp, data, 0600); err != nil {
        return err
    }
    return os.Rename(tmp, s.path) // атомарная замена на macOS/Linux
}

// --- Методы для работы с чатами ---

func (s *JSONStore) GetConfig() *AppConfig {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.cfg
}

func (s *JSONStore) SetProviderKey(providerID, apiKey string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    cfg := s.cfg.Providers[providerID]
    cfg.APIKey = apiKey
    s.cfg.Providers[providerID] = cfg
}

func (s *JSONStore) SetProviderModel(providerID, modelID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    cfg := s.cfg.Providers[providerID]
    cfg.SelectedModel = modelID
    s.cfg.Providers[providerID] = cfg
}

func (s *JSONStore) AddChat(chat Chat) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.cfg.Chats = append([]Chat{chat}, s.cfg.Chats...) // новые сверху
}

func (s *JSONStore) AppendMessage(chatID string, msg Message) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.cfg.Chats {
        if s.cfg.Chats[i].ID == chatID {
            s.cfg.Chats[i].Messages = append(s.cfg.Chats[i].Messages, msg)
            s.cfg.Chats[i].UpdatedAt = time.Now()
            // Автозаголовок из первого сообщения пользователя
            if len(s.cfg.Chats[i].Messages) == 1 {
                title := msg.Text
                if len(title) > 45 {
                    title = title[:45] + "..."
                }
                s.cfg.Chats[i].Title = title
            }
            return
        }
    }
}

func (s *JSONStore) DeleteChat(chatID string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    filtered := s.cfg.Chats[:0]
    for _, c := range s.cfg.Chats {
        if c.ID != chatID {
            filtered = append(filtered, c)
        }
    }
    s.cfg.Chats = filtered
}
```

> **Атомарная запись** через temp файл + rename — защита от корруции при внезапном выключении macOS.

---

### Вариант B: SQLite (для v2.0, pure Go без CGo)

```go
// go.mod
require modernc.org/sqlite v1.34.0  // ✅ Без CGo, работает на darwin/amd64 и arm64
```

```go
// internal/storage/sqlite_store.go
package storage

import (
    "database/sql"
    "os"
    "path/filepath"

    _ "modernc.org/sqlite" // регистрирует драйвер "sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS chats (
    id          TEXT PRIMARY KEY,
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

-- Индекс для быстрой загрузки сообщений чата
CREATE INDEX IF NOT EXISTS idx_messages_chat ON messages(chat_id, created_at);

-- Виртуальная таблица для полнотекстового поиска (опционально, v2.x)
-- CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(content, content=messages);
`

type SQLiteStore struct {
    db *sql.DB
}

func NewSQLiteStore() (*SQLiteStore, error) {
    home, _ := os.UserHomeDir()
    dir := filepath.Join(home, ".config", "gemini-chat")
    os.MkdirAll(dir, 0700)

    dbPath := filepath.Join(dir, "history.db")

    // WAL mode: быстрые конкурентные записи, не блокирует чтение
    db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
    if err != nil {
        return nil, err
    }

    if _, err := db.Exec(schema); err != nil {
        return nil, err
    }

    return &SQLiteStore{db: db}, nil
}

// AddMessage добавляет сообщение — одна SQL операция, очень быстро
func (s *SQLiteStore) AddMessage(chatID, role, content string) error {
    _, err := s.db.Exec(
        `INSERT INTO messages (chat_id, role, content) VALUES (?, ?, ?)`,
        chatID, role, content,
    )
    if err == nil {
        s.db.Exec(`UPDATE chats SET updated_at=CURRENT_TIMESTAMP WHERE id=?`, chatID)
    }
    return err
}

// LoadMessages загружает только нужный чат — не всю историю в память
func (s *SQLiteStore) LoadMessages(chatID string, limit int) ([]Message, error) {
    rows, err := s.db.Query(
        `SELECT role, content, created_at FROM messages
         WHERE chat_id = ? ORDER BY created_at ASC LIMIT ?`,
        chatID, limit,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    // ... scan rows
}

// SearchChats — будущая v2.x фича, возможна только с SQLite
func (s *SQLiteStore) SearchChats(query string) ([]Chat, error) {
    rows, err := s.db.Query(
        `SELECT DISTINCT c.id, c.title FROM chats c
         JOIN messages m ON m.chat_id = c.id
         WHERE m.content LIKE ? LIMIT 20`,
        "%"+query+"%",
    )
    // ...
}
```

---

## ЧАСТЬ 5 — Конфигурация с учётом мульти-провайдера

```json
// ~/.config/gemini-chat/data.json (итоговая структура для v1.0)
{
  "active_provider": "gemini",
  "providers": {
    "gemini": {
      "api_key": "AIza...",
      "selected_model": "gemini-2.0-flash"
    },
    "kimi": {
      "api_key": "",
      "selected_model": "moonshot-v1-8k"
    },
    "deepseek": {
      "api_key": "",
      "selected_model": "deepseek-chat"
    }
  },
  "chats": [
    {
      "id": "uuid-...",
      "title": "Первые 45 символов...",
      "provider": "gemini",
      "model": "gemini-2.0-flash",
      "messages": [],
      "created_at": "2025-03-01T10:00:00Z",
      "updated_at": "2025-03-01T10:05:00Z"
    }
  ]
}
```

> **Ключевое решение:** поле `"provider"` в каждом чате позволит в будущем корректно
> отображать иконку провайдера и передавать нужный API-ключ при restore сессии.

---

## Итоговые решения

| Вопрос | Решение |
|---|---|
| Список моделей | `client.Models.List()` + fallback список + кэш на сессию |
| Триггер загрузки | `OnChanged` поля API-ключа, с debounce 500ms |
| Мульти-провайдер | `Provider` интерфейс + реестр, Kimi/DeepSeek через `go-openai` |
| Хранилище v1.0 | JSON-файл с атомарной записью, `sync.RWMutex` |
| Хранилище v2.0 | `modernc.org/sqlite` (pure Go, без CGo, без GCC на macOS) |
| Поле `role` | `"user"` / `"assistant"` (единый формат для всех провайдеров) |