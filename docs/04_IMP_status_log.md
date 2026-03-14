# 04_IMP — Status Log
> Живой документ. Обновляется по мере разработки.
> Формат: дата | фаза | проблема → решение

---

## Как вести этот документ

После каждой сессии разработки добавлять запись:
```
### YYYY-MM-DD | PhaseN | Задача P#-##
**Статус:** ✅ Готово / 🔄 В процессе / ❌ Заблокировано
**Сделано:** ...
**Проблема:** ...
**Решение:** ...
**Следующий шаг:** ...
```

---

## Phase 1 — Hello World + Gemini

### Известные проблемы и решения (заготовлено заранее)

#### P1: Fyne требует CGo на macOS
```
Проблема:  go build падает с "C compiler not found"
Причина:   Fyne использует OpenGL/Metal — нужен XCode Command Line Tools
Решение:   xcode-select --install
           Занимает ~10 минут, требует macOS developer account
Проверка:  xcode-select -p → должен вернуть путь
```

#### P1: go mod tidy не находит google.golang.org/genai
```
Проблема:  cannot find module providing package google.golang.org/genai
Причина:   Старое имя пакета (до 2024 был github.com/google/generative-ai-go)
Решение:   go get google.golang.org/genai@latest
           НЕ использовать github.com/google/generative-ai-go — deprecated
```

#### P1: UI зависает при запросе к Gemini
```
Проблема:  Окно не отвечает пока ждём ответ API
Причина:   Вызов GenerateContent в main goroutine (блокирует event loop)
Решение:   Всегда оборачивать API-вызовы в go func() { ... }()
           sendBtn.Disable() перед запуском горутины
           sendBtn.Enable() в defer или после получения ответа

Код-паттерн:
    sendBtn.Disable()
    go func() {
        defer sendBtn.Enable()
        result, err := client.GenerateContent(ctx, ...)
        // обновление UI — через Refresh, не напрямую
    }()
```

#### P1: Обновление Label/RichText из горутины
```
Проблема:  Паника или артефакты при widget.SetText() из горутины
Причина:   Fyne не полностью thread-safe для сеттеров
Решение:   Fyne v2.4+ допускает Refresh() из горутины.
           Для SetText() — использовать паттерн:
               label.SetText(text)   // безопасно в Fyne v2
               label.Refresh()       // вызвать после
           При сомнениях — тестировать с -race флагом:
               go run -race ./cmd/app/
```

---

## Phase 2 — Streaming + Markdown

### Известные проблемы и решения (заготовлено заранее)

#### P2: Стриминг — курсор "мигает" при каждом Refresh
```
Проблема:  ParseMarkdown + Refresh вызывается на каждый байт → мерцание
Причина:   Слишком частые перерисовки
Решение:   Throttle через time.Ticker:

    ticker := time.NewTicker(50 * time.Millisecond)
    defer ticker.Stop()
    var buf strings.Builder

    go func() {
        for chunk := range textCh {
            buf.WriteString(chunk)
        }
    }()

    for range ticker.C {
        if buf.Len() > 0 {
            rt.ParseMarkdown(buf.String())
            rt.Refresh()
            scroll.ScrollToBottom()
        }
        if streamDone {
            break
        }
    }
```

#### P2: Незакрытые MD-токены ломают рендер
```
Проблема:  "**начало" без закрывающих ** → отображается как сырой текст
Причина:   Fyne MD-парсер ожидает полные токены
Решение:   ParseMarkdown(весь накопленный текст) — при неполном токене
           парсер просто показывает текст без форматирования.
           По завершении стриминга финальный ParseMarkdown() всё починит.
           НЕ использовать AppendMarkdown() пословно — ненадёжно.
```

#### P2: context.Cancel() не останавливает стриминг мгновенно
```
Проблема:  После нажатия "Стоп" стриминг продолжается ещё 1-2 сек
Причина:   HTTP-соединение закрывается не мгновенно
Решение:   Это нормальное поведение.
           Визуально — сразу скрыть кнопку "Стоп", показать "Отправить".
           Горутина завершится сама когда ctx.Done() сработает.
```

---

## Phase 3 — SQLite

### Известные проблемы и решения (заготовлено заранее)

#### P3: SQLITE_BUSY при конкурентных записях
```
Проблема:  "database is locked" при быстрой отправке сообщений
Причина:   Несколько горутин пишут одновременно
Решение:   Один write-connection с SetMaxOpenConns(1)
           PRAGMA busy_timeout = 5000 (ждать 5 сек перед ошибкой)
           Все записи через один последовательный writer

Конфигурация:
    writeDB.SetMaxOpenConns(1)
    writeDB.SetMaxIdleConns(1)
    writeDB.Exec("PRAGMA busy_timeout = 5000")
```

#### P3: Медленная загрузка большой истории
```
Проблема:  Чат с 500 сообщениями грузится 2-3 секунды
Причина:   LoadMessages() тянет всё разом, VBox рендерит все пузырьки
Решение:   Загружать последние 50 сообщений:
               SELECT ... ORDER BY created_at DESC LIMIT 50
           При скролле вверх — подгружать следующие 50 (пагинация)
           TODO Phase 5: виртуализированный список
```

#### P3: ON DELETE CASCADE не работает
```
Проблема:  При удалении чата сообщения остаются в БД
Причина:   SQLite требует PRAGMA foreign_keys = ON при каждом соединении
Решение:   Добавить в initPragmas:
               PRAGMA foreign_keys = ON;
           Проверить: sqlite3 history.db "PRAGMA foreign_keys" → должно быть 1
```

---

## Phase 4 — Keychain

### Известные проблемы и решения (заготовлено заранее)

#### P4: Keychain диалог на каждый запрос
```
Проблема:  macOS спрашивает разрешение при каждом чтении ключа
Причина:   KeychainTrustApplication не установлен
Решение:   В keyring.Config:
               KeychainTrustApplication: true
           Также убедиться что бинарник подписан (или запускается из .app)
```

#### P4: keyring не компилируется без CGo
```
Проблема:  99designs/keyring требует CGo на macOS (для Keychain API)
Причина:   Keychain — нативный macOS API, требует Obj-C bridge
Следствие: Кросс-компиляция с Mac на Windows для Keychain невозможна
Решение:   На macOS собирать нативно (CGo включён по умолчанию)
           На Windows — WinCred работает без CGo
           GitHub Actions: отдельный job для каждой ОС (см. 04_IMP_build_guide.md)
```

#### P4: Дебаунс при вводе API ключа
```
Проблема:  FetchModels() вызывается на каждое нажатие клавиши → 429 errors
Решение:   Паттерн дебаунса через time.AfterFunc:

    var debounceTimer *time.Timer
    keyEntry.OnChanged = func(key string) {
        if debounceTimer != nil {
            debounceTimer.Stop()
        }
        debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
            if len(strings.TrimSpace(key)) >= 30 {
                go validateAndFetchModels(key)
            }
        })
    }
```

---

## Лог разработки (заполняется по мере работы)

```
### 2026-03-13 | Phase 5+: Bugfix | Threading Issue
**Статус:** ✅ Исправлено
**Проблема:** Ошибка `Error in Fyne call thread` при обновлении списка моделей и текста в настройках.
**Причина:** Fyne v2.7 требует, чтобы все изменения UI (SetText, Refresh) происходили только в главном потоке. Фоновые горутины (AfterFunc, Stream) нарушали это правило.
**Решение:** Все вызовы UI внутри фоновых задач обернуты в `fyne.Do`.
**Результат:** Список моделей теперь подгружается корректно, ошибки в консоли устранены.
```