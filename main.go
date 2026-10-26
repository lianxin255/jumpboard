// JumpBoard keeps intranet forwarding rules for a list of services on the
// internal network, and serves them itself, in this process.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui" // the toolkit's own strings in Chinese

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/native"
	"github.com/lianxin255/jumpboard/internal/paths"
	"github.com/lianxin255/jumpboard/internal/services"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// builtin holds the templates shipped with the app.
//
//go:embed internal/services/builtin/*.yml
var builtin embed.FS

// appIcon is the app's mark, for the tray on platforms without template
// icons.
//
//go:embed resources/icon.png
var appIcon []byte

func main() {
	// The toolkit's controls, menus and dialogs say their strings in the
	// app's language. The language has to be declared before the first
	// window exists, or the controls keep English: setting the locale
	// alone is not enough.
	mygo.App.SetLanguages("zh-Hans", "en")

	// A second copy would try to bind the same ports, and the first one
	// already owns them: bring the running window forward instead.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	p, err := paths.Resolve()
	if err != nil {
		fail("无法准备数据目录", err)
	}

	app, err := appt.New(p)
	if err != nil {
		fail("启动失败", err)
	}
	app.Tpl = services.NewTemplates(mustBuiltinTemplates(), p.DataDir+"/templates/custom")
	// The clipboard needs the running app, so it is wired here rather than
	// inside the app's own code.
	app.Clipboard = mygo.Clipboard.WriteText
	// The desktop notifier too: it silently does nothing until the app is
	// packaged, so the behaviour holds in development and in tests.
	app.Notify = native.Notify
	// The native file dialogs too.
	app.SaveDialog = mygo.Dialog.Save
	app.OpenDialog = mygo.Dialog.Open
	// The sidebar's material follows the toolkit's theme source, which
	// otherwise stays on the desktop's appearance. The saved choice has
	// to be in place before the window exists, or its first frame shows
	// the desktop's material. Later changes go through the same call.
	njui.ApplyAppearance(app.Theme())
	app.OnTheme = njui.ApplyAppearance

	// The engine is started here, before the window appears, so that a
	// failure is known when the first frame draws.
	app.OnChange = func() {} // replaced below, once the window exists

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "JumpBoard",
			Width:           1280,
			Height:          800,
			MinWidth:        960,
			MinHeight:       640,
			BackgroundColor: "light-dark(#FBFCFE, #14161c)",
			StateKey:        "main",
			// Immersive: the view reaches the top of the window and draws
			// its own bar; the traffic lights float over the sidebar. The
			// frame reads c.TitleBar() to keep clear of the controls.
			TitleBarStyle: mygo.TitleBarHiddenInset,
			// The sidebar material shows wherever the view draws no
			// background (macOS, Windows 11 22H2). Elsewhere the sidebar
			// paints its own frost color; the layout does not change.
			Vibrancy: mygo.VibrancySidebar,
			// Centred beside the brand mark, clear of the traffic lights.
			TrafficLightPosition: &mygo.Point{X: 16, Y: 20},
			Content:              ui.View(njui.NewView(app)),
		})
		app.OnChange = func() { win.Update(func() {}) }
		app.Change()

		// The checker reports status changes from its own goroutine; the
		// window asks for a frame, and the frame reads the new state.
		app.Check.Start()

		// The tray, the close-to-tray behaviour and the global shortcut.
		// Dock.Setup installs its own quit hooks; the engine shutdown
		// below runs after them through the same OnBeforeQuit list.
		native.Setup(app, win, appIcon)

		// Quitting must not leave nginx behind: stop it before the
		// process exits, and close the database after. Closing the window
		// no longer quits — it hides into the tray — so the real quit
		// paths are the tray's 退出 and Cmd+Q.
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			app.Close()
		})
	})

	if err := mygo.App.Run(); err != nil {
		fail("运行失败", err)
	}
}

// mustBuiltinTemplates parses the embedded YAML templates.
func mustBuiltinTemplates() map[string]model.Template {
	files := map[string][]byte{}
	entries, err := fs.ReadDir(builtin, "internal/services/builtin")
	if err != nil {
		log.Fatalf("读取内置模板: %v", err)
	}
	for _, e := range entries {
		data, err := builtin.ReadFile("internal/services/builtin/" + e.Name())
		if err != nil {
			log.Fatalf("读取内置模板 %s: %v", e.Name(), err)
		}
		files[e.Name()] = data
	}
	tpls, err := services.LoadBuiltin(files)
	if err != nil {
		log.Fatalf("解析内置模板: %v", err)
	}
	return tpls
}

// fail reports a startup problem the user can act on, using the system's
// own dialog: the app has no window yet to show it in.
func fail(title string, err error) {
	msg := fmt.Sprintf("%s\n\n%v", title, err)
	mygo.Dialog.Message(mygo.MessageOptions{
		Type:    mygo.MessageError,
		Message: "JumpBoard 启动失败",
		Detail:  msg,
	})
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
