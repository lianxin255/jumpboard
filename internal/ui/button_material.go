package ui

import (
	"github.com/egoist/mygo/ui"
)

// The button materials.
//
// A flat fill reads as a rectangle that happens to have a word on it. What
// makes a control feel pressable is a stack of very small cues that agree
// with each other: the surface is lit from above, so it is lighter at the top
// than the bottom; it has an edge; the top edge catches light and the bottom
// edge sits in shadow; and it casts a shadow onto the surface below it. When
// the button is pressed, every one of those cues flips or shrinks together,
// which is what the eye reads as "it went down".
//
// The layers, in the order they are applied:
//
//  1. a 180° linear gradient, top lighter than bottom by a few percent
//  2. a 1px border, a step darker than the face
//  3. an inset highlight along the top edge
//  4. an inset shadow along the bottom edge
//  5. two drop shadows — a 1px contact shadow and a wider diffuse one
//
// The pressed face keeps the same number of inset layers as the resting one,
// with the highlight swapped for a shadow rather than removed. Shadows here
// are painted, not laid out — they add nothing to an element's measured size
// — so this is not about avoiding a layout twitch. It is about the face
// reading as the same object: a layer that vanishes on press makes the
// surface look thinner than a moment ago, and the eye catches that as a
// flicker even when nothing moved.

// buttonMaterial is the resolved set of colors one button face needs. It is
// computed from the tokens rather than stored, so both appearances and both
// emphasis levels get their own values without a second palette.
type buttonMaterial struct {
	top     ui.Color // gradient start, the lit top
	bottom  ui.Color // gradient end, the shaded bottom
	edge    ui.Color // 1px border
	hi      ui.Color // inset highlight along the top edge
	lo      ui.Color // inset shadow along the bottom edge
	contact ui.Color // 1px contact shadow under the face
	diffuse ui.Color // the wider shadow that lifts it off the surface
}

// primaryMaterial is the solid accent face: the one main action of a page.
func primaryMaterial(t Tokens) buttonMaterial {
	if t.Dark {
		// On the night desktop the accent is a light periwinkle, so the lit
		// top is the accent lifted and the bottom is the accent itself —
		// the gradient runs the other way from a dark face's.
		return buttonMaterial{
			top:     t.Accent.Mix(ui.Hex("#FFFFFF"), 0.14),
			bottom:  t.Accent,
			edge:    t.Accent.Mix(ui.Hex("#000000"), 0.22),
			hi:      ui.RGBA(255, 255, 255, 0.28),
			lo:      ui.RGBA(0, 0, 0, 0.16),
			contact: ui.RGBA(0, 0, 0, 0.30),
			diffuse: ui.RGBA(0, 0, 0, 0.22),
		}
	}
	return buttonMaterial{
		top:     t.Accent.Mix(ui.Hex("#FFFFFF"), 0.10),
		bottom:  t.Accent.Mix(ui.Hex("#000000"), 0.06),
		edge:    t.Accent.Mix(ui.Hex("#000000"), 0.24),
		hi:      ui.RGBA(255, 255, 255, 0.22),
		lo:      ui.RGBA(0, 0, 0, 0.14),
		contact: ui.RGBA(0, 0, 0, 0.18),
		diffuse: ui.RGBA(0, 0, 0, 0.12),
	}
}

// secondaryMaterial is the raised white key: a plain button that still reads
// as a physical thing, one step below the accent.
func secondaryMaterial(t Tokens) buttonMaterial {
	if t.Dark {
		return buttonMaterial{
			top:     t.Surface.Mix(ui.Hex("#FFFFFF"), 0.07),
			bottom:  t.Surface,
			edge:    t.Border,
			hi:      ui.RGBA(255, 255, 255, 0.10),
			lo:      ui.RGBA(0, 0, 0, 0.22),
			contact: ui.RGBA(0, 0, 0, 0.34),
			diffuse: ui.RGBA(0, 0, 0, 0.24),
		}
	}
	return buttonMaterial{
		top:     ui.Hex("#FFFFFF"),
		bottom:  t.Surface.Mix(ui.Hex("#000000"), 0.035),
		edge:    t.Border,
		hi:      ui.RGBA(255, 255, 255, 0.90),
		lo:      ui.RGBA(0, 0, 0, 0.05),
		contact: ui.RGBA(0, 0, 0, 0.10),
		diffuse: ui.RGBA(0, 0, 0, 0.07),
	}
}

// applyButtonMaterial paints one face. pressed flips the light: the gradient
// reverses, the top highlight becomes a shadow, and the drop shadows pull in
// — which, together with the 1px translate the caller applies, is the whole
// physical story of a press.
//
// The diffuse shadow is deliberately tight. A wide one looks right on a
// single hero button and wrong everywhere else: these controls sit in dense
// toolbars on a flat wash, and a 10px blur at low alpha tints the whole
// surrounding bar by a shade, which reads as a smudge rather than as depth.
// Keeping it to a few pixels means the lift is felt at the control's own edge
// and nowhere else.
func applyButtonMaterial(e ui.Element, m buttonMaterial, pressed bool) ui.Element {
	if pressed {
		// The face has moved down into the surface: its own light now comes
		// from the shadow above it, and it no longer casts much.
		return e.
			Gradient(m.bottom, m.top, 180).
			Border(1, m.edge).
			InsetShadow(0, 1, 2, 0, m.lo).
			InsetShadow(0, -1, 0, 0, m.hi.Alpha(0.35)).
			Shadow(0, 1, 1, 0, m.contact.Alpha(0.55)).
			Shadow(0, 1, 2, 0, m.diffuse.Alpha(0.35))
	}
	return e.
		Gradient(m.top, m.bottom, 180).
		Border(1, m.edge).
		InsetShadow(0, 1, 0, 0, m.hi).
		InsetShadow(0, -1, 0, 0, m.lo).
		Shadow(0, 1, 1, 0, m.contact).
		Shadow(0, 2, 3, 0, m.diffuse)
}

// PressOffset is how far a pressed control moves down, in points, for a
// control of the given font size.
//
// It is always 1. That is the point: the travel of a press is a physical
// constant, not something that scales with type. A 22px button that moves 2px
// reads as sinking into the page, and an 11px chip that moves 2px reads as
// falling over — both worse than the same 1px everywhere. The parameter stays
// so a caller that ever needs a different figure has somewhere to say so,
// and so the reason is written down rather than guessed at.
func PressOffset(fontSize float32) float32 {
	_ = fontSize
	return 1
}
