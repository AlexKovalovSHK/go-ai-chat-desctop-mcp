package fyne

import (
	"context"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/alexkovalov/gemini-chat/internal/keystore"
	"github.com/alexkovalov/gemini-chat/internal/provider"
	"github.com/alexkovalov/gemini-chat/internal/store"
)

type SettingsDialog struct {
	Window   fyne.Window
	Store    store.Store
	Provider provider.Provider
	OnSave   func()

	apiKeyEntry   *widget.Entry
	modelSelect   *widget.Select
	themeSelect   *widget.RadioGroup
	systemPrompt  *widget.Entry
	statusLabel   *widget.Label
	models        []provider.ModelInfo
	validateTimer *time.Timer
}

func ShowSettings(parent fyne.Window, s store.Store, p provider.Provider, onSave func()) {
	win := fyne.CurrentApp().NewWindow("Настройки")
	sd := &SettingsDialog{
		Window:   win,
		Store:    s,
		Provider: p,
		OnSave:   onSave,
	}

	sd.apiKeyEntry = widget.NewPasswordEntry()
	sd.apiKeyEntry.SetPlaceHolder("AIza...")
	if key, err := keystore.GetAPIKey(); err == nil {
		sd.apiKeyEntry.SetText(key)
	}

	sd.statusLabel = widget.NewLabel("")
	sd.apiKeyEntry.OnChanged = func(s string) {
		sd.debounceValidation(s)
	}

	deleteKeyBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if err := keystore.DeleteAPIKey(); err == nil {
			sd.apiKeyEntry.SetText("")
			sd.statusLabel.SetText("Ключ удален")
			sd.models = nil
			sd.modelSelect.Options = nil
			sd.modelSelect.Refresh()
		}
	})
	deleteKeyBtn.Importance = widget.DangerImportance

	sd.modelSelect = widget.NewSelect([]string{}, nil)
	sd.modelSelect.PlaceHolder = "Выберите модель"

	sd.systemPrompt = widget.NewMultiLineEntry()
	sd.systemPrompt.SetPlaceHolder("Вы — полезный помощник...")

	if m, _ := s.GetConfig("model_id"); m != "" {
		sd.modelSelect.SetSelected(m)
	}
	if sp, _ := s.GetConfig("system_prompt"); sp != "" {
		sd.systemPrompt.SetText(sp)
	}

	sd.themeSelect = widget.NewRadioGroup([]string{"System", "Dark", "Light"}, nil)
	sd.themeSelect.Horizontal = true
	if t, _ := s.GetConfig("theme"); t != "" {
		sd.themeSelect.SetSelected(t)
	} else {
		sd.themeSelect.SetSelected("System")
	}

	apiKeyContainer := container.NewBorder(nil, nil, nil, deleteKeyBtn, sd.apiKeyEntry)

	form := widget.NewForm(
		widget.NewFormItem("API Ключ", apiKeyContainer),
		widget.NewFormItem("", sd.statusLabel),
		widget.NewFormItem("Модель", sd.modelSelect),
		widget.NewFormItem("Инструкции", sd.systemPrompt),
		widget.NewFormItem("Тема", sd.themeSelect),
	)

	saveBtn := widget.NewButtonWithIcon("Сохранить", theme.ConfirmIcon(), func() {
		keystore.SaveAPIKey(sd.apiKeyEntry.Text)
		s.SetConfig("model_id", sd.modelSelect.Selected)
		s.SetConfig("system_prompt", sd.systemPrompt.Text)
		s.SetConfig("theme", sd.themeSelect.Selected)
		
		sd.applyTheme(sd.themeSelect.Selected)
		
		if onSave != nil {
			onSave()
		}
		win.Close()
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Отмена", func() { win.Close() })

	win.SetContent(container.NewBorder(nil, container.NewHBox(cancelBtn, saveBtn), nil, nil, container.NewPadded(form)))
	win.Resize(fyne.NewSize(500, 400))
	win.CenterOnScreen()
	win.Show()

	// Initial validation if key exists (run in goroutine)
	if sd.apiKeyEntry.Text != "" {
		go sd.validateKey(sd.apiKeyEntry.Text)
	}
}

func (sd *SettingsDialog) debounceValidation(key string) {
	if sd.validateTimer != nil {
		sd.validateTimer.Stop()
	}
	sd.statusLabel.SetText("Проверка...")
	sd.validateTimer = time.AfterFunc(800*time.Millisecond, func() {
		// validateKey itself will handle Do wrapping for its internal UI calls
		sd.validateKey(key)
	})
}

func (sd *SettingsDialog) validateKey(key string) {
	if key == "" {
		fyne.Do(func() {
			sd.statusLabel.SetText("Введите ключ")
			sd.models = nil
			sd.modelSelect.Options = nil
			sd.modelSelect.Refresh()
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := sd.Provider.ValidateKey(ctx, key)
	if err != nil {
		fyne.Do(func() {
			sd.statusLabel.SetText("❌ Ошибка: " + err.Error())
		})
		return
	}

	fyne.Do(func() {
		sd.statusLabel.SetText("✅ Ключ валиден")
	})
	
	// Fetch models
	models, err := sd.Provider.ListModels(ctx, key)
	if err != nil {
		return
	}

	fyne.Do(func() {
		log.Printf("Settings: updating model select with %d models", len(models))
		sd.models = models
		var options []string
		for _, m := range models {
			options = append(options, m.ID)
		}
		sd.modelSelect.Options = options
		sd.modelSelect.Refresh()
	})
}

func (sd *SettingsDialog) applyTheme(t string) {
	// Re-applying theme. DefaultTheme() automatically follows Fyne settings
	// which we can't fully control without a custom theme wrapper,
	// but calling Refresh on the app is a good practice.
	fyne.CurrentApp().Settings().SetTheme(theme.DefaultTheme())
}
