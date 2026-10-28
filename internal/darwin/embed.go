//go:build darwin

package darwin

import (
	"slices"

	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo/internal/platform"
)

// A view of the system's in a window of native UI (platform.NativeView:
// a source list, a search field) sits in a clip view under the surface,
// as a web view does: the surface's frames are transparent where the
// content shows it, and its hitTest: lets the pointer through to it there.
// Over a sidebar, the clip view is in the sidebar's material, as AppKit's
// sidebars hold their views, which makes their text vibrant.

// embedder is a view of the system's under a surface.
type embedder interface {
	platform.NativeView
	embedded() *embed
}

// embed is what the views of the system's under a surface share: the
// surface, the clip view (MyGoEmbedClip) holding the view, and the view
// that takes the keyboard, which assistive technology finds in
// accessParent, the element of the content showing it.
type embed struct {
	s            *surface
	clip         id
	focusView    id
	accessParent id
	closed       bool
}

func (e *embed) embedded() *embed { return e }

// newEmbed makes the clip view of a view of the system's under the
// surface, hidden until placed.
func (s *surface) newEmbed(v embedder) {
	e := v.embedded()
	e.s = s
	e.clip = msgInitRect(send(class("MyGoEmbedClip"), "alloc"), sel("initWithFrame:"), NSRect{})
	send(e.clip, "setAutoresizingMask:", nsViewMinYMargin)
	send(e.clip, "setHidden:", 1)
	send(s.w.view, "addSubview:positioned:relativeTo:", uintptr(e.clip), nsWindowBelow, uintptr(s.view))
	s.embeds = append(s.embeds, v)
}

// PlaceNativeViews shows the window's views of the system's where the
// content shows them, and hides the others.
func (s *surface) PlaceNativeViews(views []platform.NativeViewPlacement) {
	if s.w.closed {
		return
	}
	s.placedViews = append(s.placedViews[:0], views...)
	height := msgRect(s.w.view, sel("bounds")).Size.Height
	var shown []embedder
	withPool(func() {
		for _, p := range views {
			v, ok := p.View.(embedder)
			if !ok {
				continue
			}
			e := v.embedded()
			if e.s != s || e.closed {
				continue
			}
			e.place(p.Clip, p.Frame, height)
			if r, ok := v.(interface{ placed(frame NSRect) }); ok {
				r.placed(NSRect{Origin: NSPoint{p.Frame.X - p.Clip.X, p.Frame.Y - p.Clip.Y}, Size: NSSize{p.Frame.W, p.Frame.H}})
			}
			shown = append(shown, v)
		}
		for _, v := range s.embeds {
			e := v.embedded()
			if !slices.Contains(shown, v) && !sendBool(e.clip, "isHidden") {
				if v.HasFocus() {
					send(s.w.win, "makeFirstResponder:", uintptr(s.view))
				}
				send(e.clip, "setHidden:", 1)
			}
		}
	})
}

// place puts the clip view at c, in the surface, of a content view height
// high: in the sidebar's material where it shows over a sidebar, as
// AppKit's sidebars hold their views; under the surface elsewhere.
func (e *embed) place(c, f platform.RectF, height float64) {
	s := e.s
	clip := NSRect{Origin: NSPoint{c.X, height - c.Y - c.H}, Size: NSSize{c.W, c.H}}
	parent := s.materialUnder(clip)
	if parent == 0 {
		parent = s.w.view
	}
	if send(e.clip, "superview") != parent {
		focused := e.hasFocus()
		retain(e.clip)
		send(e.clip, "removeFromSuperview")
		if parent == s.w.view {
			send(parent, "addSubview:positioned:relativeTo:", uintptr(e.clip), nsWindowBelow, uintptr(s.view))
		} else {
			send(parent, "addSubview:", uintptr(e.clip))
		}
		release(e.clip)
		if focused {
			e.focus()
		}
	}
	if parent != s.w.view {
		clip = msgRectToView(s.w.view, sel("convertRect:toView:"), clip, parent)
	}
	msgSetRect(e.clip, sel("setFrame:"), clip)
	if sendBool(e.clip, "isHidden") {
		send(e.clip, "setHidden:", 0)
	}
}

// materialUnder returns the view of the material of the sidebar that r,
// in the content view, shows over whole, which holds what it shows: 0
// for none.
func (s *surface) materialUnder(r NSRect) id {
	t := s.w.toolbar
	if t == nil {
		return 0
	}
	for _, v := range t.sidebars {
		if v == 0 || sendBool(v, "isHidden") {
			continue
		}
		f := msgRect(v, sel("frame"))
		if r.Origin.X < f.Origin.X-0.5 || r.Origin.X+r.Size.Width > f.Origin.X+f.Size.Width+0.5 {
			continue
		}
		subviews := arrayItems(send(v, "subviews"))
		if len(subviews) == 0 {
			continue
		}
		material := subviews[0]
		// Before macOS 26 the sidebar is on a visual effect view
		// (sidebarGlass), and there is no class of glass to ask for.
		if !hasClass("NSGlassEffectView") || !sendBool(material, "isKindOfClass:", uintptr(class("NSGlassEffectView"))) {
			return material
		}
		// The glass holds what shows on it in its content view.
		content := send(material, "contentView")
		if content == 0 {
			content = autorelease(msgInitRect(send(class("NSView"), "alloc"), sel("initWithFrame:"), msgRect(material, sel("bounds"))))
			send(content, "setAutoresizingMask:", nsViewWidthHeightSizable)
			send(material, "setContentView:", uintptr(content))
		}
		return content
	}
	return 0
}

// nativeViewAt reports whether a view of the system's takes the pointer
// at x, y in the surface.
func (s *surface) nativeViewAt(x, y float64) bool {
	for i := range s.placedViews {
		if s.placedViews[i].At(x, y) {
			return true
		}
	}
	return false
}

// closeNativeViews closes the views of the system's of a window that
// closes.
func (s *surface) closeNativeViews() {
	for len(s.embeds) > 0 {
		s.embeds[len(s.embeds)-1].Close()
	}
}

// focus gives the view the keyboard.
func (e *embed) focus() {
	if !e.closed {
		send(e.s.w.win, "makeFirstResponder:", uintptr(e.focusView))
	}
}

// hasFocus reports whether the view, a view inside it, or the field
// editor editing it has the keyboard.
func (e *embed) hasFocus() bool {
	if e.closed {
		return false
	}
	r := send(e.s.w.win, "firstResponder")
	switch {
	case r == 0:
		return false
	case r == e.focusView:
		return true
	case sendBool(r, "isKindOfClass:", uintptr(class("NSText"))) && send(r, "delegate") == e.focusView:
		return true
	}
	return sendBool(r, "isKindOfClass:", uintptr(class("NSView"))) && sendBool(r, "isDescendantOf:", uintptr(e.focusView))
}

// pressed tells the content that a button went down on the view.
func (e *embed) pressed(ev id, button int) {
	if e.closed || e.s.w.closed {
		return
	}
	x, y := e.s.location(ev)
	e.s.send(platform.SurfaceEvent{Kind: platform.WebViewPress, X: x, Y: y, Button: button, Mods: eventMods(ev)})
}

// leave gives the content the keyboard back from the view, as Tab leaves
// it.
func (e *embed) leave() {
	send(e.s.w.win, "makeFirstResponder:", uintptr(e.s.view))
}

// closeEmbed takes the clip view out and the view out of the surface's.
func (e *embed) closeEmbed(v embedder) {
	if e.hasFocus() {
		e.leave()
	}
	e.closed = true
	send(e.clip, "removeFromSuperview")
	release(e.clip)
	s := e.s
	s.embeds = slices.DeleteFunc(s.embeds, func(x embedder) bool { return x == v })
	s.placedViews = slices.DeleteFunc(s.placedViews, func(p platform.NativeViewPlacement) bool { return p.View == platform.NativeView(v) })
}

func registerEmbedClasses() {
	classDef("MyGoEmbedClip", "NSView", nil, []objc.MethodDef{
		method("isFlipped", func(self id, _ objc.SEL) bool { return true }),
		// Assistive technology finds the view in the element of the
		// content showing it, not beside the content.
		method("accessibilityChildren", func(self id, _ objc.SEL) id { return nsArray() }),
	})
}
