//go:build !windows

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func showChoiceDialog(ctx context.Context, options runtime.MessageDialogOptions) (string, error) {
	return runtime.MessageDialog(ctx, options)
}
