package platform

// NativeView is a view of the system's that a surface shows under it, as
// it shows web views: the surface's frames are transparent where the
// content shows one (holes), and the pointer goes through to it there,
// where nothing the content painted over it takes the pointer. A press on
// one, which takes it, comes as WebViewPress.
type NativeView interface {
	// Focus gives the view the keyboard; TabInto gives it as Tab does,
	// from its end when back. Tab out of it comes to its handler.
	Focus()
	TabInto(back bool)
	// HasFocus reports whether the view has the keyboard.
	HasFocus() bool
	// Close destroys the view; its handler hears no more of it.
	Close()
}

// NativeViewSurface is a Surface that shows views of the system's under
// it: source lists (SourceListSurface) and search fields
// (SearchFieldSurface).
type NativeViewSurface interface {
	// PlaceNativeViews shows the views of the surface where the content's
	// last frame shows them, and hides the others.
	PlaceNativeViews(views []NativeViewPlacement)
}

// NativeViewPlacement is where the content of a surface shows a view of
// the system's, in DIPs relative to the surface, as WebViewPlacement is a
// web view's.
type NativeViewPlacement struct {
	View        NativeView
	Frame, Clip RectF
	Covers      []RectF
}

// At reports whether the pointer at x, y is the view's.
func (p *NativeViewPlacement) At(x, y float64) bool {
	w := WebViewPlacement{Clip: p.Clip, Covers: p.Covers}
	return w.At(x, y)
}
