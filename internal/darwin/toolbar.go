//go:build darwin

package darwin

import (
	"slices"
	"sync"

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
	tb, delegate, split id
	// spec is the toolbar SetToolbar gave last, and items the items in
	// the toolbar by ID, with the button of an activity and of one
	// showing a symbol and a title in buttons.
	spec *platform.Toolbar
	// sidebars are the backgrounds behind the columns that have one
	// (ToolbarColumn.Sidebar), by column.
	sidebars map[int]id
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
		var titles []id
		for _, s := range it.Segments {
			titles = append(titles, nsString(s))
		}
		item = send(class("NSToolbarItemGroup"), "groupWithItemIdentifier:titles:selectionMode:labels:target:action:",
			uintptr(ident), uintptr(nsArray(titles...)), 0, uintptr(nsArray(titles...)), uintptr(t.delegate), action)
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
			t = &toolbar{items: map[string]id{}, buttons: map[string]id{}, sidebars: map[int]id{}}
			w.toolbar = t
			t.delegate = send(alloc("MyGoToolbarDelegate"), "init")
			toolbarOwners[t.delegate] = w
		}
		t.activated = activated
		// The split view first, which the separators track as they are
		// made.
		w.placeColumns(spec.Columns)
		spec = hideCollapsed(spec)
		was := t.spec
		t.spec = spec
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

// placeColumns lays the split view behind the content out as columns,
// making it as the first ones come.
func (w *window) placeColumns(columns []platform.ToolbarColumn) {
	t := w.toolbar
	if len(columns) < 2 {
		return
	}
	if t.split == 0 {
		bounds := msgRect(w.view, sel("bounds"))
		t.split = msgInitRect(alloc("NSSplitView"), sel("initWithFrame:"), bounds)
		send(t.split, "setVertical:", 1)
		send(t.split, "setDividerStyle:", 2)        // NSSplitViewDividerStyleThin
		send(t.split, "setAutoresizingMask:", 2|16) // width and height sizable
		msgSetFloat(t.split, sel("setAlphaValue:"), 0)
		send(w.view, "addSubview:positioned:relativeTo:", uintptr(t.split), uintptr(^uint(0)), 0) // NSWindowBelow
	}
	views := arrayItems(send(t.split, "subviews"))
	for len(views) < len(columns) {
		v := autorelease(msgInitRect(alloc("NSView"), sel("initWithFrame:"), NSRect{}))
		send(t.split, "addSubview:", uintptr(v))
		views = append(views, v)
	}
	thickness := msgFloat(t.split, sel("dividerThickness"))
	// The dividers of the split view take room between its columns, and
	// those of the content none: the backgrounds go where the content's
	// columns are.
	x, at := 0.0, 0.0
	room := toolbarRoom(columns)
	for i, c := range columns {
		send(views[i], "setHidden:", boolArg(c.Collapsed))
		w.placeSidebar(i, c, at)
		if !c.Collapsed {
			x += room[i]
			at += c.Width
		}
		if i < len(columns)-1 {
			msgSetPosition(t.split, sel("setPosition:ofDividerAtIndex:"), x, i)
			if !c.Collapsed {
				x += thickness
			}
		}
	}
}

// minToolbarColumn is the least room the split view behind the content
// gives a column that shows. AppKit lays out the toolbar of a column a few
// points wide as that of one it collapsed, its items jumping, and keeps
// the room of a column's items anyway: a column narrowing, as it slides
// away, leaves its items where they have the room, as Xcode's do, while
// its sidebar goes on.
const minToolbarColumn = 40

// toolbarRoom returns the widths of the columns in the split view behind
// the content: theirs, at least minToolbarColumn where they show, the
// widest giving the room the others take more.
func toolbarRoom(columns []platform.ToolbarColumn) []float64 {
	room := make([]float64, len(columns))
	widest, more := -1, 0.0
	for i, c := range columns {
		if c.Collapsed {
			continue
		}
		room[i] = max(c.Width, minToolbarColumn)
		more += room[i] - c.Width
		if widest < 0 || c.Width > columns[widest].Width {
			widest = i
		}
	}
	if widest >= 0 {
		room[widest] = max(room[widest]-more, minToolbarColumn)
	}
	return room
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
		// Liquid Glass, as AppKit's split views put their sidebars on
		// macOS 26, over the window's background (backdrop); the sidebar
		// material before. The glass rounds all of its corners alike,
		// whichever corners its layer masks, so it is square in a view
		// that rounds the window's corners alone (v).
		var glass id
		inner := NSRect{Size: frame.Size}
		if hasClass("NSGlassEffectView") {
			glass = msgInitRect(alloc("NSGlassEffectView"), sel("initWithFrame:"), inner)
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
			w.placeBackdrop()
		} else {
			glass = msgInitRect(alloc("NSVisualEffectView"), sel("initWithFrame:"), inner)
			send(glass, "setMaterial:", vibrancyMaterials["sidebar"])
			send(glass, "setBlendingMode:", 0) // NSVisualEffectBlendingModeBehindWindow
		}
		send(glass, "setAutoresizingMask:", nsViewWidthHeightSizable)
		v = msgInitRect(alloc("NSView"), sel("initWithFrame:"), frame)
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
	if t.split != 0 {
		send(t.split, "removeFromSuperview")
		release(t.split)
	}
	for _, v := range t.sidebars {
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
