package platform

// Toolbar is a toolbar of the system's own over a window's content
// (macOS's NSToolbar), which a backend that has one shows
// (ToolbarWindow): items in order, which tracking separators divide over
// the content's columns.
type Toolbar struct {
	Items []ToolbarItem
	// Columns are the content's columns from the start, as wide as their
	// Width in DIPs, which the separators of the items follow.
	Columns []ToolbarColumn
	// Customizable lets the user rearrange the items, which the system
	// keeps under AutosaveName.
	Customizable bool
	AutosaveName string
	// Preference shows the toolbar as a settings window's, its buttons
	// symbols over their labels, the one Selected names chosen.
	Preference bool
	Selected   string
	// ColumnsChanged tells the widths the columns show at as they change:
	// as the user drags a divider, which user tells, and as a column
	// slides in or out.
	ColumnsChanged func(widths []float64, user bool)
}

// ToolbarColumn is a column of a window's content under a toolbar.
type ToolbarColumn struct {
	Width float64
	// Collapsed hides the column: its separator goes, and its items join
	// the next column's.
	Collapsed bool
	// Sidebar puts the system's sidebar material behind the column.
	Sidebar bool
	// Resizable lets the user drag the divider beside the column, between
	// MinWidth and MaxWidth (0 for none), and slides it in and out as
	// Collapsed changes; Width is its width while it shows.
	Resizable          bool
	MinWidth, MaxWidth float64
}

// ToolbarItemKind is the kind of a ToolbarItem.
type ToolbarItemKind uint8

const (
	// ToolbarButton is a button: a symbol, a title, or both.
	ToolbarButton ToolbarItemKind = iota
	// ToolbarSegmented is a segmented control of Segments, Selected
	// chosen.
	ToolbarSegmented
	// ToolbarActivity is a spinner beside Title, of work under way.
	ToolbarActivity
	// ToolbarSpace and ToolbarFlexibleSpace are room between items, of
	// a fixed width and as wide as there is room.
	ToolbarSpace
	ToolbarFlexibleSpace
	// ToolbarSeparator is the line between two columns, Divider the one
	// after Columns[Divider], which it follows.
	ToolbarSeparator
)

// ToolbarItem is an item of a Toolbar.
type ToolbarItem struct {
	// ID tells the item apart from the others, as the user arranges them.
	ID   string
	Kind ToolbarItemKind
	// Title is the text the item shows, Symbol the name of the system's
	// symbol it shows (SF Symbols), and Label what names it: in the
	// customization palette, the overflow menu and to assistive
	// technology.
	Title, Symbol, Label string
	ToolTip              string
	Disabled, Hidden     bool
	// Segments and Selected are a segmented control's, SegmentSymbols the
	// symbols it shows in place of the names, where it has as many.
	Segments       []string
	SegmentSymbols []string
	Selected       int
	// Divider is a separator's.
	Divider int
}

// ToolbarWindow is a Window that shows a toolbar of the system's own.
type ToolbarWindow interface {
	// SetToolbar shows t, or no toolbar where it is nil, and calls
	// activated, on the main thread, as the user clicks a button or
	// chooses a segment.
	SetToolbar(t *Toolbar, activated func(id string, segment int))
	// ToolbarItemBounds returns where an item shows, in DIPs relative to
	// the content area, and whether it shows.
	ToolbarItemBounds(id string) (Rect, bool)
	// ToolbarColumnWidths returns the widths the columns show at now, as
	// they slide too: nil before the toolbar has columns.
	ToolbarColumnWidths() []float64
}
