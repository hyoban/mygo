//go:build darwin

package darwin

import (
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A control (platform.ControlSurface) is an NSButton check box, an
// NSPopUpButton or an NSTextField in a clip view under the surface
// (embed.go), its target and the field's delegate MyGoControlTarget,
// which tells the handler what the user changed.
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

func (s *surface) NewControl(kind platform.ControlKind, h platform.ControlHandler) platform.Control {
	c := &control{h: h, kind: kind}
	withPool(func() {
		s.newEmbed(c)
		c.target = alloc("MyGoControlTarget")
		frame := NSRect{Size: NSSize{Width: 100, Height: 22}}
		switch kind {
		case platform.ControlCheckbox:
			c.view = msgInitRect(send(class("NSButton"), "alloc"), sel("initWithFrame:"), frame)
			send(c.view, "setButtonType:", nsButtonTypeSwitch)
			send(c.view, "setTitle:", uintptr(nsString("")))
		case platform.ControlPopUp:
			c.view = msgInitRect(send(class("NSPopUpButton"), "alloc"), sel("initWithFrame:"), frame)
		case platform.ControlTextField:
			c.view = msgInitRect(send(class("NSTextField"), "alloc"), sel("initWithFrame:"), frame)
			send(c.view, "setDelegate:", uintptr(c.target))
			send(c.view, "setUsesSingleLineMode:", 1)
			send(send(c.view, "cell"), "setScrollable:", 1)
		}
		send(c.view, "setTarget:", uintptr(c.target))
		send(c.view, "setAction:", uintptr(sel("mygoControlChanged:")))
		send(c.clip, "addSubview:", uintptr(c.view))
		c.focusView = c.view
	})
	controls[c.view], controls[c.target] = c, c
	return c
}

// placed fills frame with the control, which the content sizes as the
// control fits (FittingSize) unless given a width.
func (c *control) placed(frame NSRect) {
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
	withPool(func() { send(c.view, "setAccessibilityLabel:", uintptr(nsString(label))) })
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
		if c.kind == platform.ControlTextField {
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
	classDef("MyGoControlTarget", "NSObject", []string{"NSTextFieldDelegate"}, []objc.MethodDef{
		method("mygoControlChanged:", func(self id, _ objc.SEL, sender id) {
			if c := control(self); c != nil && c.kind != platform.ControlTextField {
				c.h.Changed()
			}
		}),
		method("controlTextDidChange:", func(self id, _ objc.SEL, note id) {
			if c := control(self); c != nil {
				c.h.Changed()
			}
		}),
		// Tab leaves a field for the content, which moves the keyboard on.
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
			}
			return false
		}),
	})
}
