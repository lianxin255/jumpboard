// Package ui draws the app with mygo's native UI toolkit.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// Shape tokens keep panels, modals and chips consistent across pages.
// The toolkit theme uses Radius/Spacing 10/6; these are for hand-drawn boxes.
// Sizes follow the glass desktop: window 16, floating bars 14, cards 12,
// buttons and inputs 9–10, chips 8, pills 12.
const (
	RadiusWindow = float32(16)
	RadiusModal  = float32(16)
	RadiusFloat  = float32(14)
	RadiusPanel  = float32(12)
	RadiusButton = float32(10)
	RadiusChip   = float32(8)
	RadiusBadge  = float32(6)
	RadiusTiny   = float32(2)
	RadiusPill   = float32(12)

	SidebarW   = float32(232)
	PagePadY   = float32(22)
	PagePadX   = float32(32)
	PageGap    = float32(14)
	TitleSize  = float32(24)
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

	// Sidebar is the frosted rail. Canvas is the content wash behind the
	// page. Chip is the glass chip and search field. Meta is timestamps and
	// placeholders, quieter than Muted.
	Sidebar ui.Color
	Canvas  ui.Color
	Chip    ui.Color
	Meta    ui.Color
	// Soft fills behind status words (healthy / warn / danger).
	SuccessSoft ui.Color
	WarningSoft ui.Color
	DangerSoft  ui.Color
	AmberText   ui.Color
	AmberSoft   ui.Color
	AmberLine   ui.Color

	// Protocol badges.
	TCP   ui.Color
	UDP   ui.Color
	HTTP  ui.Color
	HTTPS ui.Color
}

// Dark keeps the same roles as the light glass desktop, on a zinc night
// surface, so a widget that asks for a role still gets an answer.
func Dark() Tokens {
	return Tokens{
		Dark: true,
		Base: ui.Hex("#14161c"), Topbar: ui.Hex("#1a1e27"),
		Surface: ui.Hex("#1e232d"), Surface2: ui.Hex("#282e3a"), Surface3: ui.Hex("#181c24"),
		Sidebar: ui.Hex("#1a1e27"), Canvas: ui.Hex("#14161c"), Chip: ui.Hex("#242a36"),
		FG: ui.Hex("#f4f6fa"), FG2: ui.Hex("#c5ccd8"), Muted: ui.Hex("#9aa3b5"), Meta: ui.Hex("#7d8799"),
		Border: ui.Hex("#343b4a"), Hair: ui.RGBA(244, 246, 250, 0.08),
		Accent: ui.Hex("#3b82f6"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#22c55e"), SuccessSoft: ui.RGBA(34, 197, 94, 0.16),
		Warning: ui.Hex("#f59e0b"), WarningSoft: ui.RGBA(245, 158, 11, 0.16),
		Danger: ui.Hex("#ef6460"), DangerSoft: ui.RGBA(239, 100, 96, 0.16),
		AmberText: ui.Hex("#f3d7a6"), AmberSoft: ui.RGBA(243, 215, 166, 0.12), AmberLine: ui.RGBA(243, 215, 166, 0.35),
		TCP: ui.Hex("#60a5fa"), UDP: ui.Hex("#a78bfa"),
		HTTP: ui.Hex("#2dd4bf"), HTTPS: ui.Hex("#fbbf24"),
	}
}

// Light is the glass desktop: a frosted blue-grey rail, a near-white
// content wash, and one blue used for the primary action, the selected
// nav, and an open switch.
func Light() Tokens {
	return Tokens{
		Base: ui.Hex("#FBFCFE"), Topbar: ui.Hex("#E7EDF6"),
		Surface: ui.Hex("#FFFFFF"), Surface2: ui.Hex("#F4F7FB"), Surface3: ui.Hex("#F4F7FB"),
		Sidebar: ui.Hex("#E7EDF6"), Canvas: ui.Hex("#FBFCFE"), Chip: ui.Hex("#F4F7FB"),
		FG: ui.Hex("#1C2434"), FG2: ui.Hex("#3A4458"), Muted: ui.Hex("#5C677C"), Meta: ui.Hex("#8A93A6"),
		Border: ui.Hex("#E1E7F0"), Hair: ui.Hex("#EEF1F6"),
		Accent: ui.Hex("#3B82F6"), AccentText: ui.Hex("#ffffff"),
		Success: ui.Hex("#16A34A"), SuccessSoft: ui.Hex("#E9F8EE"),
		Warning: ui.Hex("#D97706"), WarningSoft: ui.Hex("#FDF3E6"),
		Danger: ui.Hex("#DC5B54"), DangerSoft: ui.Hex("#FDECEC"),
		AmberText: ui.Hex("#8A5416"), AmberSoft: ui.Hex("#FFF8EE"), AmberLine: ui.Hex("#F3D7A6"),
		TCP: ui.Hex("#3B82F6"), UDP: ui.Hex("#7C6BF2"),
		HTTP: ui.Hex("#0E9F8E"), HTTPS: ui.Hex("#D97706"),
	}
}

// ApplyAppearance tells the toolkit which appearance the window's material
// should follow. The sidebar draws nothing of its own where the window has
// a material (macOS vibrancy, Windows 11 Mica), so that material is the
// sidebar's color, and it only changes with the toolkit's theme source.
// The widgets follow the tokens either way; without this, a light choice
// on a dark desktop leaves the rail in the desktop's dark material.
func ApplyAppearance(choice appt.ThemeChoice) {
	switch choice {
	case appt.ThemeLight:
		mygo.Theme.SetSource(mygo.ThemeLight)
	case appt.ThemeDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
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
	// Buttons, inputs and chips share the glass radius.
	th.Radius, th.Spacing = 10, 6
	th.FontSize = 13
	if !t.Dark {
		th.Inverse, th.InverseText = ui.Hex("#1C2434"), ui.Hex("#FBFCFE")
	}
	return th
}
