package editor

import (
	"bytes"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDefinitionLinkFeedback(t *testing.T) {
	ct := newEditorTest(t, "go", "func main() { target() } // target\n")
	calls := 0
	ct.st.OnDefinition = func(int) { calls++ }
	ct.tt.Frame()
	ed := &ct.st.ed
	at := strings.Index(ct.st.Buffer.String(), "target")
	cr, _ := ed.caretRect(at + 2)
	x, y := ed.textX+cr.X, cr.Y+cr.H/2
	ct.tt.Move(x, y)
	if ct.tt.Cursor() != ui.CursorText {
		t.Fatal("ordinary pointer became a link")
	}
	before := bytes.Clone(ct.tt.Image().Pix)
	ct.tt.HoldModifiers(ui.Cmd)
	if ed.definitionLink != (Range{at, at + 6}) || ct.tt.Cursor() != ui.CursorPointer {
		t.Fatalf("link %+v, cursor %v", ed.definitionLink, ct.tt.Cursor())
	}
	if bytes.Equal(before, ct.tt.Image().Pix) {
		t.Fatal("link feedback did not paint")
	}
	if calls != 0 {
		t.Fatal("hover resolved a definition")
	}
	ct.tt.HoldModifiers(0)
	if ed.definitionLink != (Range{}) || ct.tt.Cursor() != ui.CursorText {
		t.Fatal("release kept link")
	}
	ct.tt.HoldModifiers(ui.Cmd)
	ct.tt.Move(590, 300)
	if ed.definitionLink != (Range{}) {
		t.Fatal("empty space kept link")
	}
	ct.tt.Move(x, y)
	ct.tt.HoldModifiers(ui.Cmd)
	ct.st.OnDefinition = nil
	ct.tt.Frame()
	if ed.definitionLink != (Range{}) || ct.tt.Cursor() == ui.CursorPointer {
		t.Fatal("disabled callback kept link")
	}
	ct.st.OnDefinition = func(int) { calls++ }
	ct.tt.Frame()
	ct.tt.SetFocused(false)
	if ed.definitionLink != (Range{}) {
		t.Fatal("window blur kept link")
	}
}

func TestDefinitionLinkHitTesting(t *testing.T) {
	text := "const emoji = \"😀\"; const $名字 = call(42); // comment\n\t$名字();\n"
	ct := newEditorTest(t, "typescript", text)
	ct.st.OnDefinition = func(int) {}
	ct.st.SoftWrap = true
	ct.st.WrapColumn = 30
	ct.tt.Frame()
	ed := &ct.st.ed
	for _, tc := range []struct {
		word string
		link bool
	}{
		{"const", false}, {"emoji", true}, {"😀", false}, {"$名字", true}, {"call", true}, {"42", false}, {"comment", false},
	} {
		t.Run(tc.word, func(t *testing.T) {
			at := strings.Index(text, tc.word)
			cr, _ := ed.caretRect(at)
			ct.tt.Move(ed.textX+cr.X+1, cr.Y+cr.H/2)
			ct.tt.HoldModifiers(ui.Cmd)
			if got := ed.definitionLink.End > ed.definitionLink.Start; got != tc.link {
				t.Fatalf("link %+v, want %v", ed.definitionLink, tc.link)
			}
		})
	}
	// A tab-indented identifier after wrapping, then the punctuation after it.
	at := strings.LastIndex(text, "$名字")
	cr, _ := ed.caretRect(at)
	ct.tt.Move(ed.textX+cr.X+1, cr.Y+cr.H/2)
	ct.tt.HoldModifiers(ui.Cmd)
	if ed.definitionLink != (Range{at, at + len("$名字")}) {
		t.Fatalf("unicode link %+v", ed.definitionLink)
	}
	cr, _ = ed.caretRect(at + len("$名字"))
	ct.tt.Move(ed.textX+cr.X+1, cr.Y+cr.H/2)
	ct.tt.HoldModifiers(ui.Cmd)
	if ed.definitionLink != (Range{}) {
		t.Fatal("punctuation offered a link")
	}
	ct.tt.Move(2, cr.Y+cr.H/2)
	ct.tt.HoldModifiers(ui.Cmd)
	if ed.definitionLink != (Range{}) {
		t.Fatal("gutter offered a link")
	}
}
