package ui

import (
	"bytes"
	"image"
	"image/png"
	"slices"
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

// SourceListItem is the row of an item of a SourceList: an icon, a title,
// and badges at its end.
type SourceListItem struct {
	Title string
	// Bold shows the title bold, as a message list's subjects.
	Bold bool
	// Subtitle is a second line under the title, muted: give the rows
	// room for it with OutlineState.RowHeight.
	Subtitle string
	// Symbol is the name of an SF Symbol shown before the title, in Tint,
	// or the accent color without one, as Finder's sidebar shows its
	// folders. Only macOS has them.
	Symbol string
	// Image is shown before the title in place of a symbol, as big as the
	// row's icons: draw it at twice their size, 32 pixels square for a row
	// of 16 points. With Template, it is a mask colored as Tint, or as
	// muted text without one, and as the text of a row chosen; otherwise
	// it keeps its colors. The list converts each image once: give the
	// same image, a pointer such as an *image.RGBA, for the rows showing
	// it.
	Image    image.Image
	Template bool
	Tint     Color
	// ImageSize is the side of the icon in DIPs, 0 for the row's own.
	ImageSize float32
	// Dimmed shows the title muted.
	Dimmed bool
	// Badges show at the end of the row: counts, a status.
	Badges []SourceListBadge
	// Busy shows a spinner at the end of the row, as of work under way.
	Busy    bool
	Tooltip string
	// Group makes the row the heading of a section, its items its
	// children, as the sidebars of Finder and Mail head theirs.
	Group bool
	// Menu builds the row's context menu, as Element.ContextMenu's build.
	Menu func(m *Menu)
	// OnClick runs when the row is clicked, after it is chosen.
	OnClick func()
}

// SourceListBadge is a short text at the end of a SourceList's row.
type SourceListBadge struct {
	Text string
	// Color is the text's, muted without one.
	Color Color
	Bold  bool
	// Digits shows digits of equal widths, as counts line up.
	Digits bool
}

// SourceList creates a list of items in a tree as a sidebar shows them:
// the system's own where it has one, AppKit's outline view in the source
// list style on macOS, and an Outline of the rows elsewhere. roots are the
// items at the top, children an item's children, nil for an item without
// any, and item describes an item's row. It takes s as an Outline does:
// s.Open holds the items open, s.List.Selected the row chosen,
// s.List.ScrollIntoView scrolls to a row, and s.List.Handle gives the
// list the keyboard.
//
// The system's list draws its rows, and handles the pointer and the
// keyboard while it has it, as the element does a web view's page (see
// WebView): it shows in the element's content box, through a hole in the
// window's content, under what paints after it, and Tab leaves it for the
// next element. It asks for the children and the rows of the items as it
// shows them, also between frames, as they open: children and item must
// only answer, and change nothing.
//
//	app.files.List.Selected = &app.file
//	ui.SourceList(c, &app.files, []string{""}, app.entries, func(p string) ui.SourceListItem {
//		return ui.SourceListItem{Title: path.Base(p), Symbol: "folder"}
//	}).Grow(1)
func coreSourceList[K comparable](c *context, s *OutlineState[K], roots []K, children func(K) []K, item func(K) SourceListItem) *node {
	defer s.noteChoice()
	rt := c.rt
	surface, _ := rt.host.nativeViews().(platform.SourceListSurface)
	if surface == nil {
		s.flatten(roots, children)
		s.followChoice()
		return drawnSourceList(c, s, roots, children, item)
	}
	b, _ := s.native.(*sourceList[K])
	if b == nil || b.rt != rt || b.n == nil {
		b = &sourceList[K]{rt: rt, s: s, ids: map[K]uint64{}, told: map[uint64][]uint64{}, shown: map[uint64]platform.SourceListItem{}, pngs: map[image.Image][]byte{}}
		b.n = surface.NewSourceList(b)
		s.native = b
	}
	b.roots, b.children, b.item = roots, children, item
	// What the user did with the list since the last frame, as an
	// Outline's rows tell it.
	clicks := b.clicks
	b.clicks = nil
	for _, id := range clicks {
		if f := item(b.key(id)).OnClick; f != nil {
			f()
		}
	}
	if m := b.menuChoice; m != nil {
		b.menuChoice = nil
		if build := item(b.key(m.item)).Menu; build != nil {
			build(&Menu{run: &menuRun{choice: m.id, label: m.label}, rt: rt})
		}
	}
	s.flatten(roots, children)
	s.followChoice()
	if b.chosen != nil {
		if sel := s.List.Selected; sel != nil {
			*sel = slices.IndexFunc(s.rows, func(r outlineRow[K]) bool { return b.ids[r.item] == *b.chosen })
		}
		b.chosen = nil
	}
	e := c.newElement(kindBox)
	e.native = b
	e.self = Stretch
	e.flags |= flagFocusable | flagOwnRing
	s.List.frame = listFrame{c: c, e: e, owner: e, s: &s.List, n: len(s.rows), frame: rt.frame, pass: rt.pass, tree: true}
	wrapElement(e).Bind(&s.List.Handle)
	b.id = e.id
	if !containsView(rt.lists, b) {
		rt.lists = append(rt.lists, b)
	}
	return e
}

// followChoice keeps the item chosen chosen as items open and close
// above it, by the list's arrows or the app, which moves its row: where
// the row chosen is the one the list showed chosen after the last frame,
// as nothing else chose another. A row the app chose, or rows that came
// and went otherwise, keep the row.
func (s *OutlineState[K]) followChoice() {
	sel, ch := s.List.Selected, &s.choice
	if sel == nil || !ch.ok || *sel != ch.row || s.sameOpen(ch.open) {
		return
	}
	if i := slices.IndexFunc(s.rows, func(r outlineRow[K]) bool { return r.item == ch.item }); i >= 0 {
		*sel = i
	}
}

// noteChoice notes the row chosen after the frame, its item, and the
// items open.
func (s *OutlineState[K]) noteChoice() {
	ch := &s.choice
	ch.ok = false
	if sel := s.List.Selected; sel != nil && *sel >= 0 && *sel < len(s.rows) {
		ch.row, ch.item, ch.ok = *sel, s.rows[*sel].item, true
	}
	if !s.sameOpen(ch.open) {
		ch.open = map[K]bool{}
		for k := range s.Open.All() {
			ch.open[k] = true
		}
	}
}

// sameOpen reports whether the items open are those of open.
func (s *OutlineState[K]) sameOpen(open map[K]bool) bool {
	if s.Open.Len() != len(open) {
		return false
	}
	for k := range s.Open.All() {
		if !open[k] {
			return false
		}
	}
	return true
}

// sourceList is the system's source list of a SourceList, and its
// handler: its items are the numbers of the SourceList's, from 1.
type sourceList[K comparable] struct {
	rt       *engine
	s        *OutlineState[K]
	n        platform.SourceList
	ids      map[K]uint64
	keys     []K
	roots    []K
	children func(K) []K
	item     func(K) SourceListItem
	// told are the children the list was given of each item asked, shown
	// the rows it was given, and pngs the images of the rows encoded.
	told  map[uint64][]uint64
	shown map[uint64]platform.SourceListItem
	pngs  map[image.Image][]byte
	// last are the items of the rows the list showed after the last
	// frame, selected the item it shows chosen.
	last     []uint64
	selected uint64
	synced   bool
	// id is the element showing the list, focused whether it had the
	// focus after the last frame.
	id      uint64
	focused bool
	// What the user did with the list, for the next frame: the item
	// chosen, the rows clicked, the item chosen from a row's menu.
	chosen     *uint64
	clicks     []uint64
	menuLabels []string
	menuChoice *listMenuChoice
}

type listMenuChoice struct {
	item  uint64
	id    int
	label string
}

func (b *sourceList[K]) native() platform.NativeView { return b.n }

// idOf returns the number of the item k.
func (b *sourceList[K]) idOf(k K) uint64 {
	id, ok := b.ids[k]
	if !ok {
		b.keys = append(b.keys, k)
		id = uint64(len(b.keys))
		b.ids[k] = id
	}
	return id
}

func (b *sourceList[K]) key(id uint64) K { return b.keys[id-1] }

// kids returns the children of the item id, the roots for 0, nil for an
// item without any.
func (b *sourceList[K]) kids(id uint64) []uint64 {
	var items []K
	if id == 0 {
		items = b.roots
	} else {
		items = b.children(b.key(id))
		if items == nil {
			return nil
		}
	}
	ids := make([]uint64, len(items))
	for i, k := range items {
		ids[i] = b.idOf(k)
	}
	return ids
}

// Children answers the list.
func (b *sourceList[K]) Children(id uint64) []uint64 {
	ids := b.kids(id)
	if ids != nil {
		b.told[id] = ids
	}
	return ids
}

func (b *sourceList[K]) Item(id uint64) platform.SourceListItem {
	r := b.row(id)
	b.shown[id] = r
	return r
}

// row returns the row of the item id, for the list.
func (b *sourceList[K]) row(id uint64) platform.SourceListItem {
	it := b.item(b.key(id))
	r := platform.SourceListItem{
		Title: it.Title, Bold: it.Bold, Subtitle: it.Subtitle, Symbol: it.Symbol, Template: it.Template,
		Tint: platformColor(it.Tint), ImageSize: float64(it.ImageSize), Dimmed: it.Dimmed, Busy: it.Busy,
		Tooltip: it.Tooltip, Group: it.Group,
	}
	if it.Image != nil {
		r.Image = b.png(it.Image)
	}
	for _, badge := range it.Badges {
		r.Badges = append(r.Badges, platform.SourceListBadge{Text: badge.Text, Color: platformColor(badge.Color), Bold: badge.Bold, Digits: badge.Digits})
	}
	return r
}

// png returns img encoded, once for each image.
func (b *sourceList[K]) png(img image.Image) []byte {
	if data, ok := b.pngs[img]; ok {
		return data
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) != nil {
		return nil
	}
	b.pngs[img] = buf.Bytes()
	return buf.Bytes()
}

func platformColor(c Color) platform.Color { return platform.Color{R: c.R, G: c.G, B: c.B, A: c.A} }

// sameRow reports whether two rows show alike: the image the same one.
func sameRow(a, b platform.SourceListItem) bool {
	return a.Title == b.Title && a.Bold == b.Bold && a.Subtitle == b.Subtitle && a.ImageSize == b.ImageSize && a.Busy == b.Busy && a.Symbol == b.Symbol && a.Template == b.Template && a.Tint == b.Tint &&
		a.Dimmed == b.Dimmed && a.Tooltip == b.Tooltip && a.Group == b.Group && slices.Equal(a.Badges, b.Badges) &&
		len(a.Image) == len(b.Image) && unsafe.SliceData(a.Image) == unsafe.SliceData(b.Image)
}

func (b *sourceList[K]) Selected(id uint64) {
	b.chosen, b.selected = &id, id
	b.rt.requestFrame()
}

func (b *sourceList[K]) Expanded(id uint64, open bool) {
	if open {
		b.s.Open.Add(b.key(id))
	} else {
		b.s.Open.Remove(b.key(id))
	}
	b.rt.requestFrame()
}

func (b *sourceList[K]) Clicked(id uint64) {
	b.clicks = append(b.clicks, id)
	b.rt.requestFrame()
}

func (b *sourceList[K]) Menu(id uint64) *platform.Menu {
	build := b.item(b.key(id)).Menu
	if build == nil {
		return nil
	}
	run := &menuRun{collect: true}
	pm := &platform.Menu{}
	build(&Menu{run: run, menu: pm, rt: b.rt})
	b.menuLabels = run.labels
	return pm
}

func (b *sourceList[K]) MenuChosen(id uint64, item int) {
	if item < 1 || item > len(b.menuLabels) {
		return
	}
	b.menuChoice = &listMenuChoice{item: id, id: item, label: b.menuLabels[item-1]}
	b.rt.requestFrame()
}

// TabOut moves the focus on from the list's element, as Tab left the
// list.
func (b *sourceList[K]) TabOut(back bool) {
	rt := b.rt
	rt.focused = b.id
	rt.moveFocus(back)
	b.focused = rt.focused == b.id
	rt.requestFrame()
}

// sync gives the list what the frame built: the items open and their
// children where they changed, the row chosen, the row to scroll to, the
// keyboard where the element took the focus, and the rows in view anew.
func (b *sourceList[K]) sync() {
	s := b.s
	b.n.SetStyle(platform.SourceListStyle{Inset: s.Inset, RowHeight: float64(s.RowHeight)})
	rows := make([]uint64, len(s.rows))
	for i, r := range s.rows {
		rows[i] = b.idOf(r.item)
	}
	reloaded := false
	if !b.synced || !slices.Equal(rows, b.last) {
		reloaded = b.reconcile()
		b.last, b.synced = rows, true
	}
	want := uint64(0)
	if sel := s.List.Selected; sel != nil && *sel >= 0 && *sel < len(rows) {
		want = rows[*sel]
	}
	if want != b.selected || reloaded {
		b.n.Select(want)
		b.selected = want
	}
	if r := s.List.req; r.set {
		if !r.end && r.row >= 0 && r.row < len(rows) {
			b.n.ScrollTo(rows[r.row])
		}
		s.List.req = listRequest{}
	}
	focused := b.rt.focused == b.id
	if focused && !b.focused {
		b.n.Focus()
	}
	b.focused = focused
	var items []uint64
	var shown []platform.SourceListItem
	for _, id := range b.n.Visible() {
		if id == 0 {
			continue
		}
		r := b.row(id)
		if was, ok := b.shown[id]; !ok || !sameRow(was, r) {
			b.shown[id] = r
			items, shown = append(items, id), append(shown, r)
		}
	}
	if len(items) > 0 {
		b.n.Refresh(items, shown)
	}
}

// reconcile has the list show the rows of the frame: it reloads the items
// whose children changed, all where those at the top did, and opens and
// closes those the frame opened and closed. It reports whether it
// reloaded all.
func (b *sourceList[K]) reconcile() bool {
	if t, ok := b.told[0]; !ok || !slices.Equal(t, b.kids(0)) {
		clear(b.told)
		clear(b.shown)
		b.n.Reload(0)
		b.expand(false)
		return true
	}
	for _, id := range slices.Collect(func(yield func(uint64) bool) {
		for id := range b.told {
			if id != 0 && !yield(id) {
				return
			}
		}
	}) {
		if t, ok := b.told[id]; ok && !slices.Equal(t, b.kids(id)) {
			delete(b.told, id)
			b.n.Reload(id)
		}
	}
	b.expand(true)
	return false
}

// expand opens and closes the items of the rows as the frame has them,
// sliding their rows where animate: not those of a list shown anew.
func (b *sourceList[K]) expand(animate bool) {
	b.n.Update(func() {
		for _, r := range b.s.rows {
			if !r.branch {
				continue
			}
			id := b.idOf(r.item)
			if open := b.s.Open.Has(r.item); b.n.Expanded(id) != open {
				b.n.Expand(id, open, animate)
			}
		}
	})
}

// drawnSourceList builds a SourceList as an Outline where the system has
// no list of its own.
func drawnSourceList[K comparable](c *context, s *OutlineState[K], roots []K, children func(K) []K, item func(K) SourceListItem) *node {
	t := c.theme
	bitmaps := map[image.Image]*Bitmap{}
	if m, ok := s.native.(map[image.Image]*Bitmap); ok {
		bitmaps = m
	}
	s.native = bitmaps
	// The row chosen is the one of the frame, as the system's list keeps
	// it: not the item it showed.
	s.List.Key = nil
	return outline(c, s, roots, children, func(k K) {
		it := item(k)
		side := float32(16)
		if it.ImageSize > 0 {
			side = it.ImageSize
		}
		r := coreRow(c).AlignItems(Center).Gap(t.Space(1.5))
		r.Grow(1).MinWidth(0)
		r.Children(func() {
			switch {
			case it.Group:
			case it.Image != nil:
				bm := bitmaps[it.Image]
				if bm == nil {
					bm = NewBitmap(it.Image)
					bitmaps[it.Image] = bm
				}
				coreImage(c, bm).Size(side, side).Shrink(0)
			case it.Symbol != "":
				// The room of a symbol, which only macOS has.
				coreBox(c).Size(side, side).Shrink(0)
			}
			var title *node
			lines := coreBox(c).Column().Grow(1).MinWidth(0).Gap(1)
			lines.Children(func() {
				title = coreText(c, it.Title).SingleLine().Ellipsis("…")
				if it.Subtitle != "" && !it.Group {
					coreText(c, it.Subtitle).FontSize(t.FontSize - 2).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
				}
			})
			switch {
			case it.Group:
				title.FontSize(t.FontSize - 2).FontWeight(600).TextColor(t.TextMuted)
			case it.Dimmed:
				title.TextColor(t.TextMuted)
			}
			if it.Bold {
				title.FontWeight(600)
			}
			for _, badge := range it.Badges {
				b := coreText(c, badge.Text).FontSize(t.FontSize - 2).SingleLine().Shrink(0)
				color := badge.Color
				if color == (Color{}) {
					color = t.TextMuted
				}
				b.TextColor(color)
				if badge.Bold {
					b.FontWeight(600)
				}
			}
			if it.Busy {
				coreSpinner(c).Size(12, 12).Shrink(0)
			}
		})
		if it.Tooltip != "" {
			r.Tooltip(it.Tooltip)
		}
		if it.Menu != nil {
			r.ContextMenu(it.Menu)
		}
		if it.OnClick != nil {
			wrapElement(r.KeepFocus()).OnClick(it.OnClick)
		}
	})
}
