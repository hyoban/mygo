//go:build darwin

package darwin

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A source list (platform.SourceListSurface) is an NSOutlineView in the
// sidebar style, in an NSScrollView in a clip view under the surface, as
// a web view in a window of native UI is: the surface's frames are
// transparent where the content shows it, and its hitTest: lets the
// pointer through to it there. Its delegate is its data source too, which
// asks the handler for the items' children and rows as the outline shows
// them; its items are NSNumbers of their numbers, one each, which it
// keeps, as an outline view keeps none of its items.
type sourceList struct {
	embed
	h        platform.SourceListHandler
	scroll   id
	outline  id // MyGoSourceListView
	delegate id // MyGoSourceListDelegate
	// objects are the NSNumbers of the items, kids the children the
	// handler gave of each item asked, and rows the rows it gave.
	objects map[uint64]id
	kids    map[uint64][]uint64
	rows    map[uint64]platform.SourceListItem
	// syncing is set while the content changes the list, which tells its
	// handler nothing of what follows.
	syncing bool
	// menuItem is the item whose context menu shows, of owner's items.
	menuItem  uint64
	menuOwner int
	style     platform.SourceListStyle
}

var (
	sourceLists     = map[id]*sourceList{} // by outline and by delegate
	sourceCells     = map[id]*sourceCell{}
	sourceImages    = map[*byte]id{}
	sourceListOnce  sync.Once
	msgRowsInRect   func(obj id, s objc.SEL, r NSRect) nsRange
	msgRowAtPoint   func(obj id, s objc.SEL, p NSPoint) int
	msgFontWithSize func(cls id, s objc.SEL, size, weight float64) id
)

// sourceCell is a row's view: its icon, its title and its badges, and the
// row it shows.
type sourceCell struct {
	image, title, subtitle id
	spinner                id // made as a row is first busy
	badges                 []id
	item                   platform.SourceListItem
	list                   *sourceList
}

func (s *surface) NewSourceList(h platform.SourceListHandler) platform.SourceList {
	sourceListOnce.Do(func() {
		purego.RegisterFunc(&msgRowsInRect, msgSendAddr)
		purego.RegisterFunc(&msgRowAtPoint, msgSendAddr)
		purego.RegisterFunc(&msgFontWithSize, msgSendAddr)
	})
	l := &sourceList{h: h, objects: map[uint64]id{}, kids: map[uint64][]uint64{}, rows: map[uint64]platform.SourceListItem{}}
	withPool(func() {
		s.newEmbed(l)
		l.scroll = msgInitRect(send(class("NSScrollView"), "alloc"), sel("initWithFrame:"), NSRect{})
		send(l.scroll, "setHasVerticalScroller:", 1)
		send(l.scroll, "setAutohidesScrollers:", 1)
		send(l.scroll, "setDrawsBackground:", 0)
		send(l.scroll, "setBorderType:", 0) // NSNoBorder
		if respondsTo(l.scroll, "setAutomaticallyAdjustsContentInsets:") {
			send(l.scroll, "setAutomaticallyAdjustsContentInsets:", 0)
		}

		l.outline = msgInitRect(send(class("MyGoSourceListView"), "alloc"), sel("initWithFrame:"), NSRect{})
		column := autorelease(send(send(class("NSTableColumn"), "alloc"), "initWithIdentifier:", uintptr(nsString("item"))))
		send(column, "setResizingMask:", 1) // NSTableColumnAutoresizingMask
		send(l.outline, "addTableColumn:", uintptr(column))
		send(l.outline, "setOutlineTableColumn:", uintptr(column))
		send(l.outline, "setHeaderView:", 0)
		if respondsTo(l.outline, "setStyle:") {
			send(l.outline, "setStyle:", 3) // NSTableViewStyleSourceList
		} else {
			send(l.outline, "setSelectionHighlightStyle:", 1) // NSTableViewSelectionHighlightStyleSourceList
		}
		send(l.outline, "setRowSizeStyle:", ^uintptr(0)) // NSTableViewRowSizeStyleDefault, the sidebar's size of the system's settings
		send(l.outline, "setBackgroundColor:", uintptr(send(class("NSColor"), "clearColor")))
		send(l.outline, "setFloatsGroupRows:", 0)
		send(l.outline, "setAllowsEmptySelection:", 1)
		send(l.outline, "setAllowsMultipleSelection:", 0)
		send(l.outline, "setColumnAutoresizingStyle:", 1) // NSTableViewUniformColumnAutoresizingStyle
		send(l.outline, "setAutoresizesOutlineColumn:", 0)
		send(l.outline, "setFocusRingType:", 1) // NSFocusRingTypeNone, as sidebars
		l.delegate = alloc("MyGoSourceListDelegate")
		send(l.outline, "setDataSource:", uintptr(l.delegate))
		send(l.outline, "setDelegate:", uintptr(l.delegate))
		send(l.outline, "setTarget:", uintptr(l.delegate))
		send(l.outline, "setAction:", uintptr(sel("mygoSourceListClicked:")))

		send(l.scroll, "setDocumentView:", uintptr(l.outline))
		send(l.clip, "addSubview:", uintptr(l.scroll))
		l.focusView = l.outline
	})
	sourceLists[l.outline], sourceLists[l.delegate] = l, l
	return l
}

// placed puts the scroll view at frame in the clip view.
func (l *sourceList) placed(frame NSRect) {
	if msgRect(l.scroll, sel("frame")) != frame {
		msgSetRect(l.scroll, sel("setFrame:"), frame)
		send(l.outline, "sizeLastColumnToFit")
	}
}

// object returns the NSNumber of item, nil for 0, the top.
func (l *sourceList) object(item uint64) id {
	if item == 0 {
		return 0
	}
	o := l.objects[item]
	if o == 0 {
		o = retain(send(class("NSNumber"), "numberWithUnsignedLongLong:", uintptr(item)))
		l.objects[item] = o
	}
	return o
}

// itemOf returns the item of the NSNumber o, 0 for nil.
func itemOf(o id) uint64 {
	if o == 0 {
		return 0
	}
	return uint64(send(o, "unsignedLongLongValue"))
}

// row returns the row of item, as the handler last gave it.
func (l *sourceList) row(item uint64) platform.SourceListItem {
	r, ok := l.rows[item]
	if !ok {
		r = l.h.Item(item)
		l.rows[item] = r
	}
	return r
}

func (l *sourceList) Reload(item uint64) {
	if l.closed {
		return
	}
	l.syncing = true
	defer func() { l.syncing = false }()
	withPool(func() {
		if item == 0 {
			clear(l.kids)
			clear(l.rows)
			send(l.outline, "reloadData")
			return
		}
		l.forget(item)
		send(l.outline, "reloadItem:reloadChildren:", uintptr(l.object(item)), 1)
	})
}

// forget forgets the children and the rows of what is inside item.
func (l *sourceList) forget(item uint64) {
	for _, k := range l.kids[item] {
		delete(l.rows, k)
		l.forget(k)
	}
	delete(l.kids, item)
	delete(l.rows, item)
}

func (l *sourceList) Expand(item uint64, open, animate bool) {
	if l.closed || item == 0 {
		return
	}
	l.syncing = true
	defer func() { l.syncing = false }()
	withPool(func() {
		// The outline view's animator slides the rows as a click on the
		// arrow does.
		target := l.outline
		if animate {
			target = send(l.outline, "animator")
		}
		if open {
			send(target, "expandItem:", uintptr(l.object(item)))
		} else {
			send(target, "collapseItem:", uintptr(l.object(item)))
		}
	})
}

func (l *sourceList) Expanded(item uint64) bool {
	return !l.closed && item != 0 && sendBool(l.outline, "isItemExpanded:", uintptr(l.object(item)))
}

func (l *sourceList) Refresh(items []uint64, rows []platform.SourceListItem) {
	if l.closed {
		return
	}
	withPool(func() {
		for i, item := range items {
			was, known := l.rows[item]
			l.rows[item] = rows[i]
			row := sendInt(l.outline, "rowForItem:", uintptr(l.object(item)))
			if row < 0 {
				continue
			}
			if known && was.Group != rows[i].Group {
				l.syncing = true
				send(l.outline, "reloadItem:", uintptr(l.object(item)))
				l.syncing = false
				continue
			}
			v := send(l.outline, "viewAtColumn:row:makeIfNecessary:", 0, uintptr(row), 0)
			if c := sourceCells[v]; c != nil {
				c.show(v, rows[i])
			}
		}
	})
}

func (l *sourceList) Visible() []uint64 {
	if l.closed {
		return nil
	}
	r := msgRowsInRect(l.outline, sel("rowsInRect:"), msgRect(l.outline, sel("visibleRect")))
	items := make([]uint64, 0, r.Length)
	for i := range r.Length {
		items = append(items, itemOf(send(l.outline, "itemAtRow:", uintptr(r.Location+i))))
	}
	return items
}

func (l *sourceList) Select(item uint64) {
	if l.closed {
		return
	}
	l.syncing = true
	defer func() { l.syncing = false }()
	withPool(func() {
		row := -1
		if item != 0 {
			row = sendInt(l.outline, "rowForItem:", uintptr(l.object(item)))
		}
		if row < 0 {
			send(l.outline, "deselectAll:", 0)
			return
		}
		if sendInt(l.outline, "selectedRow") != row {
			send(l.outline, "selectRowIndexes:byExtendingSelection:", uintptr(send(class("NSIndexSet"), "indexSetWithIndex:", uintptr(row))), 0)
		}
	})
}

func (l *sourceList) ScrollTo(item uint64) {
	if l.closed || item == 0 {
		return
	}
	if row := sendInt(l.outline, "rowForItem:", uintptr(l.object(item))); row >= 0 {
		send(l.outline, "scrollRowToVisible:", uintptr(row))
	}
}

func (l *sourceList) Focus()            { l.focus() }
func (l *sourceList) TabInto(back bool) { l.focus() }
func (l *sourceList) HasFocus() bool    { return l.hasFocus() }

func (l *sourceList) SetStyle(style platform.SourceListStyle) {
	if l.closed || l.style == style {
		return
	}
	l.style = style
	withPool(func() {
		if respondsTo(l.outline, "setStyle:") {
			st := uintptr(3) // NSTableViewStyleSourceList
			if style.Inset {
				st = 2 // NSTableViewStyleInset
			}
			send(l.outline, "setStyle:", st)
		}
		// The style gives the list a background of its own: none, for
		// what is under it to show, as before.
		send(l.outline, "setBackgroundColor:", uintptr(send(class("NSColor"), "clearColor")))
		send(l.scroll, "setDrawsBackground:", 0)
		if style.RowHeight > 0 {
			send(l.outline, "setRowSizeStyle:", 0) // NSTableViewRowSizeStyleCustom
			msgSetFloat(l.outline, sel("setRowHeight:"), style.RowHeight)
		} else {
			send(l.outline, "setRowSizeStyle:", ^uintptr(0))
		}
	})
}

func (l *sourceList) Close() {
	if l.closed {
		return
	}
	withPool(func() {
		l.closeEmbed(l)
		send(l.outline, "setDataSource:", 0)
		send(l.outline, "setDelegate:", 0)
		send(l.outline, "setTarget:", 0)
		for v, c := range sourceCells {
			if c.list == l {
				delete(sourceCells, v)
				release(v)
			}
		}
		for _, o := range l.objects {
			release(o)
		}
		if l.menuOwner != 0 {
			dropOwner(l.menuOwner)
		}
		delete(sourceLists, l.outline)
		delete(sourceLists, l.delegate)
		release(l.outline)
		release(l.scroll)
		autorelease(l.delegate)
	})
}

// menu returns the context menu of the row at the mouse event ev, nil for
// none: the handler's, whose items tell it what is chosen.
func (l *sourceList) menu(ev id) id {
	p := msgPointFromView(l.outline, sel("convertPoint:fromView:"), msgPoint(ev, sel("locationInWindow")), 0)
	row := msgRowAtPoint(l.outline, sel("rowAtPoint:"), p)
	if row < 0 {
		return 0
	}
	item := itemOf(send(l.outline, "itemAtRow:", uintptr(row)))
	m := l.h.Menu(item)
	if m == nil || len(m.Items) == 0 {
		return 0
	}
	if l.menuOwner != 0 {
		dropOwner(l.menuOwner)
	}
	l.menuOwner, l.menuItem = newOwner(), item
	menu := theBackend.buildMenu(m, "", l.menuOwner)
	l.target(menu)
	return menu
}

// target sends the items of menu that tell the app to the list's
// delegate, which tells the handler.
func (l *sourceList) target(menu id) {
	for _, it := range arrayItems(send(menu, "itemArray")) {
		if sub := send(it, "submenu"); sub != 0 {
			l.target(sub)
		}
		if objc.SEL(send(it, "action")) == sel("mygoMenuItemClicked:") {
			send(it, "setTarget:", uintptr(l.delegate))
			send(it, "setAction:", uintptr(sel("mygoSourceListMenu:")))
		}
	}
}

// rowSize returns the size of the text and of the icons of the rows, as
// the sidebar's size in the system's settings asks.
func (l *sourceList) rowSize() (text, icon float64) {
	switch sendInt(l.outline, "effectiveRowSizeStyle") {
	case 1: // NSTableViewRowSizeStyleSmall
		return 11, 14
	case 3: // NSTableViewRowSizeStyleLarge
		return 15, 20
	}
	return 13, 16
}

// cell returns a view showing row, one the outline no longer shows where
// it has one.
func (l *sourceList) cell(row platform.SourceListItem) id {
	ident := "MyGoSourceListCell"
	if row.Group {
		ident = "MyGoSourceListGroup"
	}
	v := send(l.outline, "makeViewWithIdentifier:owner:", uintptr(nsString(ident)), 0)
	c := sourceCells[v]
	if v == 0 || c == nil {
		v = msgInitRect(send(class("MyGoSourceListCell"), "alloc"), sel("initWithFrame:"), NSRect{})
		send(v, "setIdentifier:", uintptr(nsString(ident)))
		c = &sourceCell{list: l}
		c.image = autorelease(msgInitRect(send(class("NSImageView"), "alloc"), sel("initWithFrame:"), NSRect{}))
		send(c.image, "setImageScaling:", 3) // NSImageScaleProportionallyUpOrDown
		send(v, "addSubview:", uintptr(c.image))
		send(v, "setImageView:", uintptr(c.image))
		c.title = send(class("NSTextField"), "labelWithString:", uintptr(nsString("")))
		send(c.title, "setLineBreakMode:", 4) // NSLineBreakByTruncatingTail
		send(v, "addSubview:", uintptr(c.title))
		send(v, "setTextField:", uintptr(c.title))
		c.subtitle = send(class("NSTextField"), "labelWithString:", uintptr(nsString("")))
		send(c.subtitle, "setLineBreakMode:", 4)
		send(c.subtitle, "setFont:", uintptr(msgFloatID(class("NSFont"), sel("systemFontOfSize:"), 11)))
		send(c.subtitle, "setTextColor:", uintptr(send(class("NSColor"), "secondaryLabelColor")))
		send(v, "addSubview:", uintptr(c.subtitle))
		sourceCells[v] = c // kept until the list closes
	}
	c.show(v, row)
	return v
}

// show makes the cell v show row.
func (c *sourceCell) show(v id, row platform.SourceListItem) {
	c.item = row
	text, _ := c.list.rowSize()
	send(c.title, "setStringValue:", uintptr(nsString(row.Title)))
	nsColorClass := class("NSColor")
	switch {
	case row.Group:
		send(c.title, "setFont:", uintptr(msgFontWithSize(class("NSFont"), sel("systemFontOfSize:weight:"), 11, 0.3)))
		send(c.title, "setTextColor:", uintptr(send(nsColorClass, "secondaryLabelColor")))
	default:
		weight := 0.0
		if row.Bold {
			weight = 0.4 // NSFontWeightSemibold
		}
		send(c.title, "setFont:", uintptr(msgFontWithSize(class("NSFont"), sel("systemFontOfSize:weight:"), text, weight)))
		color := "labelColor"
		if row.Dimmed {
			color = "secondaryLabelColor"
		}
		send(c.title, "setTextColor:", uintptr(send(nsColorClass, color)))
	}
	if row.Tooltip != "" {
		send(v, "setToolTip:", uintptr(nsString(row.Tooltip)))
	} else {
		send(v, "setToolTip:", 0)
	}
	send(c.subtitle, "setStringValue:", uintptr(nsString(row.Subtitle)))
	send(c.subtitle, "setHidden:", boolArg(row.Subtitle == "" || row.Group))
	if row.Busy && c.spinner == 0 {
		c.spinner = autorelease(msgInitRect(send(class("NSProgressIndicator"), "alloc"), sel("initWithFrame:"), NSRect{}))
		send(c.spinner, "setStyle:", 1)       // NSProgressIndicatorStyleSpinning
		send(c.spinner, "setControlSize:", 1) // NSControlSizeSmall
		send(c.spinner, "setIndeterminate:", 1)
		send(c.spinner, "setDisplayedWhenStopped:", 0)
		send(v, "addSubview:", uintptr(c.spinner))
	}
	if c.spinner != 0 {
		if row.Busy {
			send(c.spinner, "startAnimation:", 0)
		} else {
			send(c.spinner, "stopAnimation:", 0)
		}
	}
	send(c.image, "setImage:", uintptr(c.list.image(row)))
	send(c.image, "setHidden:", boolArg(row.Group || row.Symbol == "" && row.Image == nil))
	for len(c.badges) < len(row.Badges) {
		b := send(class("NSTextField"), "labelWithString:", uintptr(nsString("")))
		send(v, "addSubview:", uintptr(b))
		c.badges = append(c.badges, b)
	}
	for i, b := range c.badges {
		send(b, "setHidden:", boolArg(i >= len(row.Badges)))
		if i >= len(row.Badges) {
			continue
		}
		badge := row.Badges[i]
		send(b, "setStringValue:", uintptr(nsString(badge.Text)))
		var font id
		switch {
		case badge.Digits:
			weight := 0.0
			if badge.Bold {
				weight = 0.3
			}
			font = msgFontWithSize(class("NSFont"), sel("monospacedDigitSystemFontOfSize:weight:"), 11, weight)
		case badge.Bold:
			font = msgFontWithSize(class("NSFont"), sel("systemFontOfSize:weight:"), 11, 0.3)
		default:
			font = msgFloatID(class("NSFont"), sel("systemFontOfSize:"), 11)
		}
		send(b, "setFont:", uintptr(font))
	}
	c.color(int(send(v, "backgroundStyle")))
	send(v, "setNeedsLayout:", 1)
}

// color colors the badges and the icon of the cell as their row, white
// on the accent of a row chosen in a list with the keyboard (emphasized).
func (c *sourceCell) color(style int) {
	emphasized := style == 1 // NSBackgroundStyleEmphasized
	white := send(class("NSColor"), "alternateSelectedControlTextColor")
	for i, b := range c.badges {
		if i >= len(c.item.Badges) {
			break
		}
		color := send(class("NSColor"), "secondaryLabelColor")
		if col := c.item.Badges[i].Color; col.A != 0 {
			color = nsColor(col)
		}
		if emphasized {
			color = white
		}
		send(b, "setTextColor:", uintptr(color))
	}
	var tint id
	switch {
	case emphasized && (c.item.Symbol != "" || c.item.Template):
		tint = white
	case c.item.Tint.A != 0:
		tint = nsColor(c.item.Tint)
	case c.item.Symbol != "":
		tint = send(class("NSColor"), "controlAccentColor")
	case c.item.Template:
		tint = send(class("NSColor"), "secondaryLabelColor")
	}
	send(c.image, "setContentTintColor:", uintptr(tint))
}

// layout puts the icon of the cell v at its start, its badges at its end,
// and its title between, each in the middle of its height.
func (c *sourceCell) layout(v id) {
	b := msgRect(v, sel("bounds"))
	w, h := b.Size.Width, b.Size.Height
	_, icon := c.list.rowSize()
	if c.item.ImageSize > 0 {
		icon = c.item.ImageSize
	}
	x := 0.0
	if !sendBool(c.image, "isHidden") {
		// Clear of the arrow before the cell, as the cells of AppKit's
		// sidebar templates inset their icons.
		const inset = 5
		msgSetRect(c.image, sel("setFrame:"), NSRect{Origin: NSPoint{inset, (h - icon) / 2}, Size: NSSize{icon, icon}})
		x = inset + icon + 5
	}
	right := w
	if c.spinner != 0 && c.item.Busy {
		const side = 16
		right -= side
		msgSetRect(c.spinner, sel("setFrame:"), NSRect{Origin: NSPoint{right, (h - side) / 2}, Size: NSSize{side, side}})
		right -= 4
	}
	for i := len(c.badges) - 1; i >= 0; i-- {
		badge := c.badges[i]
		if sendBool(badge, "isHidden") {
			continue
		}
		s := msgSize(badge, sel("fittingSize"))
		right -= s.Width
		msgSetRect(badge, sel("setFrame:"), NSRect{Origin: NSPoint{right, (h - s.Height) / 2}, Size: s})
		right -= 4
	}
	s := msgSize(c.title, sel("fittingSize"))
	width := max(right-x, 0)
	if sendBool(c.subtitle, "isHidden") {
		msgSetRect(c.title, sel("setFrame:"), NSRect{Origin: NSPoint{x, (h - s.Height) / 2}, Size: NSSize{width, s.Height}})
		return
	}
	// The title over the subtitle, the two in the middle of the row; its
	// bottom is at y 0, the cell not being flipped.
	sub := msgSize(c.subtitle, sel("fittingSize"))
	bottom := (h - s.Height - sub.Height) / 2
	msgSetRect(c.subtitle, sel("setFrame:"), NSRect{Origin: NSPoint{x, bottom}, Size: NSSize{width, sub.Height}})
	msgSetRect(c.title, sel("setFrame:"), NSRect{Origin: NSPoint{x, bottom + sub.Height}, Size: NSSize{width, s.Height}})
}

// image returns the icon of row: its symbol, or its image, which the
// list keeps for the rows showing it again.
func (l *sourceList) image(row platform.SourceListItem) id {
	_, size := l.rowSize()
	if row.ImageSize > 0 {
		size = row.ImageSize
	}
	if row.Symbol != "" {
		return symbol(row.Symbol, "")
	}
	if len(row.Image) == 0 {
		return 0
	}
	key := unsafe.SliceData(row.Image)
	img := sourceImages[key]
	if img == 0 {
		img = send(send(class("NSImage"), "alloc"), "initWithData:", uintptr(nsData(row.Image)))
		if img == 0 {
			return 0
		}
		send(img, "setTemplate:", boolArg(row.Template))
		sourceImages[key] = img // kept, as the rows show it again
	}
	msgSetSize(img, sel("setSize:"), NSSize{size, size})
	return img
}

func registerSourceListClasses() {
	press := func(button int) func(id, objc.SEL, id) {
		return func(self id, cmd objc.SEL, ev id) {
			if l := sourceLists[self]; l != nil {
				l.pressed(ev, button)
			}
			sendSuper(self, "MyGoSourceListView", cmd, uintptr(ev))
		}
	}
	classDef("MyGoSourceListView", "NSOutlineView", nil, []objc.MethodDef{
		method("mouseDown:", press(0)),
		method("rightMouseDown:", press(1)),
		method("otherMouseDown:", press(2)),
		method("acceptsFirstMouse:", func(self id, _ objc.SEL, ev id) bool { return true }),
		// The outline view draws the ring of the row whose menu it shows,
		// as AppKit's lists do, for the menu it has.
		method("menuForEvent:", func(self id, cmd objc.SEL, ev id) id {
			l := sourceLists[self]
			if l == nil || l.closed {
				return 0
			}
			menu := l.menu(ev)
			send(self, "setMenu:", uintptr(menu))
			if menu == 0 {
				return 0
			}
			return sendSuper(self, "MyGoSourceListView", cmd, uintptr(ev))
		}),
		// Tab leaves the list for the content, as it leaves a web view's
		// page. The keys the list has no use for, those with Command or
		// Control and Escape, go to the content, whose shortcuts they may
		// be, as AppKit passes them up the responder chain.
		method("keyDown:", func(self id, cmd objc.SEL, ev id) {
			l := sourceLists[self]
			const keyTab, keyEscape = 48, 53
			if l != nil && !l.closed {
				mods := eventMods(ev)
				switch code := sendInt(ev, "keyCode"); {
				case code == keyTab && mods&^platform.ModShift == 0:
					l.leave()
					l.h.TabOut(mods&platform.ModShift != 0)
					return
				case code == keyEscape || mods&(platform.ModSuper|platform.ModCtrl) != 0:
					send(l.s.view, "keyDown:", uintptr(ev))
					return
				}
			}
			sendSuper(self, "MyGoSourceListView", cmd, uintptr(ev))
		}),
		method("keyUp:", func(self id, cmd objc.SEL, ev id) {
			if l := sourceLists[self]; l != nil && !l.closed && eventMods(ev)&(platform.ModSuper|platform.ModCtrl) != 0 {
				send(l.s.view, "keyUp:", uintptr(ev))
				return
			}
			sendSuper(self, "MyGoSourceListView", cmd, uintptr(ev))
		}),
		method("accessibilityParent", func(self id, cmd objc.SEL) id {
			if l := sourceLists[self]; l != nil && l.accessParent != 0 {
				return l.accessParent
			}
			return sendSuper(self, "MyGoSourceListView", cmd)
		}),
	})
	classDef("MyGoSourceListCell", "NSTableCellView", nil, []objc.MethodDef{
		method("layout", func(self id, cmd objc.SEL) {
			sendSuper(self, "MyGoSourceListCell", cmd)
			if c := sourceCells[self]; c != nil {
				c.layout(self)
			}
		}),
		method("setBackgroundStyle:", func(self id, cmd objc.SEL, style int) {
			sendSuper(self, "MyGoSourceListCell", cmd, uintptr(style))
			if c := sourceCells[self]; c != nil {
				c.color(style)
			}
		}),
	})
	list := func(self id) *sourceList {
		if l := sourceLists[self]; l != nil && !l.closed {
			return l
		}
		return nil
	}
	notified := func(note id) uint64 {
		info := send(note, "userInfo")
		return itemOf(send(info, "objectForKey:", uintptr(nsString("NSObject"))))
	}
	classDef("MyGoSourceListDelegate", "NSObject", []string{"NSOutlineViewDataSource", "NSOutlineViewDelegate"}, []objc.MethodDef{
		method("outlineView:numberOfChildrenOfItem:", func(self id, _ objc.SEL, ov, item id) int {
			l := list(self)
			if l == nil {
				return 0
			}
			p := itemOf(item)
			kids := l.h.Children(p)
			l.kids[p] = kids
			return len(kids)
		}),
		method("outlineView:child:ofItem:", func(self id, _ objc.SEL, ov id, i int, item id) id {
			l := list(self)
			if l == nil {
				return 0
			}
			p := itemOf(item)
			kids, ok := l.kids[p]
			if !ok {
				kids = l.h.Children(p)
				l.kids[p] = kids
			}
			if i < 0 || i >= len(kids) {
				return 0
			}
			return l.object(kids[i])
		}),
		method("outlineView:isItemExpandable:", func(self id, _ objc.SEL, ov, item id) bool {
			l := list(self)
			return l != nil && l.h.Children(itemOf(item)) != nil
		}),
		method("outlineView:isGroupItem:", func(self id, _ objc.SEL, ov, item id) bool {
			l := list(self)
			return l != nil && l.row(itemOf(item)).Group
		}),
		method("outlineView:shouldSelectItem:", func(self id, _ objc.SEL, ov, item id) bool {
			l := list(self)
			return l != nil && !l.row(itemOf(item)).Group
		}),
		method("outlineView:viewForTableColumn:item:", func(self id, _ objc.SEL, ov, column, item id) id {
			l := list(self)
			if l == nil {
				return 0
			}
			k := itemOf(item)
			row := l.h.Item(k)
			l.rows[k] = row
			return l.cell(row)
		}),
		method("outlineView:typeSelectStringForTableColumn:item:", func(self id, _ objc.SEL, ov, column, item id) id {
			l := list(self)
			if l == nil {
				return 0
			}
			return nsString(l.row(itemOf(item)).Title)
		}),
		method("outlineViewSelectionDidChange:", func(self id, _ objc.SEL, note id) {
			l := list(self)
			if l == nil || l.syncing {
				return
			}
			item := uint64(0)
			if row := sendInt(l.outline, "selectedRow"); row >= 0 {
				item = itemOf(send(l.outline, "itemAtRow:", uintptr(row)))
			}
			l.h.Selected(item)
		}),
		method("outlineViewItemDidExpand:", func(self id, _ objc.SEL, note id) {
			if l := list(self); l != nil && !l.syncing {
				l.h.Expanded(notified(note), true)
			}
		}),
		method("outlineViewItemDidCollapse:", func(self id, _ objc.SEL, note id) {
			if l := list(self); l != nil && !l.syncing {
				l.h.Expanded(notified(note), false)
			}
		}),
		method("mygoSourceListClicked:", func(self id, _ objc.SEL, sender id) {
			l := list(self)
			if l == nil {
				return
			}
			if row := sendInt(l.outline, "clickedRow"); row >= 0 {
				l.h.Clicked(itemOf(send(l.outline, "itemAtRow:", uintptr(row))))
			}
		}),
		method("mygoSourceListMenu:", func(self id, _ objc.SEL, item id) {
			if l := list(self); l != nil {
				l.h.MenuChosen(l.menuItem, sendInt(item, "tag"))
			}
		}),
	})
}
