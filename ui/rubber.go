package ui

import (
	"math"
	"slices"
	"time"
)

// springBack is how a stretch goes back to the end of the content.
var springBack = Animation{Spring: SpringOptions{Duration: 400 * time.Millisecond}}

// stretch is how far a touchpad pulled a scroll container's content past
// its ends: pullX and pullY are how far the fingers went past them, which
// x and y show less and less of, as a rubber band gives (band), along
// the content's view: positive past the right and the bottom. Released,
// they spring back.
type stretch struct {
	pullX, pullY float64
	x, y         channel
	released     bool
}

// band returns how far content pulled past an end by pull shows beyond it,
// in a container size long along the pull, as AppKit's scroll views give.
func band(pull, size float64) float64 {
	if size <= 0 {
		return 0
	}
	const c = 0.55
	return math.Copysign((1-1/(math.Abs(pull)*c/size+1))*size, pull)
}

// unband returns the pull that shows a stretch of shown: band's inverse.
func unband(shown, size float64) float64 {
	if size <= 0 || math.Abs(shown) >= size {
		return 0
	}
	const c = 0.55
	return math.Copysign((1/(1-math.Abs(shown)/size)-1)*size/c, shown)
}

// stretched returns how far s shows its content past its ends.
func (s *state) stretched() (x, y float64) {
	if st := s.stretch; st != nil {
		return st.x.value, st.y.value
	}
	return 0, 0
}

// elasticAlong reports whether a touchpad scrolling s by dx, dy pulls on
// it: it scrolls that way, its content overflowing it.
func (s *state) elasticAlong(dx, dy float32) bool {
	if math.Abs(float64(dy)) >= math.Abs(float64(dx)) {
		return s.flags&flagScrollY != 0 && s.contentH > float64(s.h)
	}
	return s.flags&flagScrollX != 0 && s.contentW > float64(s.w)
}

// pullAxis moves pos, between 0 and end, by d, the content pulled past
// them by pull: it returns how far the offset moves and the pull after.
// A pull goes back before the offset moves.
func pullAxis(pos, end, pull, d float64) (move, after float64) {
	if pull != 0 {
		p := pull + d
		if p != 0 && (p < 0) == (pull < 0) {
			return 0, p
		}
		d = p // back within the content: the rest scrolls it
	}
	switch to := pos + d; {
	case to < 0:
		return -pos, to
	case to > end:
		return end - pos, to - end
	}
	return d, 0
}

// pull scrolls container s by a touchpad's dx, dy along its content's
// view, and past its ends, which stretch. A momentum reaching an end
// bounces off it; one going on after the fingers let go of a stretch
// goes nowhere, as in AppKit.
func (rt *engine) pull(s *state, dx, dy float32, momentum bool) {
	now := rt.now()
	st := s.stretch
	if st != nil && st.released {
		if momentum {
			return
		}
		// The fingers take hold of it again, where it shows.
		st.x.advance(now)
		st.y.advance(now)
		st.pullX, st.pullY = unband(st.x.value, float64(s.w)), unband(st.y.value, float64(s.h))
		st.released = false
	}
	if st == nil {
		st = &stretch{x: channel{at: now}, y: channel{at: now}}
	}
	if s.track != nil {
		s.track.glide = nil // the user takes over from ScrollTo
	}
	var mx, my float64
	if s.flags&flagScrollX != 0 && s.contentW > float64(s.w) {
		left := s.scrollX
		if s.rtl {
			left = s.contentW - float64(s.w) - s.scrollX
		}
		mx, st.pullX = pullAxis(left, s.contentW-float64(s.w), st.pullX, float64(dx))
	}
	if s.flags&flagScrollY != 0 && s.contentH > float64(s.h) {
		my, st.pullY = pullAxis(s.scrollY, s.contentH-float64(s.h), st.pullY, float64(dy))
	}
	if mx != 0 || my != 0 {
		scrollBy(s, float32(mx), float32(my))
	}
	if momentum && (st.pullX != 0 || st.pullY != 0) {
		// Thrown at an end: off it, as fast as it came, and back.
		const frame = 1.0 / 60
		st.x.value, st.x.velocity = 0, st.pullX/frame*0.5
		st.y.value, st.y.velocity = 0, st.pullY/frame*0.5
		st.pullX, st.pullY = 0, 0
		st.released = true
		st.x.retarget(0, springBack)
		st.y.retarget(0, springBack)
	} else {
		st.x.value, st.x.target, st.x.velocity, st.x.moving = band(st.pullX, float64(s.w)), 0, 0, false
		st.y.value, st.y.target, st.y.velocity, st.y.moving = band(st.pullY, float64(s.h)), 0, 0, false
	}
	switch {
	case st.released || st.x.value != 0 || st.y.value != 0:
		s.stretch = st
		if !slices.Contains(rt.stretching, s.id) {
			rt.stretching = append(rt.stretching, s.id)
		}
	default:
		s.stretch = nil
	}
	rt.requestFrame()
}

// releaseStretches springs back what the fingers stretched, as they let
// go of the touchpad.
func (rt *engine) releaseStretches() {
	now := rt.now()
	for _, id := range rt.stretching {
		s := rt.states[id]
		if s == nil || s.stretch == nil || s.stretch.released {
			continue
		}
		st := s.stretch
		st.pullX, st.pullY, st.released = 0, 0, true
		st.x.at, st.y.at = now, now
		st.x.retarget(0, springBack)
		st.y.retarget(0, springBack)
	}
	rt.requestFrame()
}

// springStretches moves the stretches springing back to the frame's time.
func (rt *engine) springStretches(now time.Time) {
	kept := rt.stretching[:0]
	for _, id := range rt.stretching {
		s := rt.states[id]
		if s == nil || s.stretch == nil {
			continue
		}
		st := s.stretch
		if st.released {
			st.x.advance(now)
			st.y.advance(now)
			if !st.x.moving && !st.y.moving {
				s.stretch = nil
				continue
			}
			rt.animating = true
		}
		kept = append(kept, id)
	}
	rt.stretching = kept
}
