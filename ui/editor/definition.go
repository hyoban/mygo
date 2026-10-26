package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// definitionAt offers identifiers as links, without asking the app to
// resolve them on every pointer move. Check the actual glyph range: the
// nearest caret alone would offer links on punctuation and empty space.
func (ed *editor) definitionAt(x, y float32) (Range, bool) {
	if ed.regionAt(x, y) != regionText {
		return Range{}, false
	}
	cx, cy := ed.toContent(x, y)
	off := ed.offsetAt(cx, cy)
	v := &ed.view
	l := v.line(v.text.LineOf(off))
	ident := func(r rune) bool {
		return isWordRune(r) || unicode.IsMark(r) || strings.ContainsRune(ed.wordChars(), r)
	}
	a, b := off-l.start, off-l.start
	for a > 0 {
		r, size := utf8.DecodeLastRuneInString(l.text[:a])
		if !ident(r) {
			break
		}
		a -= size
	}
	for b < len(l.text) {
		r, size := utf8.DecodeRuneInString(l.text[b:])
		if !ident(r) {
			break
		}
		b += size
	}
	if b <= a {
		return Range{}, false
	}
	r, _ := utf8.DecodeRuneInString(l.text[a:])
	if !unicode.IsLetter(r) && r != '_' && r != '$' {
		return Range{}, false
	}
	switch kindAt(l.toks, a) {
	case TokenText, TokenType, TokenFunction, TokenVariable, TokenProperty, TokenConstant, TokenTag, TokenAttribute:
	default:
		return Range{}, false
	}
	top := ed.rowTop(v.text.LineOf(off))
	ri := int((cy - top) / float64(v.lineHeight))
	if cy < top || ri < 0 || ri >= len(l.rows) {
		return Range{}, false
	}
	for _, rc := range v.rangeRects(l, &l.rows[ri], l.toDisp(a, false), l.toDisp(b, false), nil) {
		if cx >= rc.X && cx < rc.X+rc.W {
			return Range{Start: l.start + a, End: l.start + b}, true
		}
	}
	return Range{}, false
}
