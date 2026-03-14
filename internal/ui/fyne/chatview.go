package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

type ChatView struct {
	Container *fyne.Container
	Scroll    *container.Scroll
	Content   *fyne.Container
}

func NewChatView() *ChatView {
	content := container.NewVBox()
	scroll := container.NewVScroll(content)

	return &ChatView{
		Container: container.NewMax(scroll),
		Scroll:    scroll,
		Content:   content,
	}
}

func (v *ChatView) AddBubble(bubble *ChatBubble) {
	v.Content.Add(bubble.Container)
	v.ScrollToBottom()
}

func (v *ChatView) ScrollToBottom() {
	v.Scroll.ScrollToBottom()
}

func (v *ChatView) Clear() {
	v.Content.Objects = nil
	v.Content.Refresh()
}
