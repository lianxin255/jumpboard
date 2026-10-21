// NginxJump keeps nginx forwarding rules for a list of services on the
// internal network, and runs a bundled nginx to serve them.
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

	"github.com/lianxin255/nginx-jump-gui/internal/appt"
	"github.com/lianxin255/nginx-jump-gui/internal/model"
	"github.com/lianxin255/nginx-jump-gui/internal/paths"
	"github.com/lianxin255/nginx-jump-gui/internal/services"
	njui "github.com/lianxin255/nginx-jump-gui/internal/ui"
)

// builtin holds the templates shipped with the app.
//
//go:embed internal/services/builtin/*.yml
var builtin embed.FS

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
	// The native file dialogs too.
	app.SaveDialog = mygo.Dialog.Save
	app.OpenDialog = mygo.Dialog.Open

	// The engine is started here, before the window appears, so that a
	// failure is known when the first frame draws.
	app.OnChange = func() {} // replaced below, once the window exists

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "NginxJump",
			Width:           1200,
			Height:          760,
			MinWidth:        900,
			MinHeight:       600,
			BackgroundColor: "light-dark(#faf9f5, #141413)",
			StateKey:        "main",
			Content:         ui.View(njui.NewView(app)),
		})
		app.OnChange = func() { win.Update(func() {}) }
		app.Change()

		// The checker reports status changes from its own goroutine; the
		// window asks for a frame, and the frame reads the new state.
		app.Check.Start()

		// Quitting must not leave nginx behind: stop it before the
		// process exits, and close the database after.
		mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
			app.Close()
		})
		win.OnClosed(func() {
			mygo.App.Quit()
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
		Message: "NginxJump 启动失败",
		Detail:  msg,
	})
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
