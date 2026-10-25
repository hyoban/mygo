package editor

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTextServicesOfEditor(t *testing.T) {
	ct := newEditorTest(t, "", "one [two] three")
	if got, editable := ct.tt.ServicedText(); got != "two" || !editable {
		t.Errorf("the services get %q, editable %v", got, editable)
	}
	ct.tt.ReplaceServicedText("2")
	if got := ct.show(); got != "one 2| three" {
		t.Errorf("the service's text left %q", got)
	}
}

func TestTextServicesOfReadOnlyEditor(t *testing.T) {
	st := &State{Buffer: NewBuffer("one two three"), ReadOnly: true}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, st).Fill().AutoFocus()
	}, 600, 400)
	st.SetSelections(Selection{Anchor: 4, Head: 7})
	tt.Frame()
	if got, editable := tt.ServicedText(); got != "two" || editable {
		t.Errorf("the services get %q, editable %v", got, editable)
	}
	tt.ReplaceServicedText("2")
	if got := st.Buffer.String(); got != "one two three" {
		t.Errorf("a service changed read-only text to %q", got)
	}
	// Its context menu is one of text, which the system adds to.
	tt.RightClickAt(100, 10)
	if tt.Menu() == nil || !tt.MenuOfText() {
		t.Errorf("the editor's menu %v is not marked of text", tt.Menu())
	}
}
