package ui

import "github.com/egoist/mygo/internal/platform"

// NativeCheckbox creates a check box toggling *checked, with its title,
// as the system draws one: AppKit's NSButton on macOS, at its own size.
// Elsewhere, and without a window, it is a Checkbox. Changed reports a
// click.
//
//	ui.NativeCheckbox(c, &app.sync, "Sync across devices")
func coreNativeCheckbox(c *context, checked *bool, title string) *node {
	e, b := nativeControlNode(c, platform.ControlCheckbox)
	if e == nil {
		return coreCheckbox(c, checked, title)
	}
	b.checked = checked
	b.n.SetCheckbox(title, *checked)
	b.fit(e)
	e.label = title
	return e
}

// NativePopUpButton creates a pop-up button choosing one of options into
// *selected, as the system draws one: AppKit's NSPopUpButton on macOS, at
// its own size unless given a width. Elsewhere, and without a window, it
// is a Select. Changed reports a new choice.
//
//	ui.NativePopUpButton(c, &app.plan, []string{"Free", "Pro", "Team"})
func coreNativePopUpButton(c *context, selected *string, options []string) *node {
	e, b := nativeControlNode(c, platform.ControlPopUp)
	if e == nil {
		return coreSelect(c, selected, options)
	}
	b.selected, b.options = selected, options
	at := -1
	for i, o := range options {
		if o == *selected {
			at = i
		}
	}
	b.n.SetItems(options, at)
	b.fit(e)
	return e
}

// NativeTextField creates a field of text editing *text, as the system
// draws one: AppKit's NSTextField on macOS, placeholder showing while it
// is empty, at its own height and as wide as it is given. Elsewhere, and
// without a window, it is a TextInput. Changed reports an edit.
//
//	ui.NativeTextField(c, &app.name, "Name").Width(220)
func coreNativeTextField(c *context, text *string, placeholder string) *node {
	e, b := nativeControlNode(c, platform.ControlTextField)
	if e == nil {
		return coreTextInput(c, text).Placeholder(placeholder)
	}
	b.text = text
	b.n.SetText(*text, placeholder)
	_, h := b.n.FittingSize()
	e.width, e.height = px(160), px(float32(h))
	e.label = placeholder
	return e
}

// nativeControlNode makes the element showing a control of the kind, and
// the control, kept in the element's state: nil, nil where the window has
// none.
func nativeControlNode(c *context, kind platform.ControlKind) (*node, *nativeControl) {
	surface := controlSurface(c)
	if surface == nil {
		return nil, nil
	}
	e := c.newElement(kindBox)
	b := e.st.control
	if b == nil || b.rt != c.rt || b.kind != kind {
		if b != nil {
			b.n.Close()
		}
		b = newNativeControl(c.rt, surface, kind)
		e.st.control = b
	}
	b.show(e)
	return e, b
}

// controlSurface returns the window's surface showing controls of the
// system's, nil for none.
func controlSurface(c *context) platform.ControlSurface {
	s, _ := c.rt.host.nativeViews().(platform.ControlSurface)
	return s
}

func newNativeControl(rt *engine, surface platform.ControlSurface, kind platform.ControlKind) *nativeControl {
	b := &nativeControl{rt: rt, kind: kind}
	b.n = surface.NewControl(kind, b)
	return b
}

// show makes e the element showing the control, changed if the user
// changed the value since the last frame.
func (b *nativeControl) show(e *node) {
	if b.changed {
		b.changed = false
		e.st.markChanged()
	}
	e.native = b
	e.flags |= flagFocusable | flagOwnRing
	b.id = e.id
	if !containsView(b.rt.lists, b) {
		b.rt.lists = append(b.rt.lists, b)
	}
}

// nativeControl is the system's control of a NativeCheckbox,
// NativePopUpButton, NativeTextField or NativeSearchField, and its
// handler.
type nativeControl struct {
	rt   *engine
	kind platform.ControlKind
	n    platform.Control
	// The values the control edits, as the last frame bound them.
	checked  *bool
	selected *string
	options  []string
	text     *string
	// onKey hears the keys a field passes on, nil for none.
	onKey func(Key) bool
	// changed tells that the user changed the value since the last frame.
	changed bool
	// id is the element showing the control, focused whether it had the
	// focus after the last frame.
	id      uint64
	focused bool
}

// fit gives the element the control's own size.
func (b *nativeControl) fit(e *node) {
	w, h := b.n.FittingSize()
	e.width, e.height = px(float32(w)), px(float32(h))
}

func (b *nativeControl) native() platform.NativeView { return b.n }

// sync gives the control the keyboard where the element took the focus,
// and enables it as the element is.
func (b *nativeControl) sync() {
	rt := b.rt
	focused := rt.focused == b.id
	if focused && !b.focused {
		b.n.Focus()
	}
	b.focused = focused
	if s := rt.states[b.id]; s != nil {
		b.n.SetEnabled(s.flags&flagDisabled == 0)
	}
}

func (b *nativeControl) Changed() {
	switch b.kind {
	case platform.ControlCheckbox:
		if b.checked != nil {
			*b.checked = b.n.Checked()
		}
	case platform.ControlPopUp:
		if i := b.n.Selected(); b.selected != nil && i >= 0 && i < len(b.options) {
			*b.selected = b.options[i]
		}
	case platform.ControlTextField, platform.ControlSearchField:
		if b.text != nil {
			*b.text = b.n.Text()
		}
	}
	b.changed = true
	b.rt.requestFrame()
}

func (b *nativeControl) Key(key platform.Key) bool {
	if b.onKey == nil {
		return false
	}
	var k Key
	switch key {
	case platform.KeyDown:
		k = KeyDown
	case platform.KeyUp:
		k = KeyUp
	case platform.KeyEnter:
		k = KeyEnter
	case platform.KeyEscape:
		k = KeyEscape
	default:
		return false
	}
	took := b.onKey(k)
	b.rt.requestFrame()
	return took
}

func (b *nativeControl) TabOut(back bool) {
	rt := b.rt
	rt.focused = b.id
	rt.moveFocus(back)
	b.focused = rt.focused == b.id
	rt.requestFrame()
}
