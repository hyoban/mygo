//go:build darwin

package darwin

import (
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A control (platform.ControlSurface) is an NSButton check box, an
// NSPopUpButton, an NSTextField or an NSSearchField in a clip view under
// the surface (embed.go), of MyGo's subclass of each, which tells the
// content of a press on it and puts it in the element showing it for
// assistive technology. Its target, and a field's delegate, is
// MyGoControlTarget, which tells the handler what the user changed and
// the keys a field passes on.
type control struct {
	embed
	h      platform.ControlHandler
	kind   platform.ControlKind
	view   id
	target id
	// items are a pop-up button's, as shown last.
	items []string
}

var controls = map[id]*control{} // by view and by target

const nsButtonTypeSwitch = 3

// controlClasses are MyGo's subclasses of the controls, by kind.
var controlClasses = [...]struct{ name, super string }{
	platform.ControlCheckbox:    {"MyGoCheckbox", "NSButton"},
	platform.ControlPopUp:       {"MyGoPopUpButton", "NSPopUpButton"},
	platform.ControlTextField:   {"MyGoTextField", "NSTextField"},
	platform.ControlSearchField: {"MyGoSearchField", "NSSearchField"},
}

func (s *surface) NewControl(kind platform.ControlKind, h platform.ControlHandler) platform.Control {
	c := &control{h: h, kind: kind}
	withPool(func() {
		s.newEmbed(c)
		c.target = alloc("MyGoControlTarget")
		frame := NSRect{Size: NSSize{Width: 100, Height: 22}}
		c.view = msgInitRect(send(class(controlClasses[kind].name), "alloc"), sel("initWithFrame:"), frame)
		switch kind {
		case platform.ControlCheckbox:
			send(c.view, "setButtonType:", nsButtonTypeSwitch)
			send(c.view, "setTitle:", uintptr(nsString("")))
		case platform.ControlTextField:
			send(c.view, "setUsesSingleLineMode:", 1)
			send(send(c.view, "cell"), "setScrollable:", 1)
		case platform.ControlSearchField:
			send(c.view, "setSendsSearchStringImmediately:", 1)
			send(c.view, "setSendsWholeSearchString:", 0)
		}
		if kind.IsField() {
			send(c.view, "setDelegate:", uintptr(c.target))
		}
		send(c.view, "setTarget:", uintptr(c.target))
		send(c.view, "setAction:", uintptr(sel("mygoControlChanged:")))
		send(c.clip, "addSubview:", uintptr(c.view))
		c.focusView = c.view
	})
	controls[c.view], controls[c.target] = c, c
	return c
}

// placed puts the control in frame, at its own height in the middle of a
// frame higher than that. The content sizes it as it fits (FittingSize)
// unless given a size.
func (c *control) placed(frame NSRect) {
	if h := msgSize(c.view, sel("fittingSize")).Height; h < frame.Size.Height {
		frame.Origin.Y += (frame.Size.Height - h) / 2
		frame.Size.Height = h
	}
	if msgRect(c.view, sel("frame")) != frame {
		msgSetRect(c.view, sel("setFrame:"), frame)
	}
}

func (c *control) FittingSize() (float64, float64) {
	if c.closed {
		return 0, 0
	}
	var s NSSize
	withPool(func() { s = msgSize(c.view, sel("fittingSize")) })
	return s.Width, s.Height
}

func (c *control) SetCheckbox(title string, checked bool) {
	if c.closed {
		return
	}
	withPool(func() {
		if goString(send(c.view, "title")) != title {
			send(c.view, "setTitle:", uintptr(nsString(title)))
		}
		state := 0
		if checked {
			state = 1
		}
		if sendInt(c.view, "state") != state {
			send(c.view, "setState:", uintptr(state))
		}
	})
}

func (c *control) SetItems(items []string, selected int) {
	if c.closed {
		return
	}
	withPool(func() {
		if !slices.Equal(c.items, items) {
			c.items = slices.Clone(items)
			send(c.view, "removeAllItems")
			var titles []id
			for _, it := range items {
				titles = append(titles, nsString(it))
			}
			send(c.view, "addItemsWithTitles:", uintptr(nsArray(titles...)))
		}
		if sendInt(c.view, "indexOfSelectedItem") != selected {
			send(c.view, "selectItemAtIndex:", uintptr(selected))
		}
	})
}

func (c *control) SetText(text, placeholder string) {
	if c.closed {
		return
	}
	withPool(func() {
		if goString(send(c.view, "stringValue")) != text {
			send(c.view, "setStringValue:", uintptr(nsString(text)))
		}
		if goString(send(c.view, "placeholderString")) != placeholder {
			send(c.view, "setPlaceholderString:", uintptr(nsString(placeholder)))
		}
	})
}

func (c *control) SetEnabled(enabled bool) {
	if c.closed || sendBool(c.view, "isEnabled") == enabled {
		return
	}
	withPool(func() { send(c.view, "setEnabled:", boolArg(enabled)) })
}

func (c *control) SetLabel(label string) {
	if c.closed {
		return
	}
	withPool(func() {
		if goString(send(c.view, "accessibilityLabel")) != label {
			send(c.view, "setAccessibilityLabel:", uintptr(nsString(label)))
		}
	})
}

func (c *control) Checked() bool { return !c.closed && sendInt(c.view, "state") == 1 }

func (c *control) Selected() int {
	if c.closed {
		return -1
	}
	return sendInt(c.view, "indexOfSelectedItem")
}

func (c *control) Text() string {
	if c.closed {
		return ""
	}
	var s string
	withPool(func() { s = goString(send(c.view, "stringValue")) })
	return s
}

func (c *control) Focus()            { c.focus() }
func (c *control) TabInto(back bool) { c.focus() }
func (c *control) HasFocus() bool    { return c.hasFocus() }

func (c *control) Close() {
	if c.closed {
		return
	}
	withPool(func() {
		c.closeEmbed(c)
		if c.kind.IsField() {
			send(c.view, "setDelegate:", 0)
		}
		send(c.view, "setTarget:", 0)
		delete(controls, c.view)
		delete(controls, c.target)
		release(c.view)
		autorelease(c.target)
	})
}

func registerControlClasses() {
	control := func(self id) *control {
		if c := controls[self]; c != nil && !c.closed {
			return c
		}
		return nil
	}
	for kind, cl := range controlClasses {
		methods := []objc.MethodDef{
			// A press on the control tells the content, as one on a web
			// view does, which closes its popups, say.
			method("mouseDown:", func(self id, cmd objc.SEL, ev id) {
				if c := control(self); c != nil {
					c.pressed(ev, 0)
				}
				sendSuper(self, cl.name, cmd, uintptr(ev))
			}),
			// Assistive technology finds the control inside the element
			// showing it (access.go), not in its clip view.
			method("accessibilityParent", func(self id, cmd objc.SEL) id {
				if c := control(self); c != nil && c.accessParent != 0 {
					return c.accessParent
				}
				return sendSuper(self, cl.name, cmd)
			}),
		}
		if platform.ControlKind(kind) == platform.ControlSearchField {
			// A click on a search field of a window in the background
			// goes to it, as one on a sidebar's list does.
			methods = append(methods, method("acceptsFirstMouse:", func(self id, _ objc.SEL, ev id) bool { return true }))
		}
		classDef(cl.name, cl.super, nil, methods)
	}
	classDef("MyGoControlTarget", "NSObject", []string{"NSSearchFieldDelegate"}, []objc.MethodDef{
		method("mygoControlChanged:", func(self id, _ objc.SEL, sender id) {
			// A text field tells each edit as controlTextDidChange:, and
			// its action only as it ends editing; a search field's action
			// tells its button clearing it too.
			if c := control(self); c != nil && c.kind != platform.ControlTextField {
				c.h.Changed()
			}
		}),
		method("controlTextDidChange:", func(self id, _ objc.SEL, note id) {
			if c := control(self); c != nil {
				c.h.Changed()
			}
		}),
		// The keys a field has no use for go to the content, as moving to
		// the list a search field filters; Tab leaves the field for it.
		method("control:textView:doCommandBySelector:", func(self id, _ objc.SEL, ctl, view id, command objc.SEL) bool {
			c := control(self)
			if c == nil {
				return false
			}
			switch command {
			case sel("insertTab:"), sel("insertBacktab:"):
				c.leave()
				c.h.TabOut(command == sel("insertBacktab:"))
				return true
			case sel("moveDown:"):
				return c.h.Key(platform.KeyDown)
			case sel("moveUp:"):
				return c.h.Key(platform.KeyUp)
			case sel("insertNewline:"):
				return c.h.Key(platform.KeyEnter)
			case sel("cancelOperation:"):
				if c.kind == platform.ControlSearchField && goString(send(c.view, "stringValue")) != "" {
					send(c.view, "setStringValue:", uintptr(nsString("")))
					c.h.Changed()
					return true
				}
				return c.h.Key(platform.KeyEscape)
			}
			return false
		}),
	})
}
