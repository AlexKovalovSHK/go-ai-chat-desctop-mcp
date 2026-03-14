# 02_DES — UI Layout Specification
> Этап: Design | Статус: ✅ | Дата: 2025-03

---

## 1. Главное окно — обзор

```
┌──────────────────────────────────────────────────────────────────────┐
│  ✦ GeminiChat                                          [ ⚙ ]  [–][□][×]│
├─────────────────┬────────────────────────────────────────────────────┤
│                 │                                                    │
│  SIDEBAR        │  CHAT VIEW                                         │
│  (25%)          │  (75%)                                             │
│                 │                                                    │
│  [+ Новый чат]  │                                                    │
│  ─────────────  │                                                    │
│  ✦ 2.0 Flash   │                                                    │
│  > Объясни Go  │                                                    │
│                 │                                                    │
│  ✦ 2.0 Flash   │                                                    │
│  > Помоги с SQL│                                                    │
│                 │                                                    │
│  ✦ 1.5 Pro     │                                                    │
│  > Напиши тест │                                                    │
│                 │                                                    │
│  ...            │                                                    │
│                 │                                                    │
│                 │                                                    │
│  ─────────────  │  ──────────────────────────────────────────────── │
│  [🗑 Удалить]   │  [  Введите сообщение...                   ] [▶] │
└─────────────────┴────────────────────────────────────────────────────┘
```

---

## 2. Sidebar — левая панель

```
┌──────────────────┐
│ [➕  Новый чат ] │  ← кнопка, HighImportance (акцентный цвет)
│ ──────────────── │
│                  │
│  ┌────────────┐  │
│  │ ✦ 2.0Flash │  │  ← выбранный чат (highlighted background)
│  │ Объясни Go │  │
│  └────────────┘  │
│                  │
│  ┌────────────┐  │
│  │ ✦ 2.0Flash │  │  ← обычный чат
│  │ Помоги с S │  │    иконка провайдера + короткое имя модели
│  └────────────┘  │
│                  │
│  ┌────────────┐  │
│  │ ✦ 1.5 Pro  │  │
│  │ Напиши тест│  │
│  └────────────┘  │
│                  │
│  ... (scroll)    │
│                  │
│ ──────────────── │
│ [ 🗑 Удалить  ] │  ← удалить текущий чат (secondary importance)
└──────────────────┘
```

**Реализация Fyne:**
```go
// internal/ui/fyne/sidebar.go

func (s *Sidebar) build() fyne.CanvasObject {
    s.list = widget.NewList(
        func() int { return len(s.chats) },
        func() fyne.CanvasObject {
            icon  := widget.NewLabel("✦")
            model := widget.NewLabel("model")
            model.TextStyle = fyne.TextStyle{Italic: true}
            title := widget.NewLabel("title")
            return container.NewVBox(
                container.NewHBox(icon, model),
                title,
            )
        },
        func(i widget.ListItemID, o fyne.CanvasObject) {
            box   := o.(*fyne.Container)
            hbox  := box.Objects[0].(*fyne.Container)
            icon  := hbox.Objects[0].(*widget.Label)
            model := hbox.Objects[1].(*widget.Label)
            title := box.Objects[1].(*widget.Label)

            c := s.chats[i]
            icon.SetText(c.ProviderIcon())
            model.SetText(c.ShortModel())
            title.SetText(c.Title)
        },
    )
    s.list.OnSelected = func(id widget.ListItemID) {
        s.onSelect(s.chats[id].ID)
    }

    newBtn    := widget.NewButtonWithIcon("Новый чат", theme.ContentAddIcon(), s.onNew)
    newBtn.Importance = widget.HighImportance

    deleteBtn := widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), func() {
        if s.selectedID != "" {
            s.onDelete(s.selectedID)
        }
    })

    return container.NewBorder(newBtn, deleteBtn, nil, nil, s.list)
}
```

---

## 3. Chat View — центральная панель

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  👤 Вы                               12:34     │  │
│  │  ───────────────────────────────────────────   │  │
│  │  Объясни горутины в Go простыми словами        │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  ✦ Gemini 2.0 Flash                  12:34    │  │
│  │  ───────────────────────────────────────────   │  │
│  │  Горутина — это лёгкий поток выполнения в Go.  │  │
│  │                                                 │  │
│  │  ## Ключевые особенности                       │  │
│  │  - Запускается через `go func()`               │  │
│  │  - Весит ~2KB (vs ~2MB для потока ОС)          │  │
│  │                                                 │  │
│  │  ```go                                          │  │
│  │  go func() {                                   │  │
│  │      fmt.Println("hello from goroutine")        │  │
│  │  }()                                           │  │
│  │  ```                                           │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  👤 Вы                               12:35     │  │
│  │  ───────────────────────────────────────────   │  │
│  │  А как они общаются между собой?               │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  ✦ Gemini 2.0 Flash                  12:35    │  │
│  │  ───────────────────────────────────────────   │  │
│  │  ▌  (стриминг — курсор мигает)                 │  │
│  └────────────────────────────────────────────────┘  │
│                                                      │
│  ────────────────────────────────────────────────    │
│  ┌─────────────────────────────────────────┐  [▶]   │
│  │  Введите сообщение...                   │        │
│  │                                         │        │
│  └─────────────────────────────────────────┘        │
│  ✅ Готов: gemini-2.0-flash                          │
└──────────────────────────────────────────────────────┘
```

**Реализация bubble:**
```go
// internal/ui/fyne/bubble.go

type BubbleStyle struct {
    IsUser bool
}

func NewBubble(role Role, content string) *ChatBubble {
    isUser := role == RoleUser

    // Заголовок: иконка + имя + время
    var icon, name string
    if isUser {
        icon, name = "👤", "Вы"
    } else {
        icon, name = "✦", "Gemini" // TODO: динамически по провайдеру
    }

    timestamp := widget.NewLabelWithStyle(
        time.Now().Format("15:04"),
        fyne.TextAlignTrailing,
        fyne.TextStyle{Italic: true},
    )

    header := container.NewBorder(nil, nil, nil, timestamp,
        widget.NewLabelWithStyle(icon+" "+name, fyne.TextAlignLeading,
            fyne.TextStyle{Bold: true}),
    )

    // Контент с Markdown
    richText := widget.NewRichTextFromMarkdown(content)
    richText.Wrapping = fyne.TextWrapWord

    bubble := container.NewVBox(
        header,
        widget.NewSeparator(),
        richText,
        widget.NewSeparator(),
    )

    return &ChatBubble{container: bubble, richText: richText}
}

// AppendChunk добавляет чанк при стриминге
func (b *ChatBubble) AppendChunk(accumulated string) {
    b.richText.ParseMarkdown(accumulated)
    b.richText.Refresh()
}
```

---

## 4. Диалог настроек

```
┌─────────────────────────────────────────────────────┐
│  ⚙  Настройки                                       │
│                                                     │
│  ┌─────────────────────────────────────────────┐   │
│  │  ✅ Ключ сохранён: AIza••••••••K9fQ         │   │
│  └─────────────────────────────────────────────┘   │
│  ─────────────────────────────────────────────      │
│                                                     │
│  Новый API ключ:                                    │
│  ┌─────────────────────────────────────────────┐   │
│  │  ●●●●●●●●●●●●●●●●●●●●●●●●         (👁)   │   │
│  └─────────────────────────────────────────────┘   │
│  Получить бесплатно на Google AI Studio →           │
│                                                     │
│  ✅ Ключ рабочий, доступно моделей: 12              │
│                                                     │
│  Модель:                                            │
│  ┌─────────────────────────────────────────────┐   │
│  │  Gemini 2.0 Flash                      [▼]  │   │
│  └─────────────────────────────────────────────┘   │
│  Контекст: 1 000 000 токенов | Бесплатный тир       │
│                                                     │
│  Тема:                                              │
│  ( ) Системная  (●) Тёмная  ( ) Светлая             │
│                                                     │
│  ─────────────────────────────────────────────      │
│  [ 🗑 Удалить ключ ]                                │
│                                                     │
│  ─────────────────────────────────────────────      │
│         [  Отмена  ]      [  Сохранить  ]           │
└─────────────────────────────────────────────────────┘
```

**Состояния диалога настроек:**

| Ситуация | Отображение |
|---|---|
| Ключ не сохранён | `⚠️ Ключ не задан` + поле пустое |
| Ключ сохранён, не меняем | `✅ Ключ сохранён: AIza••••K9fQ` + поле пустое |
| Вводим новый (< 30 символов) | Поле активно, без проверки |
| Вводим новый (≥ 30 символов) | `🔄 Проверка ключа...` + модели задизейблены |
| Ключ рабочий | `✅ Ключ рабочий, моделей: 12` + модели загружены |
| Ключ не рабочий | `❌ Ключ недействителен: 401 Unauthorized` |

---

## 5. Строка статуса (внизу chat view)

```
[ ✅ Готов: gemini-2.0-flash              • 1 024 / 1 000 000 токенов ]
[ 🔄 Генерация ответа...                                               ]
[ ❌ Ошибка: превышен лимит запросов. Подождите 60 сек.    [Повторить] ]
[ ⚠️  API ключ недействителен                          [Настройки →]  ]
```

---

## 6. Состояния приложения при первом запуске

```
Первый запуск:
┌─────────────────────────────────────────────────────┐
│  Добро пожаловать в GeminiChat!                    │
│                                                    │
│  Для начала введите API ключ Google Gemini.        │
│  Ключ хранится в системном Keychain macOS —        │
│  он не покидает ваш компьютер.                     │
│                                                    │
│  [  Получить бесплатный ключ →  ]                  │
│  [  Ввести ключ                 ]                  │
└─────────────────────────────────────────────────────┘

Последующие запуски:
┌─────────────────────────────────────────────────────┐
│  (приложение запускается сразу с историей)         │
│  (тихая проверка ключа в фоне за 1-2 секунды)      │
└─────────────────────────────────────────────────────┘
```

---

## 7. Responsive: изменение размера окна

| Ширина окна | Поведение |
|---|---|
| > 800px | Sidebar видна (offset 0.25) |
| 600–800px | Sidebar сужается (offset 0.20) |
| < 600px | Sidebar скрывается, кнопка hamburger [☰] |

```go
// Реагируем на изменение размера окна
win.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {})
win.Resize(fyne.NewSize(900, 650)) // начальный размер

// TODO Phase 5: автоскрытие sidebar при узком окне
```

---

## 8. Fyne Layout — иерархия контейнеров

```
window.SetContent(
  container.NewBorder(toolbar, nil, nil, nil,        // toolbar сверху
    container.NewHSplit(                              // горизонтальный сплит
      sidebar.Build(),                               // левая панель
      container.NewBorder(nil,                       // правая панель
        container.NewVBox(
          widget.NewSeparator(),
          inputbar.Build(),                          // поле ввода снизу
          statusbar.Build(),                         // статус ещё ниже
        ),
        nil, nil,
        chatview.Build(),                            // сообщения в центре
      ),
    ),
  ),
)
```