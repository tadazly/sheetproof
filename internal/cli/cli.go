package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/diff"
	"github.com/ug-tools/ugxlsx/internal/workbook"
)

const (
	Version = "0.1.0"

	ExitOK          = 0
	ExitRuntime     = 1
	ExitRead        = 2
	ExitSave        = 3
	ExitUnsupported = 4
	ExitCancelled   = 5
)

type Launcher func(left, right string, options app.Options) error

func Run(args []string, stdout, stderr io.Writer, launch Launcher) int {
	if len(args) == 0 {
		if err := launch("", "", app.Options{}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitCode(err)
		}
		return ExitOK
	}
	switch args[0] {
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stderr, launch)
	case "help", "--help", "-h":
		printUsage(stdout)
		return ExitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "ugxlsx %s\n", Version)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return ExitRuntime
	}
}

func runDiff(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	left := flags.String("left", "", "left .xlsx path")
	right := flags.String("right", "", "right .xlsx path")
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return ExitRuntime
	}
	if *left == "" || *right == "" {
		fmt.Fprintln(stderr, "--left and --right are required")
		return ExitRuntime
	}
	if *format != "json" && *format != "text" {
		fmt.Fprintln(stderr, "--format must be json or text")
		return ExitRuntime
	}
	same, err := workbook.SamePath(*left, *right)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	if same {
		fmt.Fprintln(stderr, "left and right paths must be different")
		return ExitRuntime
	}
	reader := workbook.Reader{}
	leftFile, leftSnapshot, err := reader.Open(*left)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	defer leftFile.Close()
	rightFile, rightSnapshot, err := reader.Open(*right)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	defer rightFile.Close()
	result := diff.Compare(leftSnapshot, rightSnapshot)
	for _, sheet := range result.Sheets {
		sheet.Differences = nil
	}
	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return ExitRuntime
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "left: %s\nright: %s\nequal: %t\nsheets: %d, different sheets: %d, cell differences: %d\n",
		result.LeftFile, result.RightFile, result.Equal, result.SheetCount,
		result.DifferentSheetCount, result.DifferenceCount)
	for _, sheet := range result.Sheets {
		fmt.Fprintf(stdout, "%s\t%s\t%d\n", sheet.Name, sheet.Status, sheet.DifferenceCount)
	}
	return ExitOK
}

func runCompare(args []string, stderr io.Writer, launch Launcher) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	left := flags.String("left", "", "left .xlsx path")
	right := flags.String("right", "", "right .xlsx path")
	title := flags.String("title", "", "window title")
	leftLabel := flags.String("left-label", "", "left display label")
	rightLabel := flags.String("right-label", "", "right display label")
	readonly := flags.Bool("readonly-left", false, "disable edits and saving")
	output := flags.String("output", "", "default save target")
	if err := flags.Parse(args); err != nil {
		return ExitRuntime
	}
	if (*left == "") != (*right == "") {
		fmt.Fprintln(stderr, "--left and --right must be provided together")
		return ExitRuntime
	}
	if *left != "" {
		if err := validateComparePaths(*left, *right); err != nil {
			fmt.Fprintln(stderr, err)
			return exitCode(err)
		}
	}
	options := app.Options{
		Title: *title, LeftLabel: *leftLabel, RightLabel: *rightLabel,
		ReadonlyLeft: *readonly, Output: *output,
	}
	if err := launch(*left, *right, options); err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	return ExitOK
}

func validateComparePaths(left, right string) error {
	if err := workbook.ValidateXLSXPath(left); err != nil {
		return err
	}
	if err := workbook.ValidateXLSXPath(right); err != nil {
		return err
	}
	if same, err := workbook.SamePath(left, right); err != nil {
		return err
	} else if same {
		return &workbook.Error{Code: workbook.ErrSameFile, Path: left, Err: errors.New("left and right paths refer to the same file")}
	}
	for _, path := range []string{left, right} {
		if err := (workbook.Reader{}).Validate(path); err != nil {
			return err
		}
	}
	if strings.TrimSpace(filepath.Ext(left)) == "" {
		return errors.New("left path has no extension")
	}
	return nil
}

func exitCode(err error) int {
	switch {
	case workbook.HasCode(err, workbook.ErrUnsupported):
		return ExitUnsupported
	case workbook.HasCode(err, workbook.ErrNotFound),
		workbook.HasCode(err, workbook.ErrUnreadable),
		workbook.HasCode(err, workbook.ErrCorrupt),
		workbook.HasCode(err, workbook.ErrNoSheets):
		return ExitRead
	case workbook.HasCode(err, workbook.ErrSave),
		workbook.HasCode(err, workbook.ErrExternalEdit),
		workbook.HasCode(err, workbook.ErrUnwritable):
		return ExitSave
	default:
		return ExitRuntime
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `ugxlsx - compare and merge Excel .xlsx workbooks

Usage:
  ugxlsx compare [--left FILE --right FILE] [--title TEXT] [--left-label TEXT] [--right-label TEXT] [--readonly-left] [--output FILE]
  ugxlsx diff --left FILE --right FILE [--format json|text]
  ugxlsx`)
}
