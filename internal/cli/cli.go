package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/diff"
	"github.com/ug-tools/ugxlsx/internal/repository"
	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/xuri/excelize/v2"
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
	case "repo":
		return runRepository(args[1:], stderr, launch)
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

func runRepository(args []string, stderr io.Writer, launch Launcher) int {
	flags := flag.NewFlagSet("repo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("path", "", "local Git repository path")
	file := flags.String("file", "", "repository-relative .xlsx path")
	ref := flags.String("ref", "", "local or remote comparison ref")
	if err := flags.Parse(args); err != nil {
		return ExitRuntime
	}
	if *path == "" {
		fmt.Fprintln(stderr, "--path is required")
		return ExitRuntime
	}
	repo, info, err := repository.Open(*path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitRuntime
	}
	normalizedFile := filepath.ToSlash(*file)
	if normalizedFile != "" {
		found := false
		for _, candidate := range info.Files {
			if candidate == normalizedFile {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(stderr, "仓库中不存在指定的 XLSX 文件: %s\n", *file)
			return ExitRead
		}
		if _, err := repo.ResolveRelativePath(normalizedFile); err != nil {
			fmt.Fprintln(stderr, err)
			return ExitRuntime
		}
	}
	fullRef := ""
	if *ref != "" {
		branch, err := repo.ResolveReference(*ref, info.Branches)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return ExitRuntime
		}
		fullRef = branch.FullName
	}
	options := app.Options{
		RepositoryPath: info.Root,
		RepositoryFile: normalizedFile,
		RepositoryRef:  fullRef,
	}
	if err := launch("", "", options); err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	return ExitOK
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
	preparedLeft, preparedRight := *left, *right
	cleanup := func() {}
	if *left != "" {
		var err error
		preparedLeft, preparedRight, cleanup, err = prepareComparePaths(*left, *right)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitCode(err)
		}
		defer cleanup()
	}
	gitDiff := isGitDiffToolInvocation()
	resolvedLeftLabel, resolvedRightLabel := resolveCompareLabels(*leftLabel, *rightLabel)
	options := app.Options{
		Title: *title, LeftLabel: resolvedLeftLabel, RightLabel: resolvedRightLabel,
		ReadonlyLeft: *readonly || gitDiff, GitDiff: gitDiff, Output: *output,
	}
	if err := launch(preparedLeft, preparedRight, options); err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	return ExitOK
}

// resolveCompareLabels uses UGit's per-invocation reference titles when the
// caller has not supplied an explicit display label. These variables are
// display-only metadata; Git itself does not define them.
func resolveCompareLabels(leftLabel, rightLabel string) (string, string) {
	if strings.TrimSpace(leftLabel) == "" {
		leftLabel = strings.TrimSpace(os.Getenv("LOCAL_TITLE"))
	}
	if strings.TrimSpace(rightLabel) == "" {
		rightLabel = strings.TrimSpace(os.Getenv("REMOTE_TITLE"))
	}
	return leftLabel, rightLabel
}

func isGitDiffToolInvocation() bool {
	counter := strings.TrimSpace(os.Getenv("GIT_DIFF_PATH_COUNTER"))
	total := strings.TrimSpace(os.Getenv("GIT_DIFF_PATH_TOTAL"))
	return counter != "" && total != ""
}

// prepareComparePaths adapts the null device used by Git difftool for an added
// or deleted file into a real, temporary XLSX workbook. The placeholder mirrors
// the existing workbook's sheet names but contains no cells, so the normal
// session/diff/merge code can represent every present cell as added or deleted.
func prepareComparePaths(left, right string) (preparedLeft, preparedRight string, cleanup func(), err error) {
	leftMissing := isNullDevice(left)
	rightMissing := isNullDevice(right)
	if !leftMissing && !rightMissing {
		if err := validateComparePaths(left, right); err != nil {
			return "", "", func() {}, err
		}
		return left, right, func() {}, nil
	}
	if leftMissing && rightMissing {
		return "", "", func() {}, errors.New("left and right paths cannot both be the null device")
	}

	existing := left
	missingSide := "right"
	if leftMissing {
		existing = right
		missingSide = "left"
	}
	if err := workbook.ValidateXLSXPath(existing); err != nil {
		return "", "", func() {}, err
	}

	tempDir, err := os.MkdirTemp("", "ugxlsx-git-null-*")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("create Git diff placeholder: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tempDir) }
	placeholder := filepath.Join(tempDir, "missing-"+missingSide+".xlsx")
	if err := createEmptyWorkbookLike(existing, placeholder); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	preparedLeft, preparedRight = left, right
	if leftMissing {
		preparedLeft = placeholder
	} else {
		preparedRight = placeholder
	}
	if err := validateComparePaths(preparedLeft, preparedRight); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	return preparedLeft, preparedRight, cleanup, nil
}

func isNullDevice(path string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "/dev/null" {
		return true
	}
	switch strings.ToUpper(trimmed) {
	case "NUL", "NUL:":
		return true
	default:
		return false
	}
}

func createEmptyWorkbookLike(sourcePath, targetPath string) error {
	sourceFile, snapshot, err := (workbook.Reader{}).Open(sourcePath)
	if err != nil {
		return err
	}
	if err := sourceFile.Close(); err != nil {
		return err
	}

	placeholder := excelize.NewFile()
	defer placeholder.Close()
	defaultSheet := placeholder.GetSheetName(0)
	for index, sheet := range snapshot.Sheets {
		if index == 0 {
			if sheet.Name != defaultSheet {
				if err := placeholder.SetSheetName(defaultSheet, sheet.Name); err != nil {
					return fmt.Errorf("create Git diff placeholder: %w", err)
				}
			}
			continue
		}
		if _, err := placeholder.NewSheet(sheet.Name); err != nil {
			return fmt.Errorf("create Git diff placeholder: %w", err)
		}
	}
	if err := placeholder.SaveAs(targetPath); err != nil {
		return fmt.Errorf("create Git diff placeholder: %w", err)
	}
	return nil
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
  ugxlsx repo --path DIRECTORY [--file RELATIVE.xlsx] [--ref BRANCH]
  ugxlsx diff --left FILE --right FILE [--format json|text]
  ugxlsx`)
}
