package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/alexkovalov/gemini-chat/internal/keystore"
	"github.com/alexkovalov/gemini-chat/internal/models"
	"github.com/alexkovalov/gemini-chat/internal/provider"
	"github.com/alexkovalov/gemini-chat/internal/provider/gemini"
	"github.com/alexkovalov/gemini-chat/internal/store"
	ui "github.com/alexkovalov/gemini-chat/internal/ui/fyne"
	"github.com/google/uuid"
)

func main() {
	myApp := app.NewWithID("com.alexkovalov.geminichat")
	myWindow := myApp.NewWindow("GeminiChat")

	// Storage
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".config", "gemini-chat", "history.db")
	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Dependencies
	geminiProvider := gemini.New()

	// UI Components
	chatView := ui.NewChatView()
	inputBar := ui.NewInputBar()
	sidebar := ui.NewSidebar()

	var (
		currentChatID string
		cancel        context.CancelFunc
	)

	// Layout and State
	mainContent := container.NewBorder(nil, inputBar.Container, nil, nil, chatView.Container)
	
	// Empty state placeholder
	welcomeLabel := widget.NewLabelWithStyle("Добро пожаловать в GeminiChat!", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	welcomeSub := widget.NewLabelWithStyle("Начните новый чат или выберите существующий из списка слева.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	emptyState := container.NewCenter(container.NewVBox(welcomeLabel, welcomeSub))
	
	mainStack := container.NewStack(mainContent, emptyState)

	refreshSidebar := func() {
		chats, err := db.ListChats()
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		sidebar.UpdateChats(chats)
	}

	loadChat := func(chatID string) {
		if chatID == "" {
			emptyState.Show()
			mainContent.Hide()
			return
		}
		emptyState.Hide()
		mainContent.Show()

		currentChatID = chatID
		chatView.Clear()
		msgs, err := db.LoadMessages(chatID)
		if err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		for _, m := range msgs {
			chatView.AddBubble(ui.NewBubble(m.Role, m.Content))
		}
		chatView.ScrollToBottom()
	}

	createNewChat := func() {
		newID := uuid.New().String()
		modelID, _ := db.GetConfig("model_id")
		if modelID == "" {
			modelID = "gemini-2.0-flash"
		}

		chat := models.ChatSession{
			ID:         newID,
			Title:      "Новый чат",
			ProviderID: "gemini",
			ModelID:    modelID,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := db.CreateChat(chat); err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		refreshSidebar()
		loadChat(newID)
	}

	// Keyboard Shortcuts
	myWindow.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyN,
		Modifier: fyne.KeyModifierShortcutDefault,
	}, func(shortcut fyne.Shortcut) {
		createNewChat()
	})

	myWindow.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyComma,
		Modifier: fyne.KeyModifierShortcutDefault,
	}, func(shortcut fyne.Shortcut) {
		sidebar.OnSettings()
	})

	sidebar.OnSelect = loadChat
	sidebar.OnNew = createNewChat
	sidebar.OnDelete = func(id string) {
		dialog.ShowConfirm("Удалить чат?", "Вы уверены, что хотите удалить этот чат и всю его историю?", func(ok bool) {
			if ok {
				if err := db.DeleteChat(id); err != nil {
					dialog.ShowError(err, myWindow)
				} else {
					if currentChatID == id {
						currentChatID = ""
						loadChat("")
					}
					refreshSidebar()
				}
			}
		}, myWindow)
	}

	sidebar.OnSettings = func() {
		ui.ShowSettings(myWindow, db, geminiProvider, func() {
			// Callback on save
		})
	}

	inputBar.OnSend = func(text string) {
		text = strings.TrimSpace(text)
		if text == "" || currentChatID == "" {
			return
		}

		apiKey, err := keystore.GetAPIKey()
		if err != nil || apiKey == "" {
			dialog.ShowInformation("Настройки", "Пожалуйста, укажите API Ключ в настройках", myWindow)
			sidebar.OnSettings()
			return
		}

		userMsg := models.Message{
			ChatID:    currentChatID,
			Role:      models.RoleUser,
			Content:   text,
			CreatedAt: time.Now(),
		}
		if err := db.AddMessage(userMsg); err != nil {
			dialog.ShowError(err, myWindow)
			return
		}
		chatView.AddBubble(ui.NewBubble(userMsg.Role, userMsg.Content))
		inputBar.Clear()

		msgs, _ := db.LoadMessages(currentChatID)
		if len(msgs) == 1 {
			title := text
			if len(title) > 45 {
				title = title[:42] + "..."
			}
			db.UpdateChatTitle(currentChatID, title)
			refreshSidebar()
		}

		inputBar.SetLoading(true)
		ctx, cancelCtx := context.WithCancel(context.Background())
		cancel = cancelCtx

		assistantBubble := ui.NewBubble(models.RoleAssistant, "...")
		chatView.AddBubble(assistantBubble)

		go func() {
			defer inputBar.SetLoading(false)
			defer cancelCtx()

			modelID, _ := db.GetConfig("model_id")
			if modelID == "" {
				modelID = "gemini-2.0-flash"
			}
			systemPrompt, _ := db.GetConfig("system_prompt")

			history, _ := db.LoadMessages(currentChatID)
			req := provider.StreamRequest{
				APIKey:       apiKey,
				ModelID:      modelID,
				SystemPrompt: systemPrompt,
				Messages:     history,
			}

			textCh, errCh := geminiProvider.Stream(ctx, req)
			var fullText strings.Builder
			for {
				select {
				case <-ctx.Done():
					fyne.Do(func() {
						assistantBubble.SetContent(fullText.String())
					})
					return
				case err := <-errCh:
					if err != nil {
						fyne.Do(func() {
							dialog.ShowError(err, myWindow)
						})
					}
					content := fullText.String()
					if content != "" {
						asstMsg := models.Message{
							ChatID:    currentChatID,
							Role:      models.RoleAssistant,
							Content:   content,
							CreatedAt: time.Now(),
						}
						db.AddMessage(asstMsg)
						fyne.Do(func() {
							assistantBubble.SetContent(content)
							refreshSidebar()
						})
					} else {
						fyne.Do(func() {
							assistantBubble.SetContent("Ошибка запроса")
						})
					}
					return
				case chunk, ok := <-textCh:
					if !ok {
						content := fullText.String()
						if content != "" {
							asstMsg := models.Message{
								ChatID:    currentChatID,
								Role:      models.RoleAssistant,
								Content:   content,
								CreatedAt: time.Now(),
							}
							db.AddMessage(asstMsg)
						}
						fyne.Do(func() {
							assistantBubble.SetContent(content)
							refreshSidebar()
						})
						return
					}
					fullText.WriteString(chunk)
					fyne.Do(func() {
						assistantBubble.SetStreaming(true, fullText.String())
						chatView.ScrollToBottom()
					})
				}
			}
		}()
	}

	inputBar.OnStop = func() {
		if cancel != nil {
			cancel()
		}
	}

	// Initial State
	refreshSidebar()
	if len(sidebar.Chats) > 0 {
		loadChat(sidebar.Chats[0].ID)
	} else {
		loadChat("") // Show empty state
	}

	if key, _ := keystore.GetAPIKey(); key == "" {
		sidebar.OnSettings()
	}

	split := container.NewHSplit(sidebar.Container, mainStack)
	split.Offset = 0.25

	myWindow.SetContent(split)
	myWindow.Resize(fyne.NewSize(1000, 700))
	myWindow.ShowAndRun()
}
