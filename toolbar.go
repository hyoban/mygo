package mygo

import "github.com/egoist/mygo/internal/platform"

// Toolbar is a toolbar of the system's own over a window's content, as
// macOS's NSToolbar draws it: the system's buttons, segmented controls
// and materials (Liquid Glass on macOS 26), the overflow menu of what has
// no room, and, where Customizable, the arrangement the user chooses.
// Separators divide it over the content's Columns, as a sidebar's and an
// inspector's, as AppKit's toolbars over a split view are. Backends
// without one show none (SetToolbar).
type Toolbar struct {
	Items []ToolbarItem
	// Columns are the content's columns from the start, which the
	// separators among Items follow: the items before a separator over
	// its column, those after it over the next ones.
	Columns []ToolbarColumn
	// Customizable lets the user rearrange the items, as the toolbar's
	// context menu offers, which the system keeps under AutosaveName.
	Customizable bool
	AutosaveName string
}

// ToolbarColumn is a column of a window's content under its toolbar.
type ToolbarColumn struct {
	// Width is how wide the column is, in DIPs.
	Width float64
	// Collapsed tells that the column hides: its separator goes, and its
	// items join those of the column after it.
	Collapsed bool
	// Sidebar puts the background of the system's sidebars behind the
	// column, from the window's top to its bottom, as AppKit's split views
	// put their sidebars and inspectors (Xcode's navigator and inspector,
	// Mail's mailboxes): Liquid Glass on macOS 26, the sidebar material
	// before. The window's Content shows a material then
	// (ui.Context.Vibrancy), and the column draws no background of its own
	// there, for it to show.
	Sidebar bool
}

// ToolbarItemKind is the kind of a ToolbarItem.
type ToolbarItemKind uint8

const (
	// ToolbarButton is a button showing Symbol, Title or both, which
	// Click runs.
	ToolbarButton ToolbarItemKind = iota
	// ToolbarSegmented is a segmented control of Segments, the one at
	// Selected chosen, which Select hears the user choose.
	ToolbarSegmented
	// ToolbarActivity is a button showing a spinner beside Title, of work
	// under way, which Click runs: to stop it, say.
	ToolbarActivity
	// ToolbarSpace is room of a fixed width between items.
	ToolbarSpace
	// ToolbarFlexibleSpace is as much room between items as there is.
	ToolbarFlexibleSpace
	// ToolbarSeparator divides the toolbar between the column Divider
	// and the next, following their width.
	ToolbarSeparator
)

// ToolbarItem is an item of a Toolbar.
type ToolbarItem struct {
	// ID tells the item apart, across SetToolbar calls and as the user
	// arranges the items: one of each.
	ID   string
	Kind ToolbarItemKind
	// Title is the text the item shows, and Symbol the name of the
	// system's symbol it shows (SF Symbols, as "sidebar.left").
	Title, Symbol string
	// Label names the item: in the customization palette, in the overflow
	// menu, and to assistive technology. ToolTip shows over it.
	Label, ToolTip string
	// Disabled grays the item out; Hidden leaves it out, where it would
	// show, for as long.
	Disabled, Hidden bool
	// Segments and Selected are a segmented control's.
	Segments []string
	Selected int
	// Divider is a separator's: the column it follows the end of.
	Divider int
	// Click runs as the user clicks a button, Select as they choose a
	// segment, on the main thread, where a window's Content builds; the
	// Content builds again after.
	Click  func()
	Select func(segment int)
}

// SetToolbar shows t as the window's toolbar, or takes it away where t is
// nil. Call it again with the items as they are now, as the state they
// show changes: in every pass of the window's Content, say, which costs
// little where nothing changed. It returns whether the window shows a
// toolbar of the system's: false where the backend has none (only macOS
// has), for the app to draw its own.
func (w *Window) SetToolbar(t *Toolbar) bool {
	return get(w, func(n platform.Window) bool {
		tw, ok := n.(platform.ToolbarWindow)
		if !ok {
			return false
		}
		if t == nil {
			w.toolbarItems = nil
			tw.SetToolbar(nil, nil)
			return true
		}
		items := make(map[string]ToolbarItem, len(t.Items))
		p := &platform.Toolbar{Customizable: t.Customizable, AutosaveName: t.AutosaveName}
		for _, it := range t.Items {
			items[it.ID] = it
			p.Items = append(p.Items, platform.ToolbarItem{
				ID: it.ID, Kind: platform.ToolbarItemKind(it.Kind), Title: it.Title, Symbol: it.Symbol, Label: it.Label,
				ToolTip: it.ToolTip, Disabled: it.Disabled, Hidden: it.Hidden, Segments: it.Segments, Selected: it.Selected, Divider: it.Divider,
			})
		}
		for _, c := range t.Columns {
			p.Columns = append(p.Columns, platform.ToolbarColumn(c))
		}
		w.toolbarItems = items
		tw.SetToolbar(p, w.toolbarActivated)
		return true
	})
}

// toolbarActivated runs the handler of the item the user clicked or
// chose a segment of, then has the Content build again. Main thread only.
func (w *Window) toolbarActivated(id string, segment int) {
	it, ok := w.toolbarItems[id]
	if !ok {
		return
	}
	switch {
	case it.Kind == ToolbarSegmented && it.Select != nil:
		it.Select(segment)
	case it.Click != nil:
		it.Click()
	}
	w.contentChanged()
}

// ToolbarItemBounds returns where the toolbar's item id shows, in DIPs
// relative to the content area, as a popover's or a menu's Anchor takes
// it, and whether it shows: not while hidden, in the overflow menu, or
// without a toolbar of the system's.
func (w *Window) ToolbarItemBounds(id string) (Rectangle, bool) {
	type result struct {
		r  Rectangle
		ok bool
	}
	res := get(w, func(n platform.Window) result {
		tw, ok := n.(platform.ToolbarWindow)
		if !ok {
			return result{}
		}
		r, ok := tw.ToolbarItemBounds(id)
		return result{Rectangle(r), ok}
	})
	return res.r, res.ok
}
