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
	// native is the system's field, or the drawn one's state.
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
	rt := c.rt
	surface, _ := rt.host.nativeViews().(platform.SearchFieldSurface)
	if surface == nil {
		return drawnSearchField(c, s, query)
	}
	b, _ := s.native.(*searchField)
	if b == nil || b.rt != rt {
		b = &searchField{rt: rt, s: s}
		b.n = surface.NewSearchField(b)
		s.native = b
	}
	b.query = query
	changed := b.typed != nil
	if changed {
		*query = *b.typed
		b.typed = nil
	}
	b.n.SetText(*query)
	label := s.Label
	if label == "" {
		label = s.Placeholder
	}
	placeholder := s.Placeholder
	if placeholder == "" {
		placeholder = c.texts().Search()
	}
	b.n.SetPlaceholder(placeholder, label)
	e := c.newElement(kindBox)
	e.native = b
	e.height = px(float32(b.n.Height()))
	e.flags |= flagFocusable | flagOwnRing
	e.label = label
	wrapElement(e).Bind(&s.Handle)
	b.id = e.id
	if changed {
		e.st.markChanged()
	}
	if !containsView(rt.lists, b) {
		rt.lists = append(rt.lists, b)
	}
	return e
}

// searchField is the system's field of a NativeSearchField, and its
// handler.
type searchField struct {
	rt    *engine
	s     *SearchFieldState
	n     platform.SearchField
	query *string
	// typed is what the user typed since the last frame, nil for none.
	typed *string
	// id is the element showing the field, focused whether it had the
	// focus after the last frame.
	id      uint64
	focused bool
}

func (b *searchField) native() platform.NativeView { return b.n }

// sync gives the field the keyboard where the element took the focus.
func (b *searchField) sync() {
	focused := b.rt.focused == b.id
	if focused && !b.focused {
		b.n.Focus()
	}
	b.focused = focused
}

func (b *searchField) Changed(text string) {
	b.typed = &text
	if b.query != nil {
		*b.query = text
	}
	b.rt.requestFrame()
}

func (b *searchField) Key(key platform.Key) bool {
	if b.s.OnKey == nil {
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
	took := b.s.OnKey(k)
	b.rt.requestFrame()
	return took
}

func (b *searchField) TabOut(back bool) {
	rt := b.rt
	rt.focused = b.id
	rt.moveFocus(back)
	b.focused = rt.focused == b.id
	rt.requestFrame()
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
