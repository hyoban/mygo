package platform

// SourceListSurface is a Surface that shows source lists of the system's,
// AppKit's outline views in the sidebar style on macOS.
type SourceListSurface interface {
	NativeViewSurface
	// NewSourceList creates a source list under the surface, hidden until
	// PlaceNativeViews shows it, which asks h for its rows.
	NewSourceList(h SourceListHandler) SourceList
}

// SourceList is a list of items in a tree, as a sidebar's, which the
// system draws and handles the pointer and the keyboard of. Its items are
// numbers other than 0, which stands for the top of the tree; it asks its
// handler for their children and rows as it shows them.
type SourceList interface {
	NativeView
	// Reload has the list ask its handler again for the children of item,
	// and of the items open inside it, as their rows: 0 for all.
	Reload(item uint64)
	// Expand opens or closes item, sliding its children in or out as its
	// arrow does where animate; the handler hears nothing of it.
	Expand(item uint64, open, animate bool)
	// Expanded reports whether item shows its children.
	Expanded(item uint64) bool
	// Refresh shows the rows of these items anew, as given, where they
	// show.
	Refresh(items []uint64, rows []SourceListItem)
	// Visible returns the items of the rows in view.
	Visible() []uint64
	// Select chooses item, 0 for none, without telling the handler.
	Select(item uint64)
	// ScrollTo scrolls as little as shows the row of item.
	ScrollTo(item uint64)
	// SetStyle sets how the list shows its rows.
	SetStyle(style SourceListStyle)
}

// SourceListStyle is how a source list shows its rows.
type SourceListStyle struct {
	// Inset shows the rows as an inset list's, as an inspector's or a
	// window's own list, rather than a sidebar's.
	Inset bool
	// RowHeight is the height of all rows, in DIPs; 0 for the height of
	// a sidebar's rows of the system's settings.
	RowHeight float64
}

// SourceListHandler answers a source list's questions about its items, and
// hears what the user does with it, on the main thread.
type SourceListHandler interface {
	// Children returns the children of item, those at the top for 0: nil
	// for an item that has none, which shows no arrow.
	Children(item uint64) []uint64
	// Item describes the row of item.
	Item(item uint64) SourceListItem
	// Selected tells that the user chose item, 0 for none.
	Selected(item uint64)
	// Expanded tells that the user opened item, or closed it.
	Expanded(item uint64, open bool)
	// Clicked tells that the user clicked the row of item.
	Clicked(item uint64)
	// Menu returns the context menu of the row of item, nil for none;
	// MenuChosen the ID of the item chosen from it.
	Menu(item uint64) *Menu
	MenuChosen(item uint64, id int)
	// TabOut tells that Tab moved the keyboard out of the list, or
	// Shift+Tab when back: the surface has it again.
	TabOut(back bool)
}

// SourceListItem is the row of an item of a source list.
type SourceListItem struct {
	Title string
	// Bold shows the title bold, as a message list's subjects.
	Bold bool
	// Subtitle is a second line under the title, muted.
	Subtitle string
	// Symbol is the name of a symbol of the system's, shown before the
	// title in Tint, or the accent color without one, as a sidebar's.
	Symbol string
	// Image is a PNG shown before the title in place of a symbol, as big
	// as the row's icons, a mask colored as Tint, or as muted text
	// without one, where Template.
	Image    []byte
	Template bool
	Tint     Color
	// ImageSize is the side of the icon, in DIPs; 0 for the row's.
	ImageSize float64
	// Dimmed shows the title muted.
	Dimmed bool
	// Badges show after the title, at the end of the row.
	Badges []SourceListBadge
	// Busy shows a spinning indicator at the end of the row.
	Busy    bool
	Tooltip string
	// Group makes the row a section's heading.
	Group bool
}

// SourceListBadge is a short text at the end of a source list's row: a
// count, or a status.
type SourceListBadge struct {
	Text string
	// Color is the text's, muted without one.
	Color Color
	Bold  bool
	// Digits shows digits of equal widths, as counts line up.
	Digits bool
}
