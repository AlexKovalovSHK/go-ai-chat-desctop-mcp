package fyne

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/alexkovalov/gemini-chat/internal/models"
)

type ChatBubble struct {
	Container    *fyne.Container
	RichText     *widget.RichText // только для user-пузырьков и стриминга
	contentBox   *fyne.Container  // для assistant — динамический контент
	isUser       bool
	streamBuffer string
}

func NewBubble(role models.Role, content string) *ChatBubble {
	isUser := role == models.RoleUser

	var icon, name string
	if isUser {
		icon, name = "👤", "Вы"
	} else {
		icon, name = "✦", "Gemini"
	}

	timestamp := widget.NewLabelWithStyle(
		time.Now().Format("15:04"),
		fyne.TextAlignTrailing,
		fyne.TextStyle{Italic: true},
	)
	header := container.NewBorder(nil, nil, nil, timestamp,
		widget.NewLabelWithStyle(icon+" "+name,
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	b := &ChatBubble{isUser: isUser}

	var contentWidget fyne.CanvasObject

	if isUser {
		// Для пользователя — просто RichText, код не нужен
		rt := widget.NewRichTextFromMarkdown(content)
		rt.Wrapping = fyne.TextWrapWord
		b.RichText = rt
		contentWidget = rt
	} else {
		// Для Gemini — парсим блоки, код в Entry
		b.contentBox = container.NewVBox()
		b.renderContent(content)
		contentWidget = b.contentBox
	}

	b.Container = container.NewVBox(
		header,
		widget.NewSeparator(),
		contentWidget,
		widget.NewSeparator(),
	)

	return b
}

// renderContent разбирает текст на блоки и строит contentBox заново
func (b *ChatBubble) renderContent(text string) {
	b.contentBox.Objects = nil

	for _, block := range parseBlocks(text) {
		if block.isCode {
			b.contentBox.Add(makeCodeBlock(block.lang, block.content))
		} else if strings.TrimSpace(block.content) != "" {
			rt := widget.NewRichTextFromMarkdown(block.content)
			rt.Wrapping = fyne.TextWrapWord
			b.contentBox.Add(rt)
		}
	}

	b.contentBox.Refresh()
}

// AppendChunk — вызывается при стриминге (каждый чанк)
func (b *ChatBubble) AppendChunk(accumulated string) {
	if b.isUser {
		b.RichText.ParseMarkdown(accumulated)
		b.RichText.Refresh()
		return
	}
	b.streamBuffer = accumulated
	b.renderContent(accumulated + " ▌") // курсор стриминга
}

// SetContent — вызывается по завершении стриминга
func (b *ChatBubble) SetContent(content string) {
	if b.isUser {
		b.RichText.ParseMarkdown(content)
		b.RichText.Refresh()
		return
	}
	b.renderContent(content) // без курсора ▌
}

// SetStreaming — совместимость с существующим кодом
func (b *ChatBubble) SetStreaming(streaming bool, accumulated string) {
	if b.isUser {
		text := accumulated
		if streaming {
			text += " ▌"
		}
		b.RichText.ParseMarkdown(text)
		b.RichText.Refresh()
		return
	}
	if streaming {
		b.renderContent(accumulated + " ▌")
	} else {
		b.renderContent(accumulated)
	}
}

// --- Блок кода с кнопкой копирования ---

func makeCodeBlock(lang, code string) fyne.CanvasObject {
	// Entry readonly — поддерживает выделение и Cmd+C
	entry := widget.NewMultiLineEntry()
	entry.SetText(code)
	entry.TextStyle = fyne.TextStyle{Monospace: true}
	entry.Wrapping = fyne.TextWrapOff // горизонтальный скролл

	lines := strings.Count(code, "\n") + 1
	if lines < 1 {
		lines = 1
	}
	if lines > 20 {
		lines = 20
	}
	entry.SetMinRowsVisible(lines)

	// Кнопка копирования
	copyBtn := widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Driver().AllWindows()[0].Clipboard().SetContent(code)
	})
	copyBtn.Importance = widget.LowImportance

	// Заголовок: язык слева, кнопка справа
	var header fyne.CanvasObject
	if lang != "" {
		langLabel := widget.NewLabelWithStyle(lang,
			fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
		header = container.NewBorder(nil, nil, langLabel, copyBtn)
	} else {
		header = container.NewBorder(nil, nil, nil, copyBtn)
	}

	// Серый фон
	bg := canvas.NewRectangle(theme.InputBackgroundColor())
	bg.CornerRadius = 4

	inner := container.NewVBox(header, widget.NewSeparator(), entry)

	return container.NewStack(bg, container.NewPadded(inner))
}

// --- Парсер текста на блоки ---

type textBlock struct {
	content string
	lang    string
	isCode  bool
}

func parseBlocks(text string) []textBlock {
	var blocks []textBlock
	parts := strings.Split(text, "```")

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i%2 == 0 {
			// Обычный текст
			blocks = append(blocks, textBlock{content: part})
		} else {
			// Блок кода: первая строка = язык
			lang := ""
			code := part
			if idx := strings.Index(part, "\n"); idx != -1 {
				lang = strings.TrimSpace(part[:idx])
				code = strings.TrimRight(part[idx+1:], "\n")
			}
			blocks = append(blocks, textBlock{
				content: code,
				lang:    lang,
				isCode:  true,
			})
		}
	}
	return blocks
}