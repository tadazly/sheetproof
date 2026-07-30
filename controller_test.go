package main

import (
	"context"
	"testing"
	"time"

	coreapp "github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/testutil"
)

func TestControllerAsyncBootstrapAndViewportAPI(t *testing.T) {
	pair, err := testutil.CreatePair(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	controller := NewController(pair.Left, pair.Right, coreapp.Options{})
	controller.startup(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for controller.Bootstrap().Loading && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := controller.Bootstrap()
	if state.Loading || !state.HasSession || state.Error != "" {
		t.Fatalf("bootstrap state = %+v", state)
	}
	summary, err := controller.Summary()
	if err != nil || summary.Diff.DifferenceCount != 7 {
		t.Fatalf("summary = %+v, err=%v", summary.Diff, err)
	}
	region, err := controller.Region("数据 表", 1, 10, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(region.Cells) != 100 {
		t.Fatalf("region cells = %d", len(region.Cells))
	}
	if _, err := controller.CopyRightToLeft("数据 表", 1, 1); err != nil {
		t.Fatal(err)
	}
	controller.shutdown(context.Background())
}
