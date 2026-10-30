// Package ui draws the app with mygo's native UI toolkit.
package ui

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/jumpboard/internal/appt"
)

// Shape tokens keep panels, modals and chips consistent across pages.
// The toolkit theme uses Radius/Spacing 10/6; these are for hand-drawn boxes.
// The plan's vocabulary: 12 for the window and modals, 10 for panels and
// floating bars, 7 for controls, 5 for chips and badges. A tighter set of
// corners keeps a dense workbench from reading as a stack of cards; the
// window keeps the largest radius because it is chrome, not content.
// SidebarW is the frosted rail's width. Page pads and gaps sit on the
// 4/8 grid: 20 and 28 horizontally, 20 vertically, 14 between blocks.
const (
	RadiusWindow = float32(12)
	RadiusModal  = float32(12)
	RadiusFloat  = float32(10)
	RadiusPanel  = float32(10)
	RadiusButton = float32(7)
	RadiusChip   = float32(5)
	RadiusBadge  = float32(5)
	RadiusTiny   = float32(2)
	RadiusPill   = float32(11)

	SidebarW = float32(208)
	PagePadY = float32(20)
	PagePadX = float32(28)
	PageGap  = float32(14)

	// Control geometry is one contract across pages: text buttons and
	// compact search fields 32 high, icon buttons 32 square, switches
	// 36×22. Inputs that stand as full form fields use 36. Chips are a
	// different control, not a smaller button.
	ControlHeight     = float32(32)
	ControlHeightForm = float32(36)
	IconButtonSize    = float32(32)
	SwitchWidth       = float32(36)
	SwitchHeight      = float32(22)
)

// Type tokens: the sizes text falls into, so a page cannot grow a ninth
// orphan size. Title heads the window bar — 22, the plan's page title —
// Head a dialog or a section, RowTitle is the row's own title size, Body
// is the toolkit's own default for controls and prose, Note is the
// secondary line beside Body, and Caption is metadata and the small
// controls of a row — timestamps, chips, and the small buttons lists
// carry.
const (
	TitleSize    = float32(22)
	HeadSize     = float32(15)
	RowTitleSize = float32(14)
	BodySize     = float32(13)
	NoteSize     = float32(12)
	CaptionSize  = float32(11)
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
	// AccentSoft is the accent's quiet wash — the selected nav row's and
	// the active chip's fill — with AccentLine its one step stronger
	// border. The plan's selection says itself in a tint, not in the
	// solid: the solid stays the primary action's alone.
	AccentSoft ui.Color
	AccentLine ui.Color
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

// Dark keeps the same roles as the light desktop, on a warm graphite
// night surface, so a widget that asks for a role still gets an answer.
// The accent is lifted light so its ink can be the night surface itself;
// the solid danger face keeps its own deep red so a white label reads.
func Dark() Tokens {
	return Tokens{
		Dark: true,
		Base: ui.Hex("#151918"), Topbar: ui.Hex("#1B201E"),
		Surface: ui.Hex("#222825"), Surface2: ui.Hex("#343E38"), Surface3: ui.Hex("#29312D"),
		Sidebar: ui.Hex("#1B201E"), Canvas: ui.Hex("#151918"), Chip: ui.Hex("#29312D"),
		FG: ui.Hex("#F2F4EF"), FG2: ui.Hex("#D0D7D0"), Muted: ui.Hex("#AAB6AD"), Meta: ui.Hex("#AAB6AD"),
		Border: ui.Hex("#4A574E"), Hair: ui.Hex("#37433B"),
		Accent: ui.Hex("#A7C2F7"), AccentText: ui.Hex("#151918"),
		AccentSoft: ui.Hex("#2D3E57"), AccentLine: ui.Hex("#59769E"),
		Success: ui.Hex("#8BD5A9"), SuccessSoft: ui.RGBA(139, 213, 169, 0.16),
		Warning: ui.Hex("#E6C388"), WarningSoft: ui.RGBA(230, 195, 136, 0.16),
		Danger: ui.Hex("#F5B1AA"), DangerSoft: ui.RGBA(245, 177, 170, 0.14),
		AmberText: ui.Hex("#E6C388"), AmberSoft: ui.RGBA(230, 195, 136, 0.12), AmberLine: ui.RGBA(230, 195, 136, 0.36),
		Shadow: ui.RGBA(0, 0, 0, 0.55), Step: ui.Hex("#FFFFFF"),
		TCP: ui.Hex("#A7C2F7"), UDP: ui.Hex("#CEBCED"),
		HTTP: ui.Hex("#92D5C6"), HTTPS: ui.Hex("#E6C388"),
	}
}

// Light is the day desktop: the plan's warm near-white wash (#F5F5F3)
// behind pure white surfaces, one restrained blue for the primary action
// and the open switch — dark ink on the accent where the dark theme
// inverts it. The selected nav carries the accent's tint, not the solid.
func Light() Tokens {
	return Tokens{
		Base: ui.Hex("#F5F5F3"), Topbar: ui.Hex("#EEEEE8"),
		Surface: ui.Hex("#FFFFFF"), Surface2: ui.Hex("#ECEEEB"), Surface3: ui.Hex("#F7F7F5"),
		Sidebar: ui.Hex("#EEEEE8"), Canvas: ui.Hex("#F5F5F3"), Chip: ui.Hex("#F7F7F5"),
		FG: ui.Hex("#22272B"), FG2: ui.Hex("#525C63"), Muted: ui.Hex("#616C73"), Meta: ui.Hex("#616C73"),
		Border: ui.Hex("#DFE2DD"), Hair: ui.Hex("#E9EBE7"),
		Accent: ui.Hex("#315FBF"), AccentText: ui.Hex("#FFFFFF"),
		AccentSoft: ui.Hex("#E8EEF9"), AccentLine: ui.Hex("#C7D6F0"),
		Success: ui.Hex("#26734A"), SuccessSoft: ui.Hex("#EAF5EE"),
		Warning: ui.Hex("#87520A"), WarningSoft: ui.Hex("#FAF2E4"),
		Danger: ui.Hex("#A83531"), DangerSoft: ui.Hex("#FBEDEB"),
		AmberText: ui.Hex("#87520A"), AmberSoft: ui.Hex("#FAF2E4"), AmberLine: ui.Hex("#EEDCC0"),
		Shadow: ui.RGBA(0, 0, 0, 0.45), Step: ui.Hex("#000000"),
		TCP: ui.Hex("#315FBF"), UDP: ui.Hex("#7351A6"),
		HTTP: ui.Hex("#207569"), HTTPS: ui.Hex("#87520A"),
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
	if t.Dark {
		// The form's fields stand a half-step above the panel, not into
		// it. The night palette's text is never made dim to say disabled;
		// the disabled face keeps a readable ink and a quiet border.
		th.Surface = t.Surface3
		th.TextMuted = t.Muted
	}
	th.SurfaceHover, th.SurfacePressed = t.Surface2, t.Surface2
	th.Border, th.Text, th.TextMuted = t.Border, t.FG, t.Muted
	th.Accent, th.AccentText = t.Accent, t.AccentText
	th.Danger, th.Warning, th.Success = t.Danger, t.Warning, t.Success
	th.Scrollbar = t.Muted
	// Buttons, inputs and chips share the control radius and the plan's
	// standard height. Custom compact switches and icon buttons keep their
	// own geometry helpers.
	th.Radius, th.Spacing = RadiusButton, 6
	th.FontSize = BodySize
	if !t.Dark {
		// Tooltips and toasts are the theme turned over: the day ink on the
		// day wash.
		th.Inverse, th.InverseText = t.FG, t.Canvas
	}
	return th
}
