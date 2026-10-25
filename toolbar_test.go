//go:build !js

package mygo

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestToolbar(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Width: 400, Height: 300})
	var clicked, chosen = 0, -1
	tb := &Toolbar{
		Items: []ToolbarItem{
			{ID: "sidebar", Symbol: "sidebar.left", Label: "Hide Sidebar", Click: func() { clicked++ }},
			{ID: "split", Kind: ToolbarSeparator},
			{ID: "mode", Kind: ToolbarSegmented, Segments: []string{"Changes", "Files"}, Selected: 1, Select: func(i int) { chosen = i }},
			{ID: "ship", Title: "Ship", Hidden: true},
		},
		Columns: []ToolbarColumn{{Width: 240}, {Width: 160, Collapsed: true}},
	}
	if !w.SetToolbar(tb) {
		t.Fatal("the fake backend shows no toolbar")
	}
	got := fw.Toolbar()
	if got == nil || len(got.Items) != 4 || got.Items[2].Kind != platform.ToolbarSegmented || got.Items[2].Selected != 1 ||
		len(got.Columns) != 2 || !got.Columns[1].Collapsed || got.Columns[0].Width != 240 {
		t.Fatalf("the backend got %+v", got)
	}
	onMain(func() {
		fw.ActivateToolbarItem("sidebar", -1)
		fw.ActivateToolbarItem("mode", 0)
		fw.ActivateToolbarItem("unknown", -1)
	})
	if clicked != 1 || chosen != 0 {
		t.Errorf("clicked %d times, chose %d", clicked, chosen)
	}
	if r, ok := w.ToolbarItemBounds("mode"); !ok || r.X != 80 {
		t.Errorf("the mode is at %+v, %v", r, ok)
	}
	if _, ok := w.ToolbarItemBounds("ship"); ok {
		t.Error("a hidden item has bounds")
	}
	if !w.SetToolbar(nil) || fw.Toolbar() != nil {
		t.Error("the toolbar stayed")
	}
}
