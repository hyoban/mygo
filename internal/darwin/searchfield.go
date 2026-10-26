//go:build darwin

package darwin

import (
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A search field (platform.SearchFieldSurface) is an NSSearchField in a
// clip view under the surface (embed.go), its delegate and target
// MyGoSearchFieldDelegate, which tells the handler what is typed and the
// keys a field passes on: those that move to what the field searches.
type searchField struct {
	embed
	h        platform.SearchFieldHandler
	field    id // MyGoSearchField
	delegate id
	// height is the field's own, which the content gives it.
	height float64
}

var searchFields = map[id]*searchField{} // by field and by delegate

func (s *surface) NewSearchField(h platform.SearchFieldHandler) platform.SearchField {
	f := &searchField{h: h}
	withPool(func() {
		s.newEmbed(f)
		f.field = msgInitRect(send(class("MyGoSearchField"), "alloc"), sel("initWithFrame:"), NSRect{Size: NSSize{Width: 200, Height: 22}})
		send(f.field, "setSendsSearchStringImmediately:", 1)
		send(f.field, "setSendsWholeSearchString:", 0)
		f.delegate = alloc("MyGoSearchFieldDelegate")
		send(f.field, "setDelegate:", uintptr(f.delegate))
		send(f.field, "setTarget:", uintptr(f.delegate))
		send(f.field, "setAction:", uintptr(sel("mygoSearchChanged:")))
		send(f.clip, "addSubview:", uintptr(f.field))
		f.height = msgSize(f.field, sel("fittingSize")).Height
		f.focusView = f.field
	})
	searchFields[f.field], searchFields[f.delegate] = f, f
	return f
}

// placed puts the field in the middle of frame, at its own height.
func (f *searchField) placed(frame NSRect) {
	h := min(f.height, frame.Size.Height)
	r := NSRect{Origin: NSPoint{frame.Origin.X, frame.Origin.Y + (frame.Size.Height-h)/2}, Size: NSSize{frame.Size.Width, h}}
	if msgRect(f.field, sel("frame")) != r {
		msgSetRect(f.field, sel("setFrame:"), r)
	}
}

func (f *searchField) Height() float64 { return f.height }

func (f *searchField) SetText(text string) {
	if f.closed || goString(send(f.field, "stringValue")) == text {
		return
	}
	withPool(func() { send(f.field, "setStringValue:", uintptr(nsString(text))) })
}

func (f *searchField) SetPlaceholder(text, label string) {
	if f.closed {
		return
	}
	withPool(func() {
		if goString(send(f.field, "placeholderString")) != text {
			send(f.field, "setPlaceholderString:", uintptr(nsString(text)))
		}
		send(f.field, "setAccessibilityLabel:", uintptr(nsString(label)))
	})
}

func (f *searchField) Focus()            { f.focus() }
func (f *searchField) TabInto(back bool) { f.focus() }
func (f *searchField) HasFocus() bool    { return f.hasFocus() }

func (f *searchField) Close() {
	if f.closed {
		return
	}
	withPool(func() {
		f.closeEmbed(f)
		send(f.field, "setDelegate:", 0)
		send(f.field, "setTarget:", 0)
		delete(searchFields, f.field)
		delete(searchFields, f.delegate)
		release(f.field)
		autorelease(f.delegate)
	})
}

// changed tells the handler what the field shows.
func (f *searchField) changed() {
	f.h.Changed(goString(send(f.field, "stringValue")))
}

func registerSearchFieldClasses() {
	field := func(self id) *searchField {
		if f := searchFields[self]; f != nil && !f.closed {
			return f
		}
		return nil
	}
	classDef("MyGoSearchField", "NSSearchField", nil, []objc.MethodDef{
		method("mouseDown:", func(self id, cmd objc.SEL, ev id) {
			if f := field(self); f != nil {
				f.pressed(ev, 0)
			}
			sendSuper(self, "MyGoSearchField", cmd, uintptr(ev))
		}),
		method("acceptsFirstMouse:", func(self id, _ objc.SEL, ev id) bool { return true }),
		method("accessibilityParent", func(self id, cmd objc.SEL) id {
			if f := field(self); f != nil && f.accessParent != 0 {
				return f.accessParent
			}
			return sendSuper(self, "MyGoSearchField", cmd)
		}),
	})
	classDef("MyGoSearchFieldDelegate", "NSObject", []string{"NSSearchFieldDelegate"}, []objc.MethodDef{
		method("controlTextDidChange:", func(self id, _ objc.SEL, note id) {
			if f := field(self); f != nil {
				f.changed()
			}
		}),
		method("mygoSearchChanged:", func(self id, _ objc.SEL, sender id) {
			if f := field(self); f != nil {
				f.changed()
			}
		}),
		// The keys a field has no use for go to the content, as moving to
		// the list the field filters; Tab leaves the field for it.
		method("control:textView:doCommandBySelector:", func(self id, _ objc.SEL, control, view id, command objc.SEL) bool {
			f := field(self)
			if f == nil {
				return false
			}
			switch command {
			case sel("insertTab:"), sel("insertBacktab:"):
				f.leave()
				f.h.TabOut(command == sel("insertBacktab:"))
				return true
			case sel("moveDown:"):
				return f.h.Key(platform.KeyDown)
			case sel("moveUp:"):
				return f.h.Key(platform.KeyUp)
			case sel("insertNewline:"):
				return f.h.Key(platform.KeyEnter)
			case sel("cancelOperation:"):
				if goString(send(f.field, "stringValue")) != "" {
					send(f.field, "setStringValue:", uintptr(nsString("")))
					f.changed()
					return true
				}
				return f.h.Key(platform.KeyEscape)
			}
			return false
		}),
	})
}
