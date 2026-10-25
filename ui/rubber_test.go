package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// appKitScroller is a tester of a container 200 DIPs tall scrolling 1000
// DIPs, scrolling as AppKit's scroll views do; top is its first box.
func appKitScroller(t *testing.T) (tt *Tester, scroll **node, top **node) {
	t.Helper()
	var s, first *node
	tt = coreNewTester(func(c *context) {
		s = coreScroll(c).Fill().Children(func() {
			first = coreBox(c).Height(20).Shrink(0)
			coreBox(c).Height(980).Shrink(0)
		})
	}, 200, 200)
	tt.rt.appKitScrolling = true
	tt.Frame()
	return tt, &s, &first
}

// letGo ends the fingers' scrolling at (x, y), as a touchpad tells.
func letGo(tt *Tester, x, y float32) {
	tt.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: float64(x), Y: float64(y), Precise: true, ScrollEnded: true})
}

func TestRubberBand(t *testing.T) {
	tt, _, top := appKitScroller(t)
	start := (*top).y
	tt.Move(100, 100)
	// Past the top: the content follows less and less far.
	tt.Scroll(100, 100, 0, -100)
	pulled := (*top).y - start
	if pulled <= 0 || pulled >= 100 {
		t.Fatalf("pulled 100 past the top, the content moved %v", pulled)
	}
	tt.Scroll(100, 100, 0, -100)
	if more := (*top).y - start; more <= pulled || more-pulled >= pulled {
		t.Errorf("pulled 100 more, the content went from %v to %v", pulled, more)
	}
	// Back: the stretch gives way before the content scrolls.
	tt.Scroll(100, 100, 0, 150)
	if d := (*top).y - start; d <= 0 {
		t.Errorf("pushed back 150 of a pull of 200, the content is at %v", d)
	}
	tt.Scroll(100, 100, 0, 100)
	if d := (*top).y - start; d != -50 {
		t.Errorf("pushed back 50 past the pull, the content is at %v, not -50", d)
	}
	// Let go past the end, the content springs back to it, and the
	// momentum after does not move it.
	tt.Scroll(100, 100, 0, 2000)
	end := start - 800 // the content's 1000 DIPs, less the container's 200
	if (*top).y >= end {
		t.Fatalf("pulled past the end, the content stayed at %v", (*top).y)
	}
	letGo(tt, 100, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: 30, Precise: true, Momentum: true})
	tt.Advance(time.Second)
	if (*top).y != end {
		t.Errorf("let go, the content is at %v, not back at the end at %v", (*top).y, end)
	}
}

func TestRubberBandMomentumBounces(t *testing.T) {
	tt, _, top := appKitScroller(t)
	start := (*top).y
	tt.Move(100, 100)
	// Thrown at the top: off it, then back.
	tt.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: -40, Precise: true, Momentum: true})
	tt.Advance(50 * time.Millisecond)
	if (*top).y <= start {
		t.Errorf("thrown at the top, the content did not bounce off it: %v", (*top).y-start)
	}
	tt.Advance(time.Second)
	if (*top).y != start {
		t.Errorf("the bounce came back to %v, not the top", (*top).y-start)
	}
}

func TestOverlayScrollBars(t *testing.T) {
	tt, s, _ := appKitScroller(t)
	tt.Move(100, 100)
	if (*s).st.barsShown {
		t.Error("the bars show with the pointer over the container, which did not scroll")
	}
	tt.Scroll(100, 100, 0, 50)
	if !(*s).st.barsShown {
		t.Error("the bars do not show as the content scrolls")
	}
	tt.Advance(2 * time.Second)
	if (*s).st.barsShown {
		t.Error("the bars still show long after the content stopped")
	}
	// On the track, they show.
	tt.Move(197, 100)
	if !(*s).st.barsShown {
		t.Error("the bars do not show with the pointer on the track")
	}
	tt.Move(100, 100)
	tt.rt.barsAlways = true
	tt.Frame()
	if !(*s).st.barsShown {
		t.Error("the bars do not show where the desktop asks for them always")
	}
}
