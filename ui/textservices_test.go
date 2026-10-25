package ui

import "testing"

func TestTextServicesOfInput(t *testing.T) {
	text := "hello world"
	tt := coreNewTester(func(c *context) {
		coreTextInput(c, &text).Width(200).Label("name").AutoFocus()
	}, 300, 100)
	ts := textServices{tt.rt}
	if got, editable := ts.ServicedText(); got != "" || !editable {
		t.Errorf("nothing selected, the services get %q, editable %v", got, editable)
	}
	tt.Command("selectAll")
	if got, editable := ts.ServicedText(); got != "hello world" || !editable {
		t.Errorf("all selected, the services get %q, editable %v", got, editable)
	}
	// What a service gives back takes the selection's place, as typed.
	ts.ReplaceServicedText("Hello, World!")
	tt.Frame()
	if text != "Hello, World!" {
		t.Errorf("the service's text left %q", text)
	}
}

func TestTextServicesOfPassword(t *testing.T) {
	text := "secret"
	tt := coreNewTester(func(c *context) {
		coreTextInput(c, &text).Password().Width(200).Label("password").AutoFocus()
	}, 300, 100)
	tt.Command("selectAll")
	if got, _ := (textServices{tt.rt}).ServicedText(); got != "" {
		t.Errorf("a password gives the services %q", got)
	}
}

func TestTextServicesOfSelectableText(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Padding(12).Gap(8).Children(func() {
			coreText(c, "Alpha beta")
			coreText(c, "Gamma delta")
		})
	}, 300, 150)
	x, y := selectableAt(t, tt, "Alpha beta", 6)
	tt.Press(x, y)
	x, y = selectableAt(t, tt, "Gamma delta", 5)
	tt.Move(x, y)
	tt.Release(x, y)
	ts := textServices{tt.rt}
	if got, editable := ts.ServicedText(); got != "beta\nGamma" || editable {
		t.Errorf("the services get %q, editable %v, of the selection", got, editable)
	}
	// Nothing takes the place of text that cannot be edited.
	ts.ReplaceServicedText("x")
	if got, _ := ts.ServicedText(); got != "beta\nGamma" {
		t.Errorf("replacing selectable text left %q selected", got)
	}
}

func TestTextMenusAreMarked(t *testing.T) {
	text := "hello"
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(8).Children(func() {
			coreTextInput(c, &text).Width(200).Label("name")
			coreButton(c, "Sort").Menu(func(m *Menu) { m.Item("Name") })
			coreButton(c, "Row").ContextMenu(func(m *Menu) { m.Item("Rename") })
		})
	}, 300, 200)
	for _, c := range []struct {
		name string
		open func() error
		text bool
	}{
		{"a text input's", func() error { return tt.RightClick("name") }, true},
		{"a menu button's", func() error { return tt.Click("Sort") }, false},
		{"another element's", func() error { return tt.RightClick("Row") }, false},
	} {
		tt.h.menu = nil
		if err := c.open(); err != nil {
			t.Fatal(err)
		}
		if tt.h.menu == nil {
			t.Fatalf("%s menu did not open", c.name)
		}
		if tt.h.menu.Text != c.text {
			t.Errorf("%s menu is marked of text %v, want %v", c.name, tt.h.menu.Text, c.text)
		}
		tt.Key(0, KeyEscape)
	}
}
