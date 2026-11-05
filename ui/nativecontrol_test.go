package ui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// fakeControls is a surface showing controls of the system's, as AppKit
// would, for the tests.
type fakeControls struct {
	controls []*fakeControl
	placed   []platform.NativeViewPlacement
}

func (s *fakeControls) PlaceNativeViews(views []platform.NativeViewPlacement) {
	s.placed = append(s.placed[:0], views...)
}

func (s *fakeControls) NewControl(kind platform.ControlKind, h platform.ControlHandler) platform.Control {
	c := &fakeControl{kind: kind, h: h, enabled: true, selected: -1}
	s.controls = append(s.controls, c)
	return c
}

// shown returns the controls of kind not closed.
func (s *fakeControls) shown(kind platform.ControlKind) []*fakeControl {
	var cs []*fakeControl
	for _, c := range s.controls {
		if c.kind == kind && !c.closed {
			cs = append(cs, c)
		}
	}
	return cs
}

type fakeControl struct {
	kind                     platform.ControlKind
	h                        platform.ControlHandler
	title, text, placeholder string
	label                    string
	checked                  bool
	items                    []string
	selected                 int
	enabled, focus, closed   bool
}

func (c *fakeControl) SetCheckbox(title string, checked bool) { c.title, c.checked = title, checked }
func (c *fakeControl) SetItems(items []string, selected int) {
	c.items, c.selected = slices.Clone(items), selected
}
func (c *fakeControl) SetText(text, placeholder string) { c.text, c.placeholder = text, placeholder }
func (c *fakeControl) SetEnabled(enabled bool)          { c.enabled = enabled }
func (c *fakeControl) SetLabel(label string)            { c.label = label }
func (c *fakeControl) Checked() bool                    { return c.checked }
func (c *fakeControl) Selected() int                    { return c.selected }
func (c *fakeControl) Text() string                     { return c.text }
func (c *fakeControl) FittingSize() (float64, float64)  { return 80, 22 }
func (c *fakeControl) Focus()                           { c.focus = true }
func (c *fakeControl) TabInto(back bool)                { c.focus = true }
func (c *fakeControl) HasFocus() bool                   { return c.focus }
func (c *fakeControl) Close()                           { c.closed = true }

// nativeTester tests view in a window showing the system's controls.
func nativeTester(view func(c *context)) (*Tester, *fakeControls) {
	s := &fakeControls{}
	tt := coreNewTester(view, 400, 300)
	tt.h.native = s
	tt.Frame()
	return tt, s
}

func TestNativeCheckboxAndPopUp(t *testing.T) {
	sync, plan, show, disabled := false, "Pro", true, false
	var syncChanged, planChanged int
	tt, s := nativeTester(func(c *context) {
		coreColumn(c).Children(func() {
			if show {
				if coreNativeCheckbox(c, &sync, "Sync").Disabled(disabled).Changed() {
					syncChanged++
				}
			}
			if coreNativePopUpButton(c, &plan, []string{"Free", "Pro", "Team"}).Changed() {
				planChanged++
			}
		})
	})
	boxes, pops := s.shown(platform.ControlCheckbox), s.shown(platform.ControlPopUp)
	if len(boxes) != 1 || len(pops) != 1 {
		t.Fatalf("%d check boxes and %d pop-up buttons, not one of each", len(boxes), len(pops))
	}
	box, pop := boxes[0], pops[0]
	if box.title != "Sync" || box.checked || !box.enabled {
		t.Errorf("check box %+v", box)
	}
	if !slices.Equal(pop.items, []string{"Free", "Pro", "Team"}) || pop.selected != 1 {
		t.Errorf("pop-up button shows %q, %d chosen", pop.items, pop.selected)
	}
	if len(s.placed) != 2 {
		t.Errorf("%d controls placed, not 2", len(s.placed))
	}

	box.checked = true
	box.h.Changed()
	tt.Frame()
	if !sync || syncChanged != 1 {
		t.Errorf("clicking the check box: sync %v, Changed %v", sync, syncChanged)
	}
	pop.selected = 2
	pop.h.Changed()
	tt.Frame()
	if plan != "Team" || planChanged != 1 {
		t.Errorf("choosing Team: plan %q, Changed %v", plan, planChanged)
	}

	disabled = true
	tt.Frame()
	if box.enabled {
		t.Error("a disabled check box stays enabled")
	}
	show = false
	tt.Frame()
	if !box.closed {
		t.Error("the check box stays open once its element goes")
	}
}

func TestNativeSearchField(t *testing.T) {
	var state SearchFieldState
	query, disabled := "", false
	var keys []Key
	var changed int
	state.OnKey = func(k Key) bool { keys = append(keys, k); return k == KeyDown }
	tt, s := nativeTester(func(c *context) {
		coreColumn(c).Children(func() {
			if coreNativeSearchField(c, &state, &query).Disabled(disabled).Changed() {
				changed++
			}
			coreButton(c, "After").Label("after")
		})
	})
	fields := s.shown(platform.ControlSearchField)
	if len(fields) != 1 {
		t.Fatalf("%d search fields, not one", len(fields))
	}
	f := fields[0]
	if f.placeholder != "Search" || f.label != "" || !f.enabled {
		t.Errorf("search field %+v", f)
	}

	f.text = "main"
	f.h.Changed()
	tt.Frame()
	if query != "main" || changed != 1 {
		t.Errorf("typing: query %q, Changed %v", query, changed)
	}
	if !f.h.Key(platform.KeyDown) || f.h.Key(platform.KeyUp) || !slices.Equal(keys, []Key{KeyDown, KeyUp}) {
		t.Errorf("the keys passed on reach OnKey as %v", keys)
	}

	state.Focus()
	tt.Frame()
	if !f.focus {
		t.Error("Focus doesn't give the field the keyboard")
	}
	f.h.TabOut(false)
	tt.Frame()
	if !tt.Focused("after") {
		t.Error("Tab out of the field doesn't move on to the next element")
	}

	disabled = true
	tt.Frame()
	if f.enabled {
		t.Error("a disabled search field stays enabled")
	}
}
