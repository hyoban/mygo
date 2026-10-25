package ui

import "github.com/egoist/mygo/internal/platform"

// textServices gives the system's services of text the engine's: those
// of macOS's Services menu and Writing Tools.
type textServices struct{ rt *engine }

// ServicedText returns the selection of the element with the keyboard
// focus: of a text input, or of a text input client, which they may
// replace, or of an element's TextServices; or else that of selectable
// text.
func (t textServices) ServicedText() (string, bool) {
	rt := t.rt
	if s := rt.editable(); s != nil {
		if s.editor.password {
			return "", false
		}
		return s.editor.selectedText(), true
	}
	if c := rt.focusedClient(); c != nil {
		text, _ := c.TextForRange(c.Selection().Range)
		return text, true
	}
	if s := rt.states[rt.focused]; s != nil && s.servicedText != nil {
		return s.servicedText(), false
	}
	return rt.selectionText(), false
}

// ReplaceServicedText puts what a service gave in place of the selection
// of the text input or the text input client with the keyboard focus, as
// if typed.
func (t textServices) ReplaceServicedText(text string) {
	rt := t.rt
	switch c := rt.focusedClient(); {
	case rt.editable() != nil:
		rt.editEvent(editEvent{kind: editInsert, text: text})
	case c != nil:
		r := c.Selection().Range
		c.ReplaceText(&r, text)
	default:
		return
	}
	rt.updateTextInput()
	rt.requestFrame()
}

// focusedClient returns the TextInputClient of the element with the
// keyboard focus, which takes text, or nil.
func (rt *engine) focusedClient() TextInputClient {
	s := rt.states[rt.focused]
	if s == nil || s.editor != nil || s.textClient == nil || s.flags&flagDisabled != 0 {
		return nil
	}
	return s.textClient
}

// editable returns the text input with the keyboard focus, which takes
// text, or nil.
func (rt *engine) editable() *state {
	s := rt.states[rt.focused]
	if s == nil || s.editor == nil || s.editor.readOnly || s.flags&flagDisabled != 0 {
		return nil
	}
	return s
}

var _ platform.TextServices = textServices{}
