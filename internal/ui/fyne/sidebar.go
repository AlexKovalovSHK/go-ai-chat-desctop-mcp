package fyne

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/alexkovalov/gemini-chat/internal/models"
)

type Sidebar struct {
	Container  *fyne.Container
	List       *widget.List
	Chats      []models.ChatMeta
	OnSelect   func(string)
	OnNew      func()
	OnDelete   func(string)
	OnSettings func()
	SelectedID string
}

func NewSidebar() *Sidebar {
	s := &Sidebar{}

	s.List = widget.NewList(
		func() int { return len(s.Chats) },
		func() fyne.CanvasObject {
			icon := widget.NewLabel("✦")
			model := widget.NewLabel("model")
			model.TextStyle = fyne.TextStyle{Italic: true}
			title := widget.NewLabel("title")
			return container.NewVBox(
				container.NewHBox(icon, model),
				title,
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			hbox := box.Objects[0].(*fyne.Container)
			icon := hbox.Objects[0].(*widget.Label)
			model := hbox.Objects[1].(*widget.Label)
			title := box.Objects[1].(*widget.Label)

			c := s.Chats[i]
			icon.SetText(s.getProviderIcon(c.ProviderID))
			model.SetText(s.getShortModel(c.ModelID))
			title.SetText(c.Title)
		},
	)

	s.List.OnSelected = func(id widget.ListItemID) {
		s.SelectedID = s.Chats[id].ID
		if s.OnSelect != nil {
			s.OnSelect(s.SelectedID)
		}
	}

	newBtn := widget.NewButtonWithIcon("Новый чат", theme.ContentAddIcon(), func() {
		if s.OnNew != nil {
			s.OnNew()
		}
	})
	newBtn.Importance = widget.HighImportance

	deleteBtn := widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), func() {
		if s.SelectedID != "" && s.OnDelete != nil {
			s.OnDelete(s.SelectedID)
		}
	})

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		if s.OnSettings != nil {
			s.OnSettings()
		}
	})

	s.Container = container.NewBorder(newBtn, container.NewHBox(settingsBtn, deleteBtn), nil, nil, s.List)

	return s
}

func (s *Sidebar) UpdateChats(chats []models.ChatMeta) {
	s.Chats = chats
	s.List.Refresh()
}

func (s *Sidebar) getProviderIcon(providerID string) string {
	switch providerID {
	case "gemini":
		return "✦"
	default:
		return "🤖"
	}
}

func (s *Sidebar) getShortModel(modelID string) string {
	name := modelID
	name = strings.TrimPrefix(name, "gemini-")
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}
