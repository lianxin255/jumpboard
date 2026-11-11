package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion tokens: the animation curves the app moves by, one per role.
//
// A single default ease everywhere is what makes an interface feel generic —
// a menu that fades in with the same curve as a button that settles reads as
// neither. Each role below answers a different question about how something
// arrived:
//
//   - MotionEnter — it appeared where it was not. Ease-out decelerates into
//     place, so the eye catches the arrival and not the travel.
//   - MotionPress — it answered a click and must feel instantaneous. A very
//     short, strongly decelerating curve is what makes a press read as
//     "landed" rather than "animated".
//   - MotionExit — it left. Ease-in accelerates away, so attention is already
//     moving on before it finishes.
//   - MotionShift — it moved within the page, an indicator sliding between
//     tabs. Symmetric ease-in-out, because both ends are equally interesting.
//   - MotionSettle — it was released and returns. A spring, so it carries the
//     velocity it had rather than restarting from a dead stop.
//   - MotionSpin — it is still going. Linear, because a spinner that eases
//     looks like it is stuttering.
//
// Reduced motion is the toolkit's job: it reads the desktop's preference and
// snaps positions and sizes to their targets while still crossfading colors,
// which is why these are plain Animation values and not a check at each call
// site.
var (
	MotionEnter  = ui.Animation{Duration: 180 * time.Millisecond, Ease: ui.EaseOut}
	MotionPress  = ui.Animation{Duration: 90 * time.Millisecond, Ease: ui.CubicBezier(0.22, 1, 0.36, 1)}
	MotionExit   = ui.Animation{Duration: 140 * time.Millisecond, Ease: ui.EaseIn}
	MotionShift  = ui.Animation{Duration: 220 * time.Millisecond, Ease: ui.EaseInOut}
	MotionSettle = ui.Snappy
	MotionSpin   = ui.Animation{Duration: 900 * time.Millisecond, Ease: ui.Linear}

	// MotionFade is the same curve as MotionEnter but marked as moving
	// nothing on screen, so a crossfade still happens when the desktop asks
	// for less motion — a fade is not motion, and losing it just makes the
	// interface feel broken rather than calm.
	MotionFade = ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut, NonMotion: true}
)

// StaggerDelay is the delay the nth element of a group takes before it starts
// moving, so a list arrives near-to-far instead of all at once. n is the
// element's distance from the one the eye is already on — 0 for the first.
//
// The step is small on purpose: at 24ms the tenth row still starts within a
// quarter second, which reads as a wave. Larger steps read as lag.
func StaggerDelay(n int) time.Duration {
	if n < 0 {
		n = 0
	}
	if n > 8 {
		n = 8 // past the eighth the wave is over; more delay is just waiting
	}
	return time.Duration(n) * 24 * time.Millisecond
}

// Staggered returns a that starts after the nth step of a group.
func Staggered(a ui.Animation, n int) ui.Animation {
	a.Delay = StaggerDelay(n)
	return a
}
