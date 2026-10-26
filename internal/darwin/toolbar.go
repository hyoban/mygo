//go:build darwin

package darwin

import (
	"slices"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A window's Toolbar is an NSToolbar whose delegate (MyGoToolbarDelegate,
// one a window) makes its items from the items the app gives: AppKit
// draws them as its own, in Liquid Glass on macOS 26, puts what has no
// room in the overflow menu, and lets the user rearrange them. The
// content is a surface of its own rather than an NSSplitView, so the
// columns the separators follow are those of a split view of the same
// widths behind it, transparent, which NSTrackingSeparatorToolbarItem
// tracks as it does a split view controller's.

// toolbar is a window's NSToolbar and what it was made from.
type toolbar struct {
	tb, delegate id
	// ctl is the split view controller behind the content, split its
	// split view, columnItems its items, one a column, and applied the
	// columns as they were last given; dragging tells that the user drags
	// a divider.
	ctl, split  id
	columnItems []id
	applied     []platform.ToolbarColumn
	dragging    bool
	// spec is the toolbar SetToolbar gave last, and items the items in
	// the toolbar by ID, with the button of an activity and of one
	// showing a symbol and a title in buttons.
	spec *platform.Toolbar
	// sidebars are the backgrounds behind the columns that have one
	// (ToolbarColumn.Sidebar), by column.
	sidebars map[int]id
	// fullScreenSidebars are their backgrounds behind the toolbar in full
	// screen (placeFullScreenSidebars), by column.
	fullScreenSidebars map[int]id
	// backdrop is the window's background under the sidebars' glass.
	backdrop  id
	items     map[string]id
	buttons   map[string]id
	activated func(id string, segment int)
}

var (
	toolbarClassOnce sync.Once
	// toolbarOwners are the windows of the toolbars' delegates. Main
	// thread only.
	toolbarOwners = map[id]*window{}
)

const (
	spaceItemIdentifier         = "NSToolbarSpaceItem"
	flexibleSpaceItemIdentifier = "NSToolbarFlexibleSpaceItem"
	toolbarDisplayIconOnly      = 2 // NSToolbarDisplayModeIconOnly
	windowToolbarStyleUnified   = 3 // NSWindowToolbarStyleUnified
	bezelStyleToolbar           = 11
	imageLeading                = 7 // NSImageLeading
	// activitySpinner is the size of an activity's spinner, and
	// activityInset where it is from its button's start: where the
	// button's symbol would be.
	activitySpinner = 16
	activityInset   = 10
)

// identifier is the identifier of an item in the toolbar: the system's
// for room between items.
func identifier(it platform.ToolbarItem) string {
	switch it.Kind {
	case platform.ToolbarSpace:
		return spaceItemIdentifier
	case platform.ToolbarFlexibleSpace:
		return flexibleSpaceItemIdentifier
	}
	return it.ID
}

func registerToolbarClass() {
	identifiers := func(self id) id {
		w := toolbarOwners[self]
		if w == nil || w.toolbar == nil || w.toolbar.spec == nil {
			return nsArray()
		}
		var ids []id
		for _, it := range w.toolbar.spec.Items {
			ids = append(ids, nsString(identifier(it)))
		}
		return nsArray(ids...)
	}
	classDef("MyGoToolbarDelegate", "NSObject", []string{"NSToolbarDelegate"}, []objc.MethodDef{
		method("toolbarDefaultItemIdentifiers:", func(self id, _ objc.SEL, _ id) id { return identifiers(self) }),
		method("toolbarAllowedItemIdentifiers:", func(self id, _ objc.SEL, _ id) id { return identifiers(self) }),
		method("toolbar:itemForItemIdentifier:willBeInsertedIntoToolbar:", func(self id, _ objc.SEL, _, ident id, insert bool) id {
			w := toolbarOwners[self]
			if w == nil || w.toolbar == nil || w.toolbar.spec == nil {
				return 0
			}
			name := goString(ident)
			i := slices.IndexFunc(w.toolbar.spec.Items, func(it platform.ToolbarItem) bool { return it.ID == name })
			if i < 0 {
				return 0
			}
			return w.toolbar.makeItem(w.toolbar.spec.Items[i], insert)
		}),
		method("mygoColumnsResized:", func(self id, _ objc.SEL, note id) {
			if w := toolbarOwners[self]; w != nil {
				w.columnsResized()
			}
		}),
		method("toolbarItemClicked:", func(self id, _ objc.SEL, sender id) {
			w := toolbarOwners[self]
			if w == nil || w.toolbar == nil || w.toolbar.activated == nil {
				return
			}
			var name string
			segment := -1
			switch {
			case respondsTo(sender, "itemIdentifier"):
				name = goString(send(sender, "itemIdentifier"))
				if respondsTo(sender, "selectedIndex") {
					segment = sendInt(sender, "selectedIndex")
				}
			default:
				name = goString(send(sender, "identifier"))
			}
			w.toolbar.activated(name, segment)
		}),
	})
}

// makeItem makes the NSToolbarItem of it, autoreleased, which the toolbar
// keeps where it inserts it rather than show it in its palette.
func (t *toolbar) makeItem(it platform.ToolbarItem, insert bool) id {
	ident := nsString(it.ID)
	action := uintptr(sel("toolbarItemClicked:"))
	var item id
	switch it.Kind {
	case platform.ToolbarSeparator:
		item = send(class("NSTrackingSeparatorToolbarItem"), "trackingSeparatorToolbarItemWithIdentifier:splitView:dividerIndex:",
			uintptr(ident), uintptr(t.split), uintptr(it.Divider))
	case platform.ToolbarSegmented:
		var titles, images []id
		for i, s := range it.Segments {
			titles = append(titles, nsString(s))
			if len(it.SegmentSymbols) == len(it.Segments) {
				images = append(images, symbol(it.SegmentSymbols[i], s))
			}
		}
		if images != nil {
			item = send(class("NSToolbarItemGroup"), "groupWithItemIdentifier:images:selectionMode:labels:target:action:",
				uintptr(ident), uintptr(nsArray(images...)), 0, uintptr(nsArray(titles...)), uintptr(t.delegate), action)
		} else {
			item = send(class("NSToolbarItemGroup"), "groupWithItemIdentifier:titles:selectionMode:labels:target:action:",
				uintptr(ident), uintptr(nsArray(titles...)), 0, uintptr(nsArray(titles...)), uintptr(t.delegate), action)
		}
		send(item, "setSelectedIndex:", uintptr(it.Selected))
	default:
		item = autorelease(send(alloc("NSToolbarItem"), "initWithItemIdentifier:", uintptr(ident)))
		switch {
		case it.Kind == platform.ToolbarActivity:
			// A button of the toolbar's as the others, its title beside a
			// spinner where a symbol would be: the same capsule and
			// padding as its neighbors, which Click answers, as stopping
			// the work.
			blank := autorelease(send(alloc("NSImage"), "init"))
			msgSetSize(blank, sel("setSize:"), NSSize{Width: activitySpinner, Height: activitySpinner})
			b := send(class("NSButton"), "buttonWithTitle:image:target:action:",
				uintptr(nsString(it.Title)), uintptr(blank), uintptr(t.delegate), action)
			send(b, "setBezelStyle:", bezelStyleToolbar)
			send(b, "setImagePosition:", imageLeading)
			send(b, "setIdentifier:", uintptr(ident))
			spinner := autorelease(send(alloc("NSProgressIndicator"), "init"))
			send(spinner, "setStyle:", 1)       // NSProgressIndicatorStyleSpinning
			send(spinner, "setControlSize:", 1) // NSControlSizeSmall
			send(spinner, "setIndeterminate:", 1)
			send(spinner, "startAnimation:", 0)
			send(b, "addSubview:", uintptr(spinner))
			// Over the blank image, which the title follows, as the
			// button's own height has it centered.
			frame := msgRect(b, sel("frame"))
			msgSetRect(spinner, sel("setFrame:"), NSRect{
				Origin: NSPoint{X: activityInset, Y: (frame.Size.Height - activitySpinner) / 2},
				Size:   NSSize{Width: activitySpinner, Height: activitySpinner},
			})
			send(spinner, "setAutoresizingMask:", 4|8|32) // NSViewMaxXMargin, NSViewMinYMargin, NSViewMaxYMargin
			send(item, "setView:", uintptr(b))
			if insert {
				t.buttons[it.ID] = b
			}
		case it.Symbol != "" && it.Title != "":
			// The symbol and the title side by side, which an item of
			// AppKit's shows one of.
			b := send(class("NSButton"), "buttonWithTitle:image:target:action:",
				uintptr(nsString(it.Title)), uintptr(symbol(it.Symbol, it.Label)), uintptr(t.delegate), action)
			send(b, "setBezelStyle:", bezelStyleToolbar)
			send(b, "setImagePosition:", imageLeading)
			send(b, "setIdentifier:", uintptr(ident))
			send(item, "setView:", uintptr(b))
			if insert {
				t.buttons[it.ID] = b
			}
		default:
			send(item, "setBordered:", 1)
			send(item, "setTarget:", uintptr(t.delegate))
			send(item, "setAction:", action)
		}
		send(item, "setAutovalidates:", 0)
	}
	if insert {
		t.items[it.ID] = retain(item)
	}
	t.apply(item, it, platform.ToolbarItem{Kind: it.Kind, Selected: -1, Divider: -1})
	return item
}

// symbol returns the image of a symbol of the system's, described as
// label to assistive technology, or 0 for none.
func symbol(name, label string) id {
	if name == "" {
		return 0
	}
	return send(class("NSImage"), "imageWithSystemSymbolName:accessibilityDescription:", uintptr(nsString(name)), uintptr(nsString(label)))
}

// apply gives item what of it changed since was.
func (t *toolbar) apply(item id, it, was platform.ToolbarItem) {
	if item == 0 {
		return
	}
	if it.Label != was.Label {
		send(item, "setLabel:", uintptr(nsString(it.Label)))
		send(item, "setPaletteLabel:", uintptr(nsString(it.Label)))
	}
	if it.ToolTip != was.ToolTip {
		send(item, "setToolTip:", uintptr(nsString(it.ToolTip)))
	}
	if it.Hidden != was.Hidden && respondsTo(item, "setHidden:") {
		send(item, "setHidden:", boolArg(it.Hidden))
	}
	switch it.Kind {
	case platform.ToolbarSegmented:
		if it.Selected != was.Selected {
			send(item, "setSelectedIndex:", uintptr(it.Selected))
		}
		if it.Disabled != was.Disabled {
			send(item, "setEnabled:", boolArg(!it.Disabled))
		}
	case platform.ToolbarButton, platform.ToolbarActivity:
		if b := t.buttons[it.ID]; b != 0 {
			if it.Title != was.Title {
				send(b, "setTitle:", uintptr(nsString(it.Title)))
			}
			if it.Symbol != was.Symbol && it.Kind == platform.ToolbarButton {
				send(b, "setImage:", uintptr(symbol(it.Symbol, it.Label)))
			}
			if it.ToolTip != was.ToolTip {
				send(b, "setToolTip:", uintptr(nsString(it.ToolTip)))
			}
			if it.Disabled != was.Disabled {
				send(b, "setEnabled:", boolArg(!it.Disabled))
			}
			return
		}
		if it.Title != was.Title {
			send(item, "setTitle:", uintptr(nsString(it.Title)))
		}
		if it.Symbol != was.Symbol {
			send(item, "setImage:", uintptr(symbol(it.Symbol, it.Label)))
		}
		if it.Disabled != was.Disabled {
			send(item, "setEnabled:", boolArg(!it.Disabled))
		}
	}
}

func (w *window) SetToolbar(spec *platform.Toolbar, activated func(id string, segment int)) {
	withPool(func() {
		if spec == nil {
			w.removeToolbar()
			return
		}
		toolbarClassOnce.Do(registerToolbarClass)
		t := w.toolbar
		if t == nil {
			t = &toolbar{items: map[string]id{}, buttons: map[string]id{}, sidebars: map[int]id{}, fullScreenSidebars: map[int]id{}}
			w.toolbar = t
			t.delegate = send(alloc("MyGoToolbarDelegate"), "init")
			toolbarOwners[t.delegate] = w
		}
		t.activated = activated
		// The split view first, which the separators track as they are
		// made.
		w.placeColumns(spec.Columns)
		w.placeSidebars()
		spec = hideCollapsed(spec)
		was := t.spec
		t.spec = spec
		w.placeFullScreenSidebars()
		if t.tb == 0 {
			name := spec.AutosaveName
			if name == "" {
				name = "mygo.toolbar"
			}
			t.tb = send(alloc("NSToolbar"), "initWithIdentifier:", uintptr(nsString(name)))
			send(t.tb, "setDelegate:", uintptr(t.delegate))
			send(t.tb, "setDisplayMode:", toolbarDisplayIconOnly)
			send(t.tb, "setAllowsUserCustomization:", boolArg(spec.Customizable))
			send(t.tb, "setAutosavesConfiguration:", boolArg(spec.Customizable && spec.AutosaveName != ""))
			send(w.win, "setToolbarStyle:", windowToolbarStyleUnified)
			send(w.win, "setToolbar:", uintptr(t.tb))
			return
		}
		if !sameItems(was, spec) {
			// Other items: the toolbar's anew.
			for n := sendInt(send(t.tb, "items"), "count"); n > 0; n-- {
				send(t.tb, "removeItemAtIndex:", 0)
			}
			for _, item := range t.items {
				release(item)
			}
			clear(t.items)
			clear(t.buttons)
			for i, it := range spec.Items {
				send(t.tb, "insertItemWithItemIdentifier:atIndex:", uintptr(nsString(identifier(it))), uintptr(i))
			}
			return
		}
		for i, it := range spec.Items {
			t.apply(t.items[it.ID], it, was.Items[i])
		}
	})
}

// hideCollapsed returns spec with the separators beside a collapsed
// column hidden, as a split view controller's toolbar hides the one of a
// sidebar it collapsed: the split view behind the content tells none.
func hideCollapsed(spec *platform.Toolbar) *platform.Toolbar {
	collapsed := func(i int) bool { return i >= 0 && i < len(spec.Columns) && spec.Columns[i].Collapsed }
	out := *spec
	out.Items = slices.Clone(spec.Items)
	for i, it := range out.Items {
		if it.Kind == platform.ToolbarSeparator && (collapsed(it.Divider) || collapsed(it.Divider+1)) {
			out.Items[i].Hidden = true
		}
	}
	return &out
}

// sameItems reports whether two toolbars have the same items, by kind and
// ID, in the same order.
func sameItems(a, b *platform.Toolbar) bool {
	if a == nil || b == nil || len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if a.Items[i].ID != b.Items[i].ID || a.Items[i].Kind != b.Items[i].Kind || a.Items[i].Divider != b.Items[i].Divider {
			return false
		}
	}
	return true
}

// placeColumns lays the columns out in the split view controller behind
// the content, making it as the first ones come: its split view's
// dividers are the user's to drag beside the Resizable columns, which it
// slides in and out as they collapse and show, as AppKit's sidebars and
// inspectors. The content lays its own columns out at the widths the
// split view's show at (ToolbarColumnWidths).
func (w *window) placeColumns(columns []platform.ToolbarColumn) {
	t := w.toolbar
	if len(columns) < 2 {
		return
	}
	first := t.ctl == 0
	if first {
		t.makeColumns(w, len(columns))
	}
	if len(columns) != len(t.columnItems) {
		return // a toolbar of as many columns as the first
	}
	for i, c := range columns {
		item := t.columnItems[i]
		var was platform.ToolbarColumn
		if !first {
			was = t.applied[i]
		}
		if first || c.MinWidth != was.MinWidth {
			msgSetFloat(item, sel("setMinimumThickness:"), thickness(c.MinWidth))
		}
		if first || c.MaxWidth != was.MaxWidth {
			msgSetFloat(item, sel("setMaximumThickness:"), thickness(c.MaxWidth))
		}
		if first || c.Resizable != was.Resizable {
			send(item, "setCanCollapse:", boolArg(c.Resizable))
			// The columns that keep their widths as the window resizes,
			// the others taking what it gains or loses.
			priority := float32(250) // NSLayoutPriorityDefaultLow
			if c.Resizable {
				priority = 260
			}
			msgSetFloat32(item, sel("setHoldingPriority:"), priority)
		}
	}
	for i, c := range columns {
		item := t.columnItems[i]
		collapsed := sendBool(item, "isCollapsed")
		switch {
		case !c.Resizable && !c.Collapsed && collapsed:
			send(item, "setCollapsed:", 0)
		case !c.Resizable:
		case first:
			send(item, "setCollapsed:", boolArg(c.Collapsed))
		case c.Collapsed != collapsed:
			// Sliding, as AppKit's split views slide their sidebars.
			send(send(item, "animator"), "setCollapsed:", boolArg(c.Collapsed))
			continue
		}
		if c.Resizable && !c.Collapsed && !t.dragging && (first || c.Width != t.applied[i].Width) && c.Width > 0 {
			t.setWidth(i, c.Width)
		}
	}
	t.applied = slices.Clone(columns)
}

// thickness is a width of a split view item, or none for 0.
func thickness(v float64) float64 {
	if v <= 0 {
		return -1 // NSSplitViewItemUnspecifiedDimension
	}
	return v
}

// makeColumns makes the split view controller of n columns behind the
// content, its divider lines clear: the columns' colors part them.
func (t *toolbar) makeColumns(w *window, n int) {
	splitOnce.Do(func() {
		purego.RegisterFunc(&msgSetFloat32, msgSendAddr)
		registerSplitViewClass()
	})
	bounds := msgRect(w.view, sel("bounds"))
	t.ctl = send(class("NSSplitViewController"), "alloc")
	t.ctl = send(t.ctl, "init")
	t.split = msgInitRect(send(class("MyGoSplitView"), "alloc"), sel("initWithFrame:"), bounds)
	send(t.split, "setVertical:", 1)
	send(t.split, "setDividerStyle:", 2) // NSSplitViewDividerStyleThin
	send(t.ctl, "setSplitView:", uintptr(t.split))
	splitOwners[t.split] = w
	for range n {
		vc := send(send(class("NSViewController"), "alloc"), "init")
		view := autorelease(msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), NSRect{}))
		send(vc, "setView:", uintptr(view))
		item := send(class("NSSplitViewItem"), "splitViewItemWithViewController:", uintptr(vc))
		release(vc)
		send(t.ctl, "addSplitViewItem:", uintptr(item))
		t.columnItems = append(t.columnItems, retain(item))
	}
	view := send(t.ctl, "view")
	msgSetRect(view, sel("setFrame:"), bounds)
	send(view, "setAutoresizingMask:", nsViewWidthHeightSizable)
	// Right under the surface, over the window's materials, which would
	// take the pointer on its dividers otherwise; the sidebars'
	// backgrounds over it let it through (MyGoSidebarView).
	var above uintptr
	if w.surface != nil {
		above = uintptr(w.surface.view)
	}
	send(w.view, "addSubview:positioned:relativeTo:", uintptr(view), nsWindowBelow, above)
	center := send(class("NSNotificationCenter"), "defaultCenter")
	send(center, "addObserver:selector:name:object:", uintptr(t.delegate), uintptr(sel("mygoColumnsResized:")),
		uintptr(nsString("NSSplitViewDidResizeSubviewsNotification")), uintptr(t.split))
}

// setWidth has column i show width wide, moving the divider beside it: at
// its end, or at its start for the last.
func (t *toolbar) setWidth(i int, width float64) {
	views := arrayItems(send(t.split, "arrangedSubviews"))
	if i >= len(views) {
		return
	}
	if i < len(views)-1 {
		f := msgRect(views[i], sel("frame"))
		msgSetPosition(t.split, sel("setPosition:ofDividerAtIndex:"), f.Origin.X+width, i)
		return
	}
	total := msgRect(t.split, sel("bounds")).Size.Width
	msgSetPosition(t.split, sel("setPosition:ofDividerAtIndex:"), total-width-msgFloat(t.split, sel("dividerThickness")), i-1)
}

// columnFrames returns where the columns show now, in the content view,
// as they slide too: those collapsed or hidden 0 wide.
func (t *toolbar) columnFrames() []NSRect {
	if t == nil || t.split == 0 {
		return nil
	}
	views := arrayItems(send(t.split, "arrangedSubviews"))
	frames := make([]NSRect, len(views))
	for i, v := range views {
		f := msgRect(v, sel("frame"))
		if sendBool(v, "isHidden") {
			f.Size.Width = 0
		}
		frames[i] = f
	}
	return frames
}

func (w *window) ToolbarColumnWidths() []float64 {
	frames := w.toolbar.columnFrames()
	if frames == nil {
		return nil
	}
	widths := make([]float64, len(frames))
	for i, f := range frames {
		widths[i] = f.Size.Width
	}
	return widths
}

// columnsResized follows the columns of the split view as they change:
// the sidebars' backgrounds go with them, and the content hears of them.
func (w *window) columnsResized() {
	t := w.toolbar
	if t == nil || t.spec == nil {
		return
	}
	w.placeSidebars()
	w.placeFullScreenSidebars()
	if changed := t.spec.ColumnsChanged; changed != nil {
		changed(w.ToolbarColumnWidths(), t.dragging)
	}
}

// placeSidebars puts the sidebars' backgrounds behind the columns where
// they show now.
func (w *window) placeSidebars() {
	t := w.toolbar
	frames := t.columnFrames()
	for i, c := range t.applied {
		if i >= len(frames) {
			break
		}
		c.Width = frames[i].Size.Width
		c.Collapsed = c.Width <= 0
		w.placeSidebar(i, c, frames[i].Origin.X)
	}
}

// dividerAt reports whether the pointer at x in the content view is the
// split view's, on a divider the user drags: one beside a Resizable
// column that shows, its line and a few points around it, as AppKit's thin
// dividers take the pointer.
func (t *toolbar) dividerAt(x float64) bool {
	if t == nil || t.split == 0 {
		return false
	}
	frames := t.columnFrames()
	for i := 0; i+1 < len(frames) && i+1 < len(t.applied); i++ {
		if !t.applied[i].Resizable && !t.applied[i+1].Resizable {
			continue
		}
		if frames[i].Size.Width <= 0 || frames[i+1].Size.Width <= 0 {
			continue
		}
		at := frames[i].Origin.X + frames[i].Size.Width
		if x >= at-3 && x <= frames[i+1].Origin.X+3 {
			return true
		}
	}
	return false
}

var (
	splitOnce     sync.Once
	msgSetFloat32 func(obj id, s objc.SEL, v float32)
	// splitOwners are the windows of the toolbars' split views.
	splitOwners = map[id]*window{}
)

// registerSplitViewClass defines MyGoSplitView, a split view whose
// dividers draw nothing, which tells its window that the user drags one.
func registerSplitViewClass() {
	// A sidebar's background takes the pointer only for the views of the
	// system's on it (embed.go): elsewhere it goes to the split view under
	// it, which drags its dividers.
	classDef("MyGoSidebarView", "NSView", nil, []objc.MethodDef{
		method("hitTest:", func(self id, cmd objc.SEL, p NSPoint) id {
			r := sendSuperPoint(self, "MyGoSidebarView", cmd, p)
			for v := r; v != 0 && v != self; v = send(v, "superview") {
				if sendBool(v, "isKindOfClass:", uintptr(class("MyGoEmbedClip"))) {
					return r
				}
			}
			return 0
		}),
	})
	classDef("MyGoSplitView", "NSSplitView", nil, []objc.MethodDef{
		method("dividerColor", func(self id, _ objc.SEL) id { return send(class("NSColor"), "clearColor") }),
		method("mouseDown:", func(self id, cmd objc.SEL, ev id) {
			w := splitOwners[self]
			if w == nil || w.toolbar == nil {
				sendSuper(self, "MyGoSplitView", cmd, uintptr(ev))
				return
			}
			// The split view tracks the drag until the button goes up.
			w.toolbar.dragging = true
			sendSuper(self, "MyGoSplitView", cmd, uintptr(ev))
			if w.toolbar != nil {
				w.columnsResized()
				w.toolbar.dragging = false
			}
		}),
	})
}

// placeSidebar puts the sidebar's background behind column i, from x,
// where the column has one and shows, and hides it elsewhere, under the
// content.
func (w *window) placeSidebar(i int, c platform.ToolbarColumn, x float64) {
	t := w.toolbar
	v := t.sidebars[i]
	if !c.Sidebar || c.Collapsed {
		if v != 0 {
			send(v, "setHidden:", 1)
		}
		return
	}
	bounds := msgRect(w.view, sel("bounds"))
	frame := NSRect{Origin: NSPoint{X: x}, Size: NSSize{Width: c.Width, Height: bounds.Size.Height}}
	if v == 0 {
		// The glass rounds all of its corners alike, whichever corners its
		// layer masks, so it is square in a view that rounds the window's
		// corners alone (v).
		glass, glassy := sidebarGlass(NSRect{Size: frame.Size})
		if glassy {
			w.placeBackdrop()
		}
		v = msgInitRect(send(class("MyGoSidebarView"), "alloc"), sel("initWithFrame:"), frame)
		send(v, "addSubview:", uintptr(autorelease(glass)))
		send(v, "setWantsLayer:", 1)
		send(v, "setAutoresizingMask:", 16) // height sizable
		var above uintptr
		if w.surface != nil {
			above = uintptr(w.surface.view)
		}
		send(w.view, "addSubview:positioned:relativeTo:", uintptr(v), nsWindowBelow, above)
		t.sidebars[i] = v
	}
	send(v, "setHidden:", 0)
	msgSetRect(v, sel("setFrame:"), frame)
	// Rounded where it meets the window's corners, which the window, see
	// through for its material, takes from what its content draws.
	var corners uintptr
	if x <= 0 {
		corners |= 1 | 4 // kCALayerMinXMinYCorner, kCALayerMinXMaxYCorner
	}
	if x+c.Width >= bounds.Size.Width {
		corners |= 2 | 8 // kCALayerMaxXMinYCorner, kCALayerMaxXMaxYCorner
	}
	layer := send(v, "layer")
	radius := 0.0
	if corners != 0 && respondsTo(w.win, "_cornerRadius") && !w.IsFullScreen() {
		radius = msgFloat(w.win, sel("_cornerRadius"))
	}
	msgSetFloat(layer, sel("setCornerRadius:"), radius)
	send(layer, "setMaskedCorners:", corners)
	send(layer, "setCornerCurve:", uintptr(nsString("continuous")))
	send(layer, "setMasksToBounds:", boolArg(radius > 0))
}

// sidebarGlass returns a sidebar's background in frame: Liquid Glass, as
// AppKit's split views put their sidebars on macOS 26, over the window's
// background (placeBackdrop), which glassy tells; the sidebar material
// before.
func sidebarGlass(frame NSRect) (glass id, glassy bool) {
	if hasClass("NSGlassEffectView") {
		glass = msgInitRect(alloc("NSGlassEffectView"), sel("initWithFrame:"), frame)
		send(glass, "setStyle:", 0) // NSGlassEffectViewStyleRegular
		// The variant of glass AppKit gives a split view's sidebar,
		// darker than the regular one, which has no public name.
		if respondsTo(glass, "set_variant:") {
			send(glass, "set_variant:", 17)
		}
		if respondsTo(glass, "set_adaptiveAppearance:") {
			send(glass, "set_adaptiveAppearance:", 1)
		}
		msgSetFloat(glass, sel("setCornerRadius:"), 0)
		glassy = true
	} else {
		glass = msgInitRect(alloc("NSVisualEffectView"), sel("initWithFrame:"), frame)
		send(glass, "setMaterial:", vibrancyMaterials["sidebar"])
		send(glass, "setBlendingMode:", 0) // NSVisualEffectBlendingModeBehindWindow
	}
	send(glass, "setAutoresizingMask:", nsViewWidthHeightSizable)
	return glass, glassy
}

// placeFullScreenSidebars puts the sidebars' backgrounds behind the
// toolbar in full screen, as AppKit's split views put them, the sidebars
// going on through the toolbar as they do in the window: AppKit moves the
// toolbar into a window of its own, over the content, whose backgrounds
// are opaque. Out of full screen, they hide, as the toolbar is the
// window's again.
func (w *window) placeFullScreenSidebars() {
	t := w.toolbar
	if t == nil || t.spec == nil {
		return
	}
	var container, titlebar id
	if w.IsFullScreen() {
		if b := send(w.win, "standardWindowButton:", 0); b != 0 {
			titlebar = send(b, "superview")
			container = send(titlebar, "superview")
		}
	}
	frames := t.columnFrames()
	for i, c := range t.applied {
		if i < len(frames) {
			c.Width = frames[i].Size.Width
			c.Collapsed = c.Width <= 0
		}
		at := 0.0
		if i < len(frames) {
			at = frames[i].Origin.X
		}
		v := t.fullScreenSidebars[i]
		shows := container != 0 && c.Sidebar && !c.Collapsed
		if !shows {
			if v != 0 {
				send(v, "setHidden:", 1)
			}
		} else {
			height := msgRect(container, sel("bounds")).Size.Height
			frame := NSRect{Origin: NSPoint{X: at}, Size: NSSize{Width: c.Width, Height: height}}
			if v == 0 {
				// Over the window's background, as the sidebars of the
				// content are (placeBackdrop).
				v = msgInitRect(alloc("NSView"), sel("initWithFrame:"), frame)
				send(v, "setAutoresizingMask:", 16) // height sizable
				glass, glassy := sidebarGlass(NSRect{Size: frame.Size})
				if glassy {
					backdrop := msgInitRect(alloc("NSVisualEffectView"), sel("initWithFrame:"), NSRect{Size: frame.Size})
					send(backdrop, "setMaterial:", vibrancyMaterials["content"])
					send(backdrop, "setBlendingMode:", 1) // NSVisualEffectBlendingModeWithinWindow
					send(backdrop, "setState:", 1)        // NSVisualEffectStateActive
					send(backdrop, "setAutoresizingMask:", nsViewWidthHeightSizable)
					send(v, "addSubview:", uintptr(autorelease(backdrop)))
				}
				send(v, "addSubview:", uintptr(autorelease(glass)))
				t.fullScreenSidebars[i] = v
			}
			if send(v, "superview") != container {
				send(container, "addSubview:positioned:relativeTo:", uintptr(v), nsWindowBelow, uintptr(titlebar))
			}
			send(v, "setHidden:", 0)
			msgSetRect(v, sel("setFrame:"), frame)
		}
	}
}

// placeBackdrop puts behind all of the window the background AppKit gives
// a window whose split view has a sidebar on macOS 26, which the sidebars'
// glass is over: the content background material, within the window.
func (w *window) placeBackdrop() {
	t := w.toolbar
	if t.backdrop != 0 {
		return
	}
	t.backdrop = msgInitRect(alloc("NSVisualEffectView"), sel("initWithFrame:"), msgRect(w.view, sel("bounds")))
	send(t.backdrop, "setMaterial:", vibrancyMaterials["content"])
	send(t.backdrop, "setBlendingMode:", 1) // NSVisualEffectBlendingModeWithinWindow
	send(t.backdrop, "setState:", 1)        // NSVisualEffectStateActive
	send(t.backdrop, "setAutoresizingMask:", nsViewWidthHeightSizable)
	send(w.view, "addSubview:positioned:relativeTo:", uintptr(t.backdrop), nsWindowBelow, 0)
}

// removeToolbar takes the toolbar and its split view away.
func (w *window) removeToolbar() {
	t := w.toolbar
	if t == nil {
		return
	}
	w.toolbar = nil
	if t.tb != 0 {
		send(w.win, "setToolbar:", 0)
		send(t.tb, "setDelegate:", 0)
		release(t.tb)
	}
	for _, item := range t.items {
		release(item)
	}
	if t.ctl != 0 {
		send(send(class("NSNotificationCenter"), "defaultCenter"), "removeObserver:", uintptr(t.delegate))
		send(send(t.ctl, "view"), "removeFromSuperview")
		delete(splitOwners, t.split)
		for _, item := range t.columnItems {
			release(item)
		}
		release(t.split)
		release(t.ctl)
	}
	for _, v := range t.sidebars {
		send(v, "removeFromSuperview")
		release(v)
	}
	for _, v := range t.fullScreenSidebars {
		send(v, "removeFromSuperview")
		release(v)
	}
	if t.backdrop != 0 {
		send(t.backdrop, "removeFromSuperview")
		release(t.backdrop)
	}
	delete(toolbarOwners, t.delegate)
	release(t.delegate)
}

func (w *window) ToolbarItemBounds(name string) (platform.Rect, bool) {
	t := w.toolbar
	if t == nil || t.items[name] == 0 {
		return platform.Rect{}, false
	}
	var r platform.Rect
	ok := false
	withPool(func() {
		item := t.items[name]
		view := send(item, "view")
		if view == 0 {
			// The view AppKit shows an item of its own in, among the
			// window's title bar's.
			view = viewOfItem(send(send(w.win, "contentView"), "superview"), item)
		}
		if view == 0 || send(view, "window") == 0 || sendBool(view, "isHiddenOrHasHiddenAncestor") {
			return
		}
		in := msgRectToView(view, sel("convertRect:toView:"), msgRect(view, sel("bounds")), 0)
		content := msgRectToView(w.view, sel("convertRect:toView:"), msgRect(w.view, sel("bounds")), 0)
		top := content.Origin.Y + content.Size.Height
		r = platform.Rect{X: int(in.Origin.X - content.Origin.X), Y: int(top - in.Origin.Y - in.Size.Height),
			Width: int(in.Size.Width), Height: int(in.Size.Height)}
		ok = r.Width > 0 && r.Height > 0
	})
	return r, ok
}

// viewOfItem returns the view under root that shows a toolbar item, as
// its viewer tells, or 0.
func viewOfItem(root, item id) id {
	if root == 0 {
		return 0
	}
	if respondsTo(root, "item") && send(root, "item") == item {
		return root
	}
	for _, v := range arrayItems(send(root, "subviews")) {
		if found := viewOfItem(v, item); found != 0 {
			return found
		}
	}
	return 0
}
