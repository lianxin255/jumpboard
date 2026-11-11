package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// The arrival halo.
//
// Switching to a page is the one moment the interface has the user's whole
// attention, and a hard cut spends it on nothing. What reads as the page
// arriving — rather than being swapped in — is a single sweep of light across
// it: a soft band that starts at the leading edge and travels out, leaving the
// content slightly lifted as it passes.
//
// The rules that keep it from becoming decoration:
//
//   - It runs once per arrival, not on every frame. A sweep that repeats is a
//     loading indicator, which is a different promise.
//   - It is short. Past ~450ms the user is waiting for it, which is the
//     opposite of polish.
//   - It never covers text. The band is drawn behind the content and only its
//     tint and a hairline of light reach the surface, so nothing is obscured
//     and no layout moves.
//   - It is skipped entirely when the desktop asks for less motion. This is
//     motion, not a fade, so it does not get the NonMotion exemption.

const haloDuration = 420 * time.Millisecond

// haloState remembers which page the sweep belongs to. The sweep itself is
// driven by the toolkit's animation clock (Element.AnimateTo), not by a
// self-scheduled frame: a c.After loop keeps the window awake for the whole
// duration, which stalls anything waiting for a settled frame — including
// the tests, and including the desktop's own idle handling.
type haloState struct {
	page appt.Page
	// seen says a page has been drawn at least once. The first frame of a
	// window is not an arrival — nothing was replaced — so it must not
	// sweep, or the app would animate every time it opens.
	seen bool
	// token changes when the page does, which restarts the animation.
	token int
}

// haloStart notes that page is being shown and reports whether it is a new
// arrival, which is what restarts the sweep.
func (h *haloState) haloStart(p appt.Page) bool {
	if !h.seen {
		h.seen, h.page = true, p
		return false
	}
	if h.page == p {
		return false
	}
	h.page = p
	h.token++
	return true
}

// haloSweep is the arrival sweep's animation state. It uses the toolkit's
// own AnimateTo, so the frame clock is the toolkit's business: the sweep
// advances while it is live and the window goes idle once it is done.
type haloSweep struct {
	// progress is 0 before the first frame of a sweep and 1 when it is
	// over; AnimateTo moves it between them.
	progress float32
	// live is true from the frame a sweep starts until it reaches 1.
	live bool
}

// Live reports whether a sweep is still running.
func (s *haloSweep) Live() bool { return s.live }

// draw advances and paints the sweep. arrived starts a new one.
func (s *haloSweep) draw(c *ui.Context, t Tokens, arrived bool, width, height float32) {
	if arrived {
		s.progress, s.live = 0, true
	}
	if !s.live {
		return
	}
	box := ui.Box(c).Absolute().Left(0).Top(0).Width(width).Height(height)
	// AnimateTo returns the current value and keeps the toolkit redrawing
	// until it reaches the target; the element it is called on anchors the
	// animation's identity.
	s.progress = box.AnimateTo("halo", 1, ui.Animation{
		Duration: haloDuration,
		Ease:     ui.EaseOut,
	})
	if s.progress >= 1 {
		s.live = false
		return
	}
	haloBand(c, t, s.progress, width, height)
}

// haloBand draws the sweep itself.
//
// It is a very low-contrast band: the accent at a few percent, fading to
// nothing at both ends, plus one hairline at its leading edge. Anything
// stronger turns a page transition into a light show, and anything that
// covers the content makes the page unreadable for the duration.
func haloBand(c *ui.Context, t Tokens, progress float32, width, height float32) {
	if progress <= 0 || progress >= 1 {
		return
	}
	// The band travels from just outside the left edge to just outside the
	// right one, so it enters and leaves rather than appearing mid-page.
	bandW := width * 0.34
	x := -bandW + progress*(width+bandW)

	ui.Box(c).Absolute().Left(x).Top(0).Width(bandW).Height(height).
		Background(t.Accent.Alpha(0.045)).
		// The leading edge is one hairline of the accent, which is what the
		// eye actually catches; the fill alone is too faint to notice.
		BorderWidth(0, 1, 0, 0).
		BorderColor(t.Accent.Alpha(0.16)).
		// It lies over the page, so it must not take the pointer: a sweep
		// that swallowed clicks for 400ms would make the page feel broken
		// exactly when the user is trying to use it.
		PassThrough()
}
