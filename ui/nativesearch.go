package ui

import "github.com/egoist/mygo/internal/platform"

// SearchFieldState is the state of a NativeSearchField, kept from frame
// to frame.
type SearchFieldState struct {
	// Handle gives the field the keyboard.
	Handle
	// Placeholder shows while the field is empty, Search without one;
	// Label names it to assistive technology.
	Placeholder, Label string
	// OnKey hears the keys a field has no use for, as the list it
	// filters takes them: KeyDown, KeyUp, KeyEnter, and KeyEscape while
	// the field is empty (with text, Escape clears it). It reports
	// whether it took the key.
	OnKey func(key Key) bool
	// native is the system's field.
	native any
}

// NativeSearchField creates a field for searching, editing *query, as the
// system draws one: AppKit's NSSearchField on macOS, with its magnifying
// glass, placeholder and button clearing it, which handles the keyboard
// while it has it, as the element does a web view's page (see WebView).
// It shows at its own height in the middle of the element, which takes
// that height unless given one. Elsewhere it is a field of MyGo's own
// alike. Changed reports a change of *query.
//
//	app.filter.Placeholder = "Filter"
//	app.filter.OnKey = func(k ui.Key) bool { return k == ui.KeyDown && app.files.Focus(c) }
//	ui.NativeSearchField(c, &app.filter, &app.query).FillWidth()
func coreNativeSearchField(c *context, s *SearchFieldState, query *string) *node {
	surface := controlSurface(c)
	if surface == nil {
		return drawnSearchField(c, s, query)
	}
	b, _ := s.native.(*nativeControl)
	if b == nil || b.rt != c.rt {
		b = newNativeControl(c.rt, surface, platform.ControlSearchField)
		s.native = b
	}
	b.text, b.onKey = query, s.OnKey
	label := s.Label
	if label == "" {
		label = s.Placeholder
	}
	placeholder := s.Placeholder
	if placeholder == "" {
		placeholder = c.texts().Search()
	}
	b.n.SetText(*query, placeholder)
	b.n.SetLabel(label)
	e := c.newElement(kindBox)
	_, h := b.n.FittingSize()
	e.height = px(float32(h))
	e.label = label
	wrapElement(e).Bind(&s.Handle)
	b.show(e)
	return e
}

// drawnSearchField builds a NativeSearchField of MyGo's own where the
// system has none.
func drawnSearchField(c *context, s *SearchFieldState, query *string) *node {
	t := c.theme
	placeholder := s.Placeholder
	if placeholder == "" {
		placeholder = c.texts().Search()
	}
	f, in := field(c, func() *node {
		magnifier(c)
		in := coreTextInputBase(c, query).Grow(1).MinWidth(t.Space(15)).Placeholder(placeholder)
		in.search = true
		in.OnTextKey(func(k TextKey) bool {
			switch {
			case k.Mods == 0 && k.Key == KeyEscape && *query != "":
				k.SetText("")
				*query = ""
				return true
			case k.Mods == 0 && s.OnKey != nil && (k.Key == KeyDown || k.Key == KeyUp || k.Key == KeyEnter || k.Key == KeyEscape):
				return s.OnKey(k.Key)
			}
			return false
		})
		wrapElement(in).Bind(&s.Handle)
		if s.Label != "" {
			in.Label(s.Label)
		}
		return in
	})
	f.Padding(t.Space(1.5), t.Space(2))
	f.afterInput(func() {
		if in.Changed() {
			f.st.markChanged()
		}
	})
	return f
}
