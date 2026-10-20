// Package ui draws the app with mygo's native UI toolkit.
package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
)

// Tokens are the colors of one appearance. The light and dark sets share
// the same roles, so a widget asks for a role and both themes answer.
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

// Dark is the app's dark palette: warm greys with a terracotta accent.
func Dark() Tokens {
	return Tokens{
		Dark: true,
		Base: ui.Hex("#141413"), Topbar: ui.Hex("#0e0e0d"),
		Surface: ui.Hex("#30302e"), Surface2: ui.Hex("#3d3d3a"), Surface3: ui.Hex("#262624"),
		FG: ui.Hex("#faf9f5"), FG2: ui.Hex("#b0aea5"), Muted: ui.Hex("#87867f"),
		Border: ui.Hex("#3d3d3a"), Hair: ui.RGBA(250, 249, 245, 0.08),
		Accent: ui.Hex("#c96442"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#5b9a4a"), Warning: ui.Hex("#c9a23a"), Danger: ui.Hex("#b53333"),
		TCP: ui.Hex("#7aa2c4"), UDP: ui.Hex("#9c8ac4"),
		HTTP: ui.Hex("#c4a05a"), HTTPS: ui.Hex("#5b9a6a"),
	}
}

// Light is the same roles for a bright window: paper greys, the same
// terracotta, and darker status colors so that they stay readable on white.
func Light() Tokens {
	return Tokens{
		Base: ui.Hex("#faf9f5"), Topbar: ui.Hex("#f2f0ea"),
		Surface: ui.Hex("#ffffff"), Surface2: ui.Hex("#f0eee7"), Surface3: ui.Hex("#f6f4ee"),
		FG: ui.Hex("#1f1e1b"), FG2: ui.Hex("#55534d"), Muted: ui.Hex("#86847c"),
		Border: ui.Hex("#d3cfc3"), Hair: ui.RGBA(31, 30, 27, 0.08),
		Accent: ui.Hex("#c96442"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#4d8a3c"), Warning: ui.Hex("#a8852b"), Danger: ui.Hex("#a52a2a"),
		TCP: ui.Hex("#4f7fa8"), UDP: ui.Hex("#7a68a8"),
		HTTP: ui.Hex("#a8823c"), HTTPS: ui.Hex("#3f8154"),
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
// widgets: buttons, inputs, tables, dialogs.
func theme(t Tokens) *ui.Theme {
	th := ui.LightTheme()
	if t.Dark {
		th = ui.DarkTheme()
	}
	th.Background, th.Surface = t.Base, t.Surface
	th.SurfaceHover, th.SurfacePressed = t.Surface2, t.Surface2
	th.Border, th.Text, th.TextMuted = t.Border, t.FG, t.Muted
	th.Accent, th.AccentHover, th.AccentPressed, th.AccentText = t.Accent, t.Accent, t.Accent, t.AccentText
	th.Danger, th.Warning, th.Success = t.Danger, t.Warning, t.Success
	th.Selection = ui.RGBA(201, 100, 66, 0.28)
	th.Focus = ui.RGBA(201, 100, 66, 0.55)
	th.Scrollbar = t.Muted
	th.Radius, th.Spacing = 3, 4
	th.FontSize = 13
	if !t.Dark {
		th.Inverse, th.InverseText = ui.Hex("#30302e"), ui.Hex("#faf9f5")
	}
	return th
}
