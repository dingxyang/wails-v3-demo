package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// Version is the running application version. Keep it in sync with
// build/config.yml and with GitHub release tags (a leading "v" is allowed).
var Version = "0.0.1"

// updateRepository is the public GitHub repository that hosts releases used by
// the auto-updater. Assets must contain the OS and architecture in their
// filename (e.g. wails-v3-demo-darwin-arm64.zip) so the updater can pick one.
const updateRepository = "dingxyang/wails-v3-demo"

func main() {
	app := application.New(application.Options{
		Name:        "简译",
		Description: "基于免费翻译服务的桌面翻译工具",
		Services: []application.Service{
			application.NewService(NewTranslationService()),
			application.NewService(&UpdateService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	setupUpdater(app)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "简译",
		Width:  1080,
		Height: 720,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupUpdater configures the framework updater against GitHub releases.
// Background checks are driven by UpdateService; the built-in update window is
// only opened when the user asks to update.
func setupUpdater(app *application.App) {
	provider, err := github.New(github.Config{
		Repository:    updateRepository,
		ChecksumAsset: "checksums.txt",
	})
	if err != nil {
		log.Printf("自动更新未启用: %v", err)
		return
	}

	err = app.Updater.Init(updater.Config{
		CurrentVersion: Version,
		Providers:      []updater.Provider{provider},
		Window: &updater.BuiltinWindow{
			Options: updater.WindowOptions{Title: "简译更新"},
		},
	})
	if err != nil {
		log.Printf("自动更新未启用: %v", err)
	}
}
