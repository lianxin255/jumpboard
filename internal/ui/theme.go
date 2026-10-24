// Package ui draws the app with mygo's native UI toolkit.
package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
)

// Shape tokens keep panels, modals and chips consistent across pages.
// The toolkit theme uses Radius/Spacing 8/6; these are for hand-drawn boxes.
const (
	RadiusModal = float32(12)
	RadiusPanel = float32(10)
	RadiusChip  = float32(6)
	RadiusBadge = float32(4)
	RadiusTiny  = float32(2)

	PagePadY   = float32(20)
	PagePadX   = float32(28)
	PageGap    = float32(14)
	TitleSize  = float32(22)
	SubtitleSz = float32(12)
)

// Tokens are the colors of one appearance. The light and dark sets share
// the same roles, so a widget asks for a role and both themes answer.
//
// The palette is mygo's own: the zinc neutrals and the blue accent of
// ui.LightTheme and ui.DarkTheme, so the app looks like the toolkit's
// examples rather than a theme of its own.
type Tokens struct {
	Dark bool

	// Backgrounds, from the window's own to the sunken ones.
	Base     ui.Color
	Topbar   ui.Color
	Surface  ui.Color
	Surface2 ui.Color
	Surface3 ui.Color

	// Text, from primary to a hint.
	FG    ui.Color
	FG2   ui.Color
	Muted ui.Color

	Border ui.Color
	Hair   ui.Color

	Accent     ui.Color
	AccentText ui.Color
	Success    ui.Color
	Warning    ui.Color
	Danger     ui.Color

	// Protocol badges.
	TCP   ui.Color
	UDP   ui.Color
	HTTP  ui.Color
	HTTPS ui.Color
}

// Dark is the toolkit's dark theme: zinc greys with the blue accent.
func Dark() Tokens {
	return Tokens{
		Dark: true,
		Base: ui.Hex("#18181b"), Topbar: ui.Hex("#131316"),
		Surface: ui.Hex("#27272a"), Surface2: ui.Hex("#323236"), Surface3: ui.Hex("#212124"),
		FG: ui.Hex("#f4f4f5"), FG2: ui.Hex("#d4d4d8"), Muted: ui.Hex("#a1a1aa"),
		Border: ui.Hex("#3f3f46"), Hair: ui.RGBA(244, 244, 245, 0.08),
		Accent: ui.Hex("#3b82f6"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#22c55e"), Warning: ui.Hex("#f59e0b"), Danger: ui.Hex("#ef4444"),
		TCP: ui.Hex("#60a5fa"), UDP: ui.Hex("#a78bfa"),
		HTTP: ui.Hex("#38bdf8"), HTTPS: ui.Hex("#34d399"),
	}
}

// Light is the toolkit's light theme for a bright window: the same zinc
// ramp and blue, with the darker status colors it uses.
func Light() Tokens {
	return Tokens{
		Base: ui.Hex("#ffffff"), Topbar: ui.Hex("#fafafa"),
		Surface: ui.Hex("#f4f4f5"), Surface2: ui.Hex("#e9e9ec"), Surface3: ui.Hex("#fafafa"),
		FG: ui.Hex("#18181b"), FG2: ui.Hex("#3f3f46"), Muted: ui.Hex("#71717a"),
		Border: ui.Hex("#d9d9de"), Hair: ui.RGBA(24, 24, 27, 0.08),
		Accent: ui.Hex("#2563eb"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#16a34a"), Warning: ui.Hex("#d97706"), Danger: ui.Hex("#dc2626"),
		TCP: ui.Hex("#2563eb"), UDP: ui.Hex("#7c3aed"),
		HTTP: ui.Hex("#0284c7"), HTTPS: ui.Hex("#059669"),
	}
}

// TokensFor picks the palette for the user's choice and the desktop's
// appearance.
func TokensFor(choice appt.ThemeChoice, systemDark bool) Tokens {
	switch choice {
	case appt.ThemeLight:
		return Light()
	case appt.ThemeDark:
		return Dark()
	}
	if systemDark {
		return Dark()
	}
	return Light()
}

// theme maps the tokens onto the toolkit's theme, which colors its own
// widgets: buttons, inputs, tables, dialogs. The starting point is the
// toolkit's own theme, so anything the app does not override keeps the
// native look.
func theme(t Tokens) *ui.Theme {
	th := ui.LightTheme()
	if t.Dark {
		th = ui.DarkTheme()
	}
	th.Background, th.Surface = t.Base, t.Surface
	th.SurfaceHover, th.SurfacePressed = t.Surface2, t.Surface2
	th.Border, th.Text, th.TextMuted = t.Border, t.FG, t.Muted
	th.Accent, th.AccentText = t.Accent, t.AccentText
	th.Danger, th.Warning, th.Success = t.Danger, t.Warning, t.Success
	th.Scrollbar = t.Muted
	// The toolkit's own shape language: the native radius, not a square one.
	th.Radius, th.Spacing = 8, 6
	th.FontSize = 13
	if !t.Dark {
		th.Inverse, th.InverseText = ui.Hex("#27272a"), ui.Hex("#f4f4f5")
	}
	return th
}
