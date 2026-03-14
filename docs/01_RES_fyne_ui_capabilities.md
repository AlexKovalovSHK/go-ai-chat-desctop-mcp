# 01_RES — Fyne UI Capabilities
> Статус: ✅ Исследование завершено | Fyne: v2.5.x | macOS | Дата: 2025-03

---

## 1. Прокручиваемый список сообщений

### Выбор подхода

В Fyne есть два кандидата:

**`widget.List`** — виртуализированный список (только видимые элементы в памяти).
Проблема: требует фиксированной высоты элементов через `SetItemHeight`, плохо работает с RichText переменной высоты.

**`container.NewVBox` + `container.NewVScroll`** — ✅ наш выбор.
Все элементы в памяти, зато динамическая высота и простота.

```go
messageBox := container.NewVBox()
scroll     := container.NewVScroll(messageBox)

// Добавить сообщение:
func appendMessage(role, text string) {
    bubble := makeBubble(role, text)
    messageBox.Add(bubble)
    messageBox.Refresh()
    scroll.ScrollToBottom() // автоскролл
}
```

**Ограничение:** при > 500 сообщений возможны подтормаживания.
**Решение:** при загрузке истории отображать только последние 100 сообщений.

### ⚠️ Thread safety — главная ловушка Fyne

Fyne не является thread-safe. Изменение UI из горутины вызывает панику или артефакты рендеринга.

```go
// ❌ Неправильно
go func() {
    messageBox.Add(newBubble)  // ПАНИКА или артефакты
}()

// ✅ Правильно — Fyne v2 автоматически синхронизирует Refresh через main thread
go func() {
    // ... получили данные из API ...
    richText.ParseMarkdown(accumulated)
    richText.Refresh()          // безопасно из горутины в Fyne v2
    scroll.ScrollToBottom()
}()
```

---

## 2. Поле ввода текста

### Многострочный Entry (наш выбор)

```go
entry := widget.NewMultiLineEntry()
entry.SetMinRowsVisible(3)
entry.SetPlaceHolder("Введите сообщение... (Enter — отправить, Shift+Enter — новая строка)")
entry.Wrapping = fyne.TextWrapWord

// Enter без Shift → отправить
entry.OnSubmitted = func(text string) {
    if strings.TrimSpace(text) == "" {
        return
    }
    go sendMessage(text)   // API-вызов в горутину
    entry.SetText("")
}
```

### Кнопка отправки справа от поля

```go
sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
    go sendMessage(entry.Text)
    entry.SetText("")
})
sendBtn.Importance = widget.HighImportance // акцентный цвет темы

inputBar := container.NewBorder(nil, nil, nil, sendBtn, entry)
// Border: top=nil, bottom=nil, left=nil, right=sendBtn, center=entry
```

---

## 3. Поддержка Markdown — встроенные средства Fyne

### Главный вывод: сторонние библиотеки не нужны!

Начиная с **Fyne v2.1** (2021), `widget.RichText` содержит встроенный MD-парсер.
В **Fyne v2.5** (2024) добавлен `AppendMarkdown`.

```go
// Создание из MD
rt := widget.NewRichTextFromMarkdown("**Жирный** и *курсив*\n```go\nfmt.Println()\n```")
rt.Wrapping = fyne.TextWrapWord  // ОБЯЗАТЕЛЬНО! Иначе текст обрежется

// Замена контента
rt.ParseMarkdown("# Новый заголовок")
rt.Refresh()

// Добавление (Fyne v2.5+)
rt.AppendMarkdown("\n## Дополнение")
rt.Refresh()
```

### Что поддерживает встроенный парсер

| MD-синтаксис | Поддержка | Примечание |
|---|---|---|
| `**bold**` | ✅ | |
| `*italic*` | ✅ | |
| `` `code` `` | ✅ | Monospace inline |
| ` ```lang\n...\n``` ` | ✅ | Monospace блок |
| `# ## ###` | ✅ | Размер + Bold |
| `- * списки` | ✅ | |
| `1. нумерованные` | ✅ | |
| `> цитаты` | ✅ | Italic + отступ |
| `[текст](url)` | ✅ | Кликабельные ссылки |
| `---` линия | ✅ | |
| Таблицы `\|...\|` | ❌ | Нет в парсере |
| `~~strikethrough~~` | ❌ | Нет в парсере |
| HTML-теги | ❌ | |

### Паттерн стриминга MD

AppendMarkdown предназначен для готовых MD-документов, а не фрагментов.
При стриминге правильный подход — перепарсивать накопленный текст:

```go
var buf strings.Builder
rt := widget.NewRichTextFromMarkdown("")
rt.Wrapping = fyne.TextWrapWord

// В горутине стриминга:
for chunk := range chunkCh {
    buf.WriteString(chunk)
    rt.ParseMarkdown(buf.String()) // перепарс целиком
    rt.Refresh()
    scroll.ScrollToBottom()
}
```

Почему не AppendMarkdown пословно? Незакрытые MD-токены (например `**бол` без закрывающего `**`) дают артефакты рендеринга. ParseMarkdown(полный текст) надёжнее.

### Ручное управление сегментами (для полного контроля)

```go
segs := []widget.RichTextSegment{
    &widget.TextSegment{Text: "Обычный ",  Style: widget.RichTextStyleInline},
    &widget.TextSegment{Text: "жирный",    Style: widget.RichTextStyleStrong},
    &widget.TextSegment{Text: " и ",       Style: widget.RichTextStyleInline},
    &widget.TextSegment{Text: "курсив",    Style: widget.RichTextStyleEmphasis},
}
rt := widget.NewRichText(segs...)
```

Доступные встроенные стили:
- `RichTextStyleInline` — обычный
- `RichTextStyleStrong` — жирный
- `RichTextStyleEmphasis` — курсив
- `RichTextStyleCodeInline` — моноширинный inline
- `RichTextStyleCodeBlock` — блок кода (Monospace, отдельная строка)
- `RichTextStyleHeading` — H1
- `RichTextStyleSubHeading` — H2
- `RichTextStyleBlockquote` — цитата

---

## 4. Смена тем Light / Dark

### macOS: автоматически из коробки

Fyne v2.3+ автоматически следует системной теме macOS через NSAppearance.
Никаких дополнительных действий не требуется.

```go
a := app.New()
// Тема переключается автоматически при изменении системных настроек macOS
```

### Принудительное переключение по кнопке

`theme.DarkTheme()` и `theme.LightTheme()` **deprecated** в v2.5, будут удалены в v3.
Правильный паттерн — `forcedVariant` (взят из исходников fyne_demo):

```go
// internal/ui/theme.go
type forcedVariant struct {
    fyne.Theme
    variant fyne.ThemeVariant
}

func (f *forcedVariant) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
    return f.Theme.Color(name, f.variant) // игнорируем системный вариант, форсируем свой
}

// Применение:
func applyDark(a fyne.App) {
    a.Settings().SetTheme(&forcedVariant{theme.DefaultTheme(), theme.VariantDark})
}
func applyLight(a fyne.App) {
    a.Settings().SetTheme(&forcedVariant{theme.DefaultTheme(), theme.VariantLight})
}
func applySystem(a fyne.App) {
    a.Settings().SetTheme(theme.DefaultTheme()) // вернуть авто
}
```

### Кастомная тема (пример с цветами Gemini)

```go
type appTheme struct{}

func (appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
    switch n {
    case theme.ColorNamePrimary:
        return color.NRGBA{R: 0x1a, G: 0x73, B: 0xe8, A: 0xff} // Google Blue
    case theme.ColorNameFocus:
        return color.NRGBA{R: 0x1a, G: 0x73, B: 0xe8, A: 0x7f}
    }
    return theme.DefaultTheme().Color(n, v) // всё остальное — по умолчанию
}
func (appTheme) Font(s fyne.TextStyle) fyne.Resource  { return theme.DefaultTheme().Font(s) }
func (appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (appTheme) Size(n fyne.ThemeSizeName) float32   { return theme.DefaultTheme().Size(n) }
```

### ThemeOverride — только для части UI (Fyne v2.5)

```go
// Например, сделать левую панель всегда тёмной, независимо от системы:
darkLeftPanel := container.NewThemeOverride(leftPanel, &forcedVariant{
    Theme:   theme.DefaultTheme(),
    variant: theme.VariantDark,
})
```

---

## 5. Рекомендуемый лейаут чат-окна

```go
// 1. Левая панель
newChatBtn := widget.NewButtonWithIcon("Новый чат", theme.ContentAddIcon(), newChatFn)
chatList   := widget.NewList(...)
leftPanel  := container.NewBorder(newChatBtn, nil, nil, nil, chatList)

// 2. Область сообщений
messageBox    := container.NewVBox()
messageScroll := container.NewVScroll(messageBox)

// 3. Строка ввода
inputEntry := widget.NewMultiLineEntry()
sendBtn    := widget.NewButtonWithIcon("", theme.MailSendIcon(), sendFn)
inputBar   := container.NewBorder(nil, nil, nil,
    container.NewVBox(sendBtn), // кнопка прижата вправо
    inputEntry,
)

// 4. Правая панель
rightPanel := container.NewBorder(nil,
    container.NewVBox(widget.NewSeparator(), inputBar),
    nil, nil,
    messageScroll,
)

// 5. Главный сплит
split := container.NewHSplit(leftPanel, rightPanel)
split.SetOffset(0.25) // 25% левая, 75% правая

window.SetContent(split)
```

---

## 6. Зависимости

```
fyne.io/fyne/v2 v2.5.2
```

Сторонние MD-библиотеки (goldmark, blackfriday и т.д.) **не нужны** — встроенный парсер Fyne покрывает все потребности чата.

---

## 7. Известные ограничения

| Проблема | Решение |
|---|---|
| RichText не поддерживает MD-таблицы | Отображать как preformatted (code block) |
| List плохо работает с разной высотой | VBox + Scroll (наш подход) |
| Нет syntax highlighting кода | Моноширинный шрифт без раскраски |
| `~~strikethrough~~` не парсится | Принять как ограничение |
| Смена темы не обновляет часть виджетов | Добавить `canvas.Refresh(w.Canvas())` |
| AppendMarkdown ломает незакрытые токены | ParseMarkdown(полный_текст) при стриминге |