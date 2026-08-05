package main

import (
	"path/filepath"
	"testing"

	coreapp "github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/localization"
)

func TestGeneratedScenariosKeepLocalizedHeadersAndDifferenceCounts(t *testing.T) {
	for _, locale := range localization.SupportedLocales {
		dir := filepath.Join(t.TempDir(), string(locale))
		if err := generateLocale(dir, locale); err != nil {
			t.Fatal(err)
		}
		assertScenarioCount(t, dir, "balance_worktree.xlsx", "balance_origin-main.xlsx", locale, 16)
		assertScenarioCount(t, dir, "balance_position-left.xlsx", "balance_position-right.xlsx", locale, 90)
		assertScenarioCount(t, dir, "balance_merged-left.xlsx", "balance_origin-main.xlsx", locale, 15)
	}
}

func assertScenarioCount(t *testing.T, dir, leftName, rightName string, locale localization.Locale, want int) {
	t.Helper()
	session, err := coreapp.Open(
		filepath.Join(dir, leftName),
		filepath.Join(dir, rightName),
		coreapp.Options{Locale: string(locale)},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	summary := session.Summary()
	if len(summary.Diff.Sheets) == 0 || summary.Diff.Sheets[0].DifferenceCount != want {
		t.Fatalf("%s %s/%s first-sheet differences = %+v, want %d", locale, leftName, rightName, summary.Diff.Sheets, want)
	}
}
