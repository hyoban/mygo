package platform

// SearchFieldSurface is a Surface that shows search fields of the
// system's, AppKit's NSSearchField on macOS.
type SearchFieldSurface interface {
	NativeViewSurface
	// NewSearchField creates a search field under the surface, hidden
	// until PlaceNativeViews shows it, which tells h what is typed.
	NewSearchField(h SearchFieldHandler) SearchField
}

// SearchField is a field of text to search for, with the system's
// magnifying glass, placeholder and button clearing it.
type SearchField interface {
	NativeView
	// SetText shows text in the field, without telling the handler.
	SetText(text string)
	// SetPlaceholder shows text in the field while it is empty, and label
	// names it to assistive technology.
	SetPlaceholder(text, label string)
	// Height is the field's own height, in DIPs, which it shows at in the
	// middle of the room the content gives it.
	Height() float64
}

// SearchFieldHandler hears what the user does with a search field, on
// the main thread.
type SearchFieldHandler interface {
	// Changed tells the text the field shows as it is typed or cleared.
	Changed(text string)
	// Key tells that the user pressed one of the keys a field passes on:
	// KeyDown, KeyUp, KeyEnter, KeyEscape (where the field is empty). It
	// reports whether the content took it.
	Key(key Key) bool
	// TabOut tells that Tab moved the keyboard out of the field, or
	// Shift+Tab when back: the surface has it again.
	TabOut(back bool)
}
