//go:build darwin

package darwin

import "github.com/egoist/mygo/internal/platform"

// scrollEvent translates AppKit's deltas and NSEventPhase values into a
// surface event. The fingers' gesture and its momentum have separate phases.
func scrollEvent(dx, dy float64, precise bool, phase, momentumPhase int) platform.SurfaceEvent {
	const phaseEnded, phaseCancelled = 8, 16
	// Pixel deltas do not imply a phased gesture: mice and synthetic
	// events can have neither phase. Each is a complete scroll of its own,
	// with no later Ended event to release a rubber-band stretch.
	phases := phase | momentumPhase
	ended := phases == 0 || phases&(phaseEnded|phaseCancelled) != 0
	if !precise {
		// A mouse wheel scrolls by lines: 40 DIPs, as in browsers.
		dx, dy = dx*40, dy*40
	}
	return platform.SurfaceEvent{
		Kind: platform.PointerScroll, DX: -dx, DY: -dy,
		Precise: precise, Momentum: momentumPhase != 0, ScrollEnded: ended,
	}
}
