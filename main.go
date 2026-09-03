package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	coreapp "github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/cli"
	"github.com/tadazly/sheetproof/internal/diagnostics"
	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	windowsoptions "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	diagnosticLog := processDiagnosticLogger()
	diagnosticLog.Record("process.start", executableDiagnosticFields())
	defer func() {
		if recovered := recover(); recovered != nil {
			recordPanic(diagnosticLog, "process.panic", recovered)
			panic(recovered)
		}
	}()
	code := cli.Run(os.Args[1:], os.Stdout, os.Stderr, func(left, right string, appOptions coreapp.Options) error {
		return launchGUIWithDiagnostics(left, right, appOptions, diagnosticLog)
	})
	diagnosticLog.Record("process.exit", map[string]any{"code": code})
	os.Exit(code)
}

func launchGUI(left, right string, appOptions coreapp.Options) error {
	diagnosticLog := processDiagnosticLogger()
	diagnosticLog.Record("process.start", executableDiagnosticFields())
	err := launchGUIWithDiagnostics(left, right, appOptions, diagnosticLog)
	code := cli.ExitOK
	if err != nil {
		code = cli.ExitRuntime
	}
	diagnosticLog.Record("process.exit", map[string]any{"code": code})
	return err
}

func launchGUIWithDiagnostics(left, right string, appOptions coreapp.Options, diagnosticLog *diagnostics.Logger) error {
	diagnosticLog.Record("app.start", map[string]any{
		"leftProvided":       left != "",
		"rightProvided":      right != "",
		"repositoryProvided": appOptions.RepositoryPath != "",
	})
	defer func() {
		if recovered := recover(); recovered != nil {
			recordPanic(diagnosticLog, "app.panic", recovered)
			panic(recovered)
		}
	}()
	controller := NewController(left, right, appOptions)
	controller.diagnosticLog = diagnosticLog
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
		Windows:          windowsWebviewOptions(),
		Logger:           diagnosticLog,
		OnStartup:        controller.startup,
		OnDomReady: func(context.Context) {
			applyPlatformWindowIcon()
		},
		OnShutdown:    controller.shutdown,
		OnBeforeClose: controller.beforeClose,
		Bind:          []interface{}{controller},
	}); err != nil {
		diagnosticLog.Record("app.run.error", map[string]any{"kind": fmt.Sprintf("%T", err)})
		return fmt.Errorf("start GUI: %w", err)
	}
	diagnosticLog.Record("app.stop", nil)
	return nil
}

func processDiagnosticLogger() *diagnostics.Logger {
	executable, err := os.Executable()
	if err == nil && isWailsBindingsExecutable(filepath.Base(executable)) {
		return diagnostics.NewDiscard()
	}
	return diagnostics.NewDefault()
}

func isWailsBindingsExecutable(name string) bool {
	extension := filepath.Ext(name)
	base := strings.TrimSuffix(name, extension)
	return strings.EqualFold(base, "wailsbindings")
}

func recordPanic(diagnosticLog *diagnostics.Logger, event string, recovered any) {
	stack := string(debug.Stack())
	if len(stack) > 8192 {
		stack = stack[:8192]
	}
	diagnosticLog.Record(event, map[string]any{
		"type":  fmt.Sprintf("%T", recovered),
		"stack": stack,
	})
}

func executableDiagnosticFields() map[string]any {
	fields := map[string]any{
		"version":   cli.Version,
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"pid":       os.Getpid(),
	}
	executable, err := os.Executable()
	if err != nil {
		fields["executableIdentity"] = "unavailable"
		return fields
	}
	fields["executable"] = filepath.Base(executable)
	if info, statErr := os.Stat(executable); statErr == nil {
		fields["executableBytes"] = info.Size()
		fields["executableModified"] = info.ModTime().UTC().Format("2006-01-02T15:04:05.000000000Z")
	}
	file, err := os.Open(executable)
	if err != nil {
		fields["executableSha256"] = "unavailable"
		return fields
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		fields["executableSha256"] = "unavailable"
		return fields
	}
	fields["executableSha256"] = fmt.Sprintf("%x", hash.Sum(nil))
	return fields
}

func windowsWebviewOptions() *windowsoptions.Options {
	return &windowsoptions.Options{
		IsZoomControlEnabled: false,
		DisablePinchZoom:     true,
	}
}

func dragAndDropOptions() *options.DragAndDrop {
	return &options.DragAndDrop{
		EnableFileDrop:     true,
		DisableWebViewDrop: false,
	}
}
