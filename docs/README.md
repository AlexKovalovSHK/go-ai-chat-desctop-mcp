# GeminiChat — десктопный AI-чат на Go + Fyne

Простой аналог Claude Desktop с поддержкой Gemini API.

## Быстрый старт (macOS)

### 1. Установка зависимостей
```bash
# Go 1.21+
brew install go

# Зависимости Fyne для macOS (уже есть в XCode)
xcode-select --install
```

### 2. Клонирование и запуск
```bash
cd gemini-chat
go mod tidy
go run ./cmd/app/
```

### 3. Сборка .app для macOS
```bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os darwin -name GeminiChat
# → создаст GeminiChat.app
```

## Получить бесплатный API Key
1. Открыть https://aistudio.google.com/app/apikey
2. Нажать "Create API Key"
3. Вставить в Настройки приложения (иконка ⚙️)

## Бесплатные лимиты Gemini
| Модель | Запросов/мин | Токенов/мин |
|--------|-------------|-------------|
| gemini-1.5-flash | 15 | 1,000,000 |
| gemini-2.0-flash-exp | 10 | 1,000,000 |
| gemini-1.5-pro | 2 | 32,000 |

## Структура проекта
```
cmd/app/main.go          # Точка входа
internal/
  api/gemini.go          # Клиент Gemini со стримингом
  ui/window.go           # Главное окно
  ui/settings.go         # Диалог настроек
  storage/config.go      # Хранение ключа и истории
  markdown/renderer.go   # Рендер MD → Fyne RichText
docs/
  01_RES_gemini_api.md   # Документация API
  02_DES_architecture.md # Архитектура
  03_PLN_phases.md       # План разработки
```

## Что работает в v0.1
- [x] Чат с Gemini (стриминг)
- [x] Выбор модели
- [x] История чатов (сохраняется между запусками)
- [x] Рендер Markdown (bold, italic, code, заголовки)
- [x] Настройки с сохранением API key

## Следующие шаги (Phase 4-5)
- [ ] Тёмная тема
- [ ] Копирование блоков кода
- [ ] Экспорт чата в MD-файл
- [ ] Иконка + сборка через CI

# START APP go run ./cmd/app/
# BULD APP go build -ldflags="-s -w" -o GeminiChat ./cmd/app/