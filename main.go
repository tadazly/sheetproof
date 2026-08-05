package main

import (
	"context"
	"embed"
	"fmt"
	"os"

	coreapp "github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/cli"
	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	code := cli.Run(os.Args[1:], os.Stdout, os.Stderr, launchGUI)
	os.Exit(code)
}

func launchGUI(left, right string, appOptions coreapp.Options) error {
	controller := NewController(left, right, appOptions)
	title := appOptions.Title
	if title == "" {
		switch localization.Normalize(appOptions.Locale) {
		case localization.SimplifiedChinese:
			title = "SheetProof — XLSX 差异审阅"
		case localization.Japanese:
			title = "SheetProof — XLSX 差分レビュー"
		default:
			title = "SheetProof — XLSX diff review"
		}
	}
	if err := wails.Run(&options.App{
		Title: title, Width: 1440, Height: 900, MinWidth: 960, MinHeight: 640,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 247, G: 248, B: 250, A: 1},
		DragAndDrop:      dragAndDropOptions(),
		OnStartup:        controller.startup,
		OnDomReady: func(context.Context) {
			applyPlatformWindowIcon()
		},
		OnShutdown:    controller.shutdown,
		OnBeforeClose: controller.beforeClose,
		Bind:          []interface{}{controller},
	}); err != nil {
		return fmt.Errorf("start GUI: %w", err)
	}
	return nil
}

func dragAndDropOptions() *options.DragAndDrop {
	return &options.DragAndDrop{
		EnableFileDrop:     true,
		DisableWebViewDrop: false,
	}
}
