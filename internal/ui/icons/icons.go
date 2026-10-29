// Package icons holds the Relay Workbench's icon family: the hand-drawn
// 24×24 marks the shell, the pages and the dialogs share.
//
// Every asset speaks one voice: a 1.8-unit stroke with round caps and
// joins, and paths that carry no color of their own — they draw in
// currentColor, so ui.Icon tints each mark with the color of the text
// beside it. Nothing here uses what the toolkit's SVG renderer leaves
// out: no text, filters, gradients or markers. Start and Stop are the
// only filled shapes; transport buttons read by their solid silhouette.
//
// The files are embedded and parsed once, in package initialization. A
// broken asset panics at startup instead of misdrawing in a view.
package icons

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed *.svg
var assets embed.FS

// icon parses one embedded asset. The names are fixed at build time, so
// a missing file is a programming error that must not survive startup.
func icon(name string) *ui.SVG {
	data, err := assets.ReadFile(name)
	if err != nil {
		panic("icons: " + err.Error())
	}
	return ui.MustParseSVG(data)
}

// Where the workbench navigates: Relay is the app mark itself, a
// simplified cut of resources/icon.svg — two endpoint blocks joined by
// a directional connector.
var (
	Relay    = icon("relay.svg")
	Logs     = icon("logs.svg")
	History  = icon("history.svg")
	Terminal = icon("terminal.svg")
	Chat     = icon("chat.svg")
	Settings = icon("settings.svg")
)

// Connector is Relay's endpoint motif without the arrowhead: the two
// endpoint blocks joined by one horizontal line. It marks the link a
// literal → beside it already names, so it reads as endpoints and their
// join at 20×12 rather than as a second arrow.
var Connector = icon("connector.svg")

// What a page does to its content.
var (
	Search   = icon("search.svg")
	Activity = icon("activity.svg")
	Edit     = icon("edit.svg")
	Delete   = icon("delete.svg")
)

// What a control says about state. Unchecked is the empty box Check
// completes; Start and Stop are filled, as transport buttons are.
var (
	Check     = icon("check.svg")
	Alert     = icon("alert.svg")
	Unchecked = icon("unchecked.svg")
	Start     = icon("start.svg")
	Stop      = icon("stop.svg")
)
