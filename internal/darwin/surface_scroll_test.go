//go:build darwin

package darwin

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestScrollEvent(t *testing.T) {
	const began, stationary, changed, ended, cancelled, mayBegin = 1, 2, 4, 8, 16, 128
	for _, tc := range []struct {
		name                 string
		precise              bool
		phase, momentumPhase int
		momentum, end        bool
	}{
		{"precise without phases", true, 0, 0, false, true},
		{"wheel without phases", false, 0, 0, false, true},
		{"gesture may begin", true, mayBegin, 0, false, false},
		{"gesture began", true, began, 0, false, false},
		{"gesture stationary", true, stationary, 0, false, false},
		{"gesture changed", true, changed, 0, false, false},
		{"gesture ended", true, ended, 0, false, true},
		{"gesture cancelled", true, cancelled, 0, false, true},
		{"momentum began", true, 0, began, true, false},
		{"momentum changed", true, 0, changed, true, false},
		{"momentum ended", true, 0, ended, true, true},
		{"momentum cancelled", true, 0, cancelled, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev := scrollEvent(1.25, -2.5, tc.precise, tc.phase, tc.momentumPhase)
			if ev.Kind != platform.PointerScroll || ev.Precise != tc.precise || ev.Momentum != tc.momentum || ev.ScrollEnded != tc.end {
				t.Errorf("scroll event: kind %v, precise %v, momentum %v, ended %v; want kind %v, precise %v, momentum %v, ended %v",
					ev.Kind, ev.Precise, ev.Momentum, ev.ScrollEnded, platform.PointerScroll, tc.precise, tc.momentum, tc.end)
			}
			dx, dy := -1.25, 2.5
			if !tc.precise {
				dx, dy = dx*40, dy*40
			}
			if ev.DX != dx || ev.DY != dy {
				t.Errorf("deltas (%v, %v), want (%v, %v)", ev.DX, ev.DY, dx, dy)
			}
		})
	}
}
