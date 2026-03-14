package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type InputBar struct {
	Container *fyne.Container
	Entry     *widget.Entry
	SendBtn   *widget.Button
	StopBtn   *widget.Button
	OnSend    func(string)
	OnStop    func()
}

type CustomEntry struct {
	widget.Entry
	shiftPressed bool
	OnSubmit     func()
}

func (e *CustomEntry) KeyDown(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shiftPressed = true
	}
	e.Entry.KeyDown(k)
}

func (e *CustomEntry) KeyUp(k *fyne.KeyEvent) {
	if k.Name == desktop.KeyShiftLeft || k.Name == desktop.KeyShiftRight {
		e.shiftPressed = false
	}
	e.Entry.KeyUp(k)
}

func (e *CustomEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyReturn || k.Name == fyne.KeyEnter {
		if e.shiftPressed {
			e.Entry.TypedKey(k)
			return
		}
		if e.OnSubmit != nil {
			e.OnSubmit()
		}
		return
	}
	e.Entry.TypedKey(k)
}

func NewInputBar() *InputBar {
	entry := &CustomEntry{}
	entry.MultiLine = true
	entry.ExtendBaseWidget(entry)
	entry.SetPlaceHolder("Введите сообщение...")
	entry.Wrapping = fyne.TextWrapWord

	ib := &InputBar{
		Entry: &entry.Entry,
	}

	submit := func() {
		if ib.OnSend != nil {
			ib.OnSend(ib.Entry.Text)
		}
	}
	entry.OnSubmit = submit

	ib.SendBtn = widget.NewButtonWithIcon("", theme.MailSendIcon(), submit)
	ib.SendBtn.Importance = widget.HighImportance

	ib.StopBtn = widget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		if ib.OnStop != nil {
			ib.OnStop()
		}
	})
	ib.StopBtn.Hide()

	ib.Container = container.NewBorder(nil, nil, nil, container.NewHBox(ib.StopBtn, ib.SendBtn), entry)

	return ib
}

func (ib *InputBar) SetLoading(loading bool) {
	if loading {
		ib.SendBtn.Hide()
		ib.StopBtn.Show()
		ib.Entry.Disable()
	} else {
		ib.SendBtn.Show()
		ib.StopBtn.Hide()
		ib.Entry.Enable()
	}
}

func (ib *InputBar) Clear() {
	ib.Entry.SetText("")
}
