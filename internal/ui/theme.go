// Package ui draws the app with mygo's native UI toolkit.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// Shape tokens keep panels, modals and chips consistent across pages.
// The toolkit theme uses Radius/Spacing 10/6; these are for hand-drawn boxes.
// The vocabulary is deliberately small: 14 for the window, floating bars,
// panels, modals and the big empty-state cards; 10 for controls; 6 for
// chips and badges. Pill (a capsule taller than its radius) and tiny
// functional marks (the nav rail, status dots) keep their own values.
// SidebarW is the frosted rail's width. Page pads and gaps sit on the
// 4/8 grid: 20 and 32 vertically and horizontally, 14 between blocks.
const (
	RadiusWindow = float32(14)
	RadiusModal  = float32(14)
	RadiusFloat  = float32(14)
	RadiusPanel  = float32(14)
	RadiusButton = float32(10)
	RadiusChip   = float32(6)
	RadiusBadge  = float32(6)
	RadiusTiny   = float32(2)
	RadiusPill   = float32(12)

	SidebarW = float32(216)
	PagePadY = float32(20)
	PagePadX = float32(32)
	PageGap  = float32(14)
)

// Type tokens: the sizes text falls into, so a page cannot grow a ninth
// orphan size. Title heads the window bar — 20, the plan's page title,
// not the 24 a dialog never had room for — Head a dialog or a section,
// RowTitle is the row's own title size, Body is the toolkit's own default
// for controls and prose, Note is the secondary line beside Body, and
// Caption is metadata and the small controls of a row — timestamps,
// chips, and the small buttons lists carry.
const (
	TitleSize    = float32(20)
	HeadSize     = float32(16)
	RowTitleSize = float32(14)
	BodySize     = float32(13)
	NoteSize     = float32(12)
	CaptionSize  = float32(12)
)

// FontMono is the family addresses, commands, logs and keys draw in: a
// name rather than a string at every call site, so the family can change
// in one place.
const FontMono = "monospace"

// Tokens are the colors of one appearance. The light and dark sets share
// the same roles, so a widget asks for a role and both themes answer.
//
// The palette is JumpBoard's own: a slate-blue neutral ramp with one blue
// accent — strong on the light desktop, lifted to a periwinkle on the dark
// one so it holds its contrast against the night surfaces — and the
// protocol and status hues the app has always carried.
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
	// placeholders; it stays at small-text contrast in both appearances.
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

	// Shadow is the drop under a modal or a floating bar. Step is the
	// neutral a face's hover and pressed neighbors mix toward — black on a
	// light desktop, white on a dark one.
	Shadow ui.Color
	Step   ui.Color

	// Protocol badges.
	TCP   ui.Color
	UDP   ui.Color
	HTTP  ui.Color
	HTTPS ui.Color
}

// Dark keeps the same roles as the light desktop, on a slate night
// surface, so a widget that asks for a role still gets an answer. The
// accent is lifted light so its ink can be the night surface itself.
func Dark() Tokens {
	return Tokens{
		Dark: true,
		Base: ui.Hex("#10141B"), Topbar: ui.Hex("#151B24"),
		Surface: ui.Hex("#171D26"), Surface2: ui.Hex("#222B38"), Surface3: ui.Hex("#131923"),
		Sidebar: ui.Hex("#151B24"), Canvas: ui.Hex("#10141B"), Chip: ui.Hex("#1F2632"),
		FG: ui.Hex("#EDF2F7"), FG2: ui.Hex("#C2CCDB"), Muted: ui.Hex("#9AA7B8"), Meta: ui.Hex("#8B98AC"),
		Border: ui.Hex("#2A3442"), Hair: ui.RGBA(237, 242, 247, 0.08),
		Accent: ui.Hex("#8FB0FF"), AccentText: ui.Hex("#10141B"),
		Success: ui.Hex("#22C55E"), SuccessSoft: ui.RGBA(34, 197, 94, 0.16),
		Warning: ui.Hex("#F59E0B"), WarningSoft: ui.RGBA(245, 158, 11, 0.16),
		Danger: ui.Hex("#F77C79"), DangerSoft: ui.RGBA(247, 124, 121, 0.16),
		AmberText: ui.Hex("#F3D7A6"), AmberSoft: ui.RGBA(243, 215, 166, 0.12), AmberLine: ui.RGBA(243, 215, 166, 0.35),
		Shadow: ui.RGBA(0, 0, 0, 0.45), Step: ui.Hex("#FFFFFF"),
		TCP: ui.Hex("#60A5FA"), UDP: ui.Hex("#A78BFA"),
		HTTP: ui.Hex("#2DD4BF"), HTTPS: ui.Hex("#FBBF24"),
	}
}

// Light is the day desktop: the plan's cool near-white wash (#F5F6F8)
// behind pure white surfaces, one strong blue for the primary action, the
// selected nav, and an open switch — dark ink on the accent where the
// dark theme inverts it.
func Light() Tokens {
	return Tokens{
		Base: ui.Hex("#F5F6F8"), Topbar: ui.Hex("#EFF1F4"),
		Surface: ui.Hex("#FFFFFF"), Surface2: ui.Hex("#F1F4F9"), Surface3: ui.Hex("#F1F4F9"),
		Sidebar: ui.Hex("#EFF1F4"), Canvas: ui.Hex("#F5F6F8"), Chip: ui.Hex("#F1F4F9"),
		FG: ui.Hex("#172033"), FG2: ui.Hex("#3E4B61"), Muted: ui.Hex("#667085"), Meta: ui.Hex("#667085"),
		Border: ui.Hex("#DDE4EC"), Hair: ui.Hex("#E9EEF4"),
		Accent: ui.Hex("#315FD5"), AccentText: ui.Hex("#FFFFFF"),
		Success: ui.Hex("#147D3F"), SuccessSoft: ui.Hex("#E9F8EE"),
		Warning: ui.Hex("#925505"), WarningSoft: ui.Hex("#FDF3E6"),
		Danger: ui.Hex("#B23633"), DangerSoft: ui.Hex("#FDECEC"),
		AmberText: ui.Hex("#8A5416"), AmberSoft: ui.Hex("#FFF8EE"), AmberLine: ui.Hex("#F3D7A6"),
		Shadow: ui.RGBA(0, 0, 0, 0.45), Step: ui.Hex("#000000"),
		TCP: ui.Hex("#315FD5"), UDP: ui.Hex("#6A4FD8"),
		HTTP: ui.Hex("#08786D"), HTTPS: ui.Hex("#9A5B06"),
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
	// Buttons, inputs and chips share the control radius.
	th.Radius, th.Spacing = 10, 6
	th.FontSize = BodySize
	if !t.Dark {
		// Tooltips and toasts are the theme turned over: the day ink on the
		// day wash.
		th.Inverse, th.InverseText = t.FG, t.Canvas
	}
	return th
}
