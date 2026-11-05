package platform

// ControlSurface is a Surface that shows controls of the system's:
// AppKit's check boxes (NSButton), pop-up buttons (NSPopUpButton), text
// fields (NSTextField) and search fields (NSSearchField) on macOS.
type ControlSurface interface {
	NativeViewSurface
	// NewControl creates a control of the kind under the surface, hidden
	// until PlaceNativeViews shows it, which tells h what the user does
	// with it.
	NewControl(kind ControlKind, h ControlHandler) Control
}

// ControlKind is the kind of a Control.
type ControlKind uint8

const (
	// ControlCheckbox is a check box with its title (SetCheckbox).
	ControlCheckbox ControlKind = iota
	// ControlPopUp is a pop-up button choosing one of its items
	// (SetItems).
	ControlPopUp
	// ControlTextField is a field of text, which the user edits
	// (SetText).
	ControlTextField
	// ControlSearchField is a field of text to search for, with the
	// system's magnifying glass and button clearing it (SetText). Escape
	// clears it while it has text.
	ControlSearchField
)

// IsField reports whether the kind is a field of text, which takes the
// keyboard: ControlTextField or ControlSearchField.
func (k ControlKind) IsField() bool { return k == ControlTextField || k == ControlSearchField }

// Control is a control of the system's, of one of the kinds.
type Control interface {
	NativeView
	// SetCheckbox shows a check box's title, checked or not.
	SetCheckbox(title string, checked bool)
	// SetItems shows a pop-up button's items, the one at selected chosen,
	// -1 for none.
	SetItems(items []string, selected int)
	// SetText shows a field's text, and placeholder while it is empty.
	SetText(text, placeholder string)
	// SetEnabled enables or disables the control; SetLabel names it to
	// assistive technology.
	SetEnabled(enabled bool)
	SetLabel(label string)
	// Checked, Selected and Text are the check box's, the pop-up button's
	// and the field's value, as the user left it.
	Checked() bool
	Selected() int
	Text() string
	// FittingSize is the control's own size, in DIPs, for its content.
	// The control shows at its own height in the middle of a frame
	// higher than that.
	FittingSize() (width, height float64)
}

// ControlHandler hears what the user does with a control, on the main
// thread.
type ControlHandler interface {
	// Changed tells that the user changed the control's value.
	Changed()
	// Key tells that the user pressed, in a field, one of the keys a
	// field passes on: KeyDown, KeyUp, KeyEnter, and KeyEscape (where a
	// search field is empty). It reports whether the content took it.
	Key(key Key) bool
	// TabOut tells that Tab moved the keyboard out of a field, or
	// Shift+Tab when back: the surface has it again.
	TabOut(back bool)
}
