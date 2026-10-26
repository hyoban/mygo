package ui

import "github.com/egoist/mygo/internal/platform"

// nativeView is a view of the system's an element shows (SourceList,
// NativeSearchField), as the engine places it and syncs it after a frame.
type nativeView interface {
	native() platform.NativeView
	sync()
}

// containsView reports whether views holds v.
func containsView(views []nativeView, v nativeView) bool {
	for _, x := range views {
		if x == v {
			return true
		}
	}
	return false
}
