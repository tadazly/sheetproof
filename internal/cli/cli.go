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

	"github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/tadazly/sheetproof/internal/repository"
	"github.com/tadazly/sheetproof/internal/workbook"
	"github.com/xuri/excelize/v2"
)

const (
	ExitOK          = 0
	ExitRuntime     = 1
	ExitRead        = 2
	ExitSave        = 3
	ExitUnsupported = 4
	ExitCancelled   = 5
)

type Launcher func(left, right string, options app.Options) error

func Run(args []string, stdout, stderr io.Writer, launch Launcher) int {
	args, locale, localeExplicit, err := parseLanguage(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return ExitRuntime
	}
	if len(args) == 0 {
		if err := launch("", "", app.Options{Locale: string(locale), LocaleExplicit: localeExplicit}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitCode(err)
		}
		return ExitOK
	}
	if handled, code := runUGitSpreadsheetCompare(args, stderr, launch, locale, localeExplicit); handled {
		return code
	}
	switch args[0] {
	case "diff":
		return runDiff(args[1:], stdout, stderr, locale)
	case "compare":
		return runCompare(args[1:], stderr, launch, locale, localeExplicit)
	case "repo":
		return runRepository(args[1:], stderr, launch, locale, localeExplicit)
	case "help", "--help", "-h":
		printUsage(stdout, locale)
		return ExitOK
	case "version", "--version":
		fmt.Fprintf(stdout, "SheetProof %s\n", Version)
		return ExitOK
	default:
		fmt.Fprintf(stderr, cliMessage(locale, "unknownCommand"), args[0])
		printUsage(stderr, locale)
		return ExitRuntime
	}
}

func runRepository(args []string, stderr io.Writer, launch Launcher, locale localization.Locale, localeExplicit bool) int {
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
			fmt.Fprintf(stderr, cliMessage(locale, "repositoryFileMissing"), *file)
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
		Locale:         string(locale),
		LocaleExplicit: localeExplicit,
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

func runDiff(args []string, stdout, stderr io.Writer, locale localization.Locale) int {
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
	result := app.CompareSnapshots(leftSnapshot, rightSnapshot)
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
	fmt.Fprintf(stdout, cliMessage(locale, "diffSummary"),
		result.LeftFile, result.RightFile, result.Equal, result.SheetCount,
		result.DifferentSheetCount, result.DifferenceCount)
	for _, sheet := range result.Sheets {
		fmt.Fprintf(stdout, "%s\t%s\t%d\n", sheet.Name, sheet.Status, sheet.DifferenceCount)
	}
	return ExitOK
}

func runCompare(args []string, stderr io.Writer, launch Launcher, locale localization.Locale, localeExplicit bool) int {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	left := flags.String("left", "", "left .xlsx path")
	right := flags.String("right", "", "right .xlsx path")
	title := flags.String("title", "", "window title")
	leftLabel := flags.String("left-label", "", "left display label")
	rightLabel := flags.String("right-label", "", "right display label")
	readonly := flags.Bool("readonly-left", false, "disable edits and saving")
	base := flags.String("base", "", "merge base .xlsx path")
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
	hasMergeBaseArgument := strings.TrimSpace(*base) != ""
	mergeBase := ""
	if hasMergeBaseArgument && !isNullDevice(*base) {
		if err := workbook.ValidateXLSXPath(*base); err != nil {
			fmt.Fprintln(stderr, err)
			return exitCode(err)
		}
		mergeBase = *base
	}
	gitDiff := isGitDiffToolInvocation()
	resolvedLeftLabel, resolvedRightLabel := resolveCompareLabels(*leftLabel, *rightLabel)
	options := app.Options{
		Locale: string(locale), LocaleExplicit: localeExplicit,
		Title: *title, LeftLabel: resolvedLeftLabel, RightLabel: resolvedRightLabel,
		ReadonlyLeft: *readonly || gitDiff, GitDiff: gitDiff, Output: *output,
		GitMerge:  hasMergeBaseArgument,
		MergeBase: mergeBase,
	}
	if err := launch(preparedLeft, preparedRight, options); err != nil {
		fmt.Fprintln(stderr, err)
		return exitCode(err)
	}
	return ExitOK
}

// runUGitSpreadsheetCompare implements the direct path-list protocol used by
// UGit 5.51 for a diff tool named SpreadsheetCompare. UGit writes two absolute
// workbook paths to a temporary text file and starts the configured program
// with only that file as its argument. Unlike git difftool, this route launches
// even when Git has no byte-level difference to hand to an external command.
func runUGitSpreadsheetCompare(args []string, stderr io.Writer, launch Launcher, locale localization.Locale, localeExplicit bool) (bool, int) {
	if len(args) != 1 {
		return false, ExitRuntime
	}
	listPath := strings.TrimSpace(args[0])
	if !strings.HasPrefix(filepath.Base(listPath), "SpreadsheetCompare-") ||
		!strings.EqualFold(filepath.Ext(listPath), ".txt") {
		return false, ExitRuntime
	}
	data, err := os.ReadFile(listPath)
	if err != nil {
		fmt.Fprintf(stderr, cliMessage(locale, "ugitListReadFailed"), err)
		return true, ExitRead
	}
	var paths []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if value := strings.TrimSpace(line); value != "" {
			paths = append(paths, value)
		}
	}
	if len(paths) != 2 {
		fmt.Fprintln(stderr, cliMessage(locale, "ugitListNeedsTwo"))
		return true, ExitRuntime
	}
	if err := validateComparePaths(paths[0], paths[1]); err != nil {
		fmt.Fprintln(stderr, err)
		return true, exitCode(err)
	}
	if worktree, snapshot, snapshotLabel, ok := identifyUGitWorktreeComparison(listPath, paths[0], paths[1], locale); ok {
		options := app.Options{
			Locale: string(locale), LocaleExplicit: localeExplicit,
			LeftLabel: sourceLabel(locale, "worktree"), RightLabel: snapshotLabel,
			UGitWorktree: true,
		}
		if err := launch(worktree, snapshot, options); err != nil {
			fmt.Fprintln(stderr, err)
			return true, exitCode(err)
		}
		return true, ExitOK
	}
	rightLabel := sourceLabel(locale, "comparisonRevision")
	if !pathWithinDirectory(paths[1], filepath.Dir(listPath)) {
		rightLabel = sourceLabel(locale, "worktree")
	}
	options := app.Options{
		Locale: string(locale), LocaleExplicit: localeExplicit,
		LeftLabel: sourceLabel(locale, "selectedRevision"), RightLabel: rightLabel,
		ReadonlyLeft: true, GitDiff: true,
	}
	if err := launch(paths[0], paths[1], options); err != nil {
		fmt.Fprintln(stderr, err)
		return true, exitCode(err)
	}
	return true, ExitOK
}

func identifyUGitWorktreeComparison(listPath, snapshotPath, workspacePath string, locale localization.Locale) (
	worktree string,
	snapshot string,
	snapshotLabel string,
	ok bool,
) {
	listDir := filepath.Dir(listPath)
	snapshotName := strings.ToLower(filepath.Base(snapshotPath))
	if !strings.HasPrefix(snapshotName, "temp-") ||
		!pathWithinDirectory(snapshotPath, listDir) ||
		pathWithinDirectory(workspacePath, listDir) {
		return "", "", "", false
	}
	worktreeFile, err := repository.IdentifyWorktreeFile(workspacePath)
	if err != nil {
		return "", "", "", false
	}
	expectedDiffDir := filepath.Join(worktreeFile.GitDirectory, "ugit", "diff")
	if !sameExistingDirectory(listDir, expectedDiffDir) {
		return "", "", "", false
	}
	label := sourceLabel(locale, "selectedRevision")
	if strings.HasPrefix(strings.ToUpper(filepath.Base(snapshotPath)), "TEMP-HEAD-") {
		label = "HEAD"
	}
	return worktreeFile.Path, snapshotPath, label, true
}

func sourceLabel(locale localization.Locale, key string) string {
	labels := map[localization.Locale]map[string]string{
		localization.English: {
			"worktree":           "Current worktree",
			"selectedRevision":   "Selected revision",
			"comparisonRevision": "Comparison revision",
		},
		localization.SimplifiedChinese: {
			"worktree":           "当前工作区",
			"selectedRevision":   "所选版本",
			"comparisonRevision": "对比版本",
		},
		localization.Japanese: {
			"worktree":           "現在のワークツリー",
			"selectedRevision":   "選択したリビジョン",
			"comparisonRevision": "比較対象のリビジョン",
		},
	}
	return labels[locale][key]
}

func sameExistingDirectory(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	return leftErr == nil && rightErr == nil && leftInfo.IsDir() && rightInfo.IsDir() && os.SameFile(leftInfo, rightInfo)
}

func pathWithinDirectory(path, directory string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDirectory, err := filepath.Abs(directory)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(absDirectory, absPath)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
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

	tempDir, err := os.MkdirTemp("", "sheetproof-git-null-*")
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

func printUsage(w io.Writer, locale localization.Locale) {
	fmt.Fprintln(w, cliMessage(locale, "usage"))
}

func parseLanguage(args []string) ([]string, localization.Locale, bool, error) {
	locale := localization.FromEnvironment()
	explicit := false
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		value := ""
		switch {
		case argument == "--lang":
			if index+1 >= len(args) {
				return nil, locale, explicit, errors.New("--lang requires en, zh-CN, or ja")
			}
			index++
			value = args[index]
		case strings.HasPrefix(argument, "--lang="):
			value = strings.TrimPrefix(argument, "--lang=")
		default:
			result = append(result, argument)
			continue
		}
		parsed, err := localization.Parse(value)
		if err != nil {
			return nil, locale, explicit, err
		}
		locale = parsed
		explicit = true
	}
	return result, locale, explicit, nil
}

func cliMessage(locale localization.Locale, key string) string {
	translations := map[localization.Locale]map[string]string{
		localization.English: {
			"unknownCommand":        "unknown command %q\n",
			"diffSummary":           "left: %s\nright: %s\nequal: %t\nsheets: %d, different sheets: %d, cell differences: %d\n",
			"repositoryFileMissing": "the repository does not contain the requested XLSX file: %s\n",
			"ugitListReadFailed":    "read UGit diff file list: %v\n",
			"ugitListNeedsTwo":      "the UGit diff file list must contain exactly two XLSX paths",
			"usage": `SheetProof - review and apply changes in Excel .xlsx workbooks

Usage:
  sheetproof compare [--lang en|zh-CN|ja] [--left FILE --right FILE] [--title TEXT] [--left-label TEXT] [--right-label TEXT] [--readonly-left] [--output FILE]
  sheetproof repo [--lang en|zh-CN|ja] --path DIRECTORY [--file RELATIVE.xlsx] [--ref BRANCH]
  sheetproof diff [--lang en|zh-CN|ja] --left FILE --right FILE [--format json|text]
  sheetproof`,
		},
		localization.SimplifiedChinese: {
			"unknownCommand":        "未知命令 %q\n",
			"diffSummary":           "左侧：%s\n右侧：%s\n相同：%t\n工作表：%d，不同工作表：%d，单元格差异：%d\n",
			"repositoryFileMissing": "仓库中不存在指定的 XLSX 文件：%s\n",
			"ugitListReadFailed":    "读取 UGit 差异文件列表失败：%v\n",
			"ugitListNeedsTwo":      "UGit 差异文件列表必须恰好包含两个 XLSX 路径",
			"usage": `SheetProof - 审阅并应用 Excel .xlsx 工作簿修改

用法：
  sheetproof compare [--lang en|zh-CN|ja] [--left 文件 --right 文件] [选项]
  sheetproof repo [--lang en|zh-CN|ja] --path 目录 [--file 相对路径.xlsx] [--ref 引用]
  sheetproof diff [--lang en|zh-CN|ja] --left 文件 --right 文件 [--format json|text]
  sheetproof`,
		},
		localization.Japanese: {
			"unknownCommand":        "不明なコマンド %q\n",
			"diffSummary":           "左：%s\n右：%s\n同一：%t\nシート：%d、差分のあるシート：%d、セル差分：%d\n",
			"repositoryFileMissing": "リポジトリに指定された XLSX ファイルがありません：%s\n",
			"ugitListReadFailed":    "UGit の差分ファイル一覧を読み込めませんでした：%v\n",
			"ugitListNeedsTwo":      "UGit の差分ファイル一覧には XLSX のパスが 2 件必要です",
			"usage": `SheetProof - Excel .xlsx ブックの差分確認と反映

使用方法：
  sheetproof compare [--lang en|zh-CN|ja] [--left ファイル --right ファイル] [オプション]
  sheetproof repo [--lang en|zh-CN|ja] --path ディレクトリ [--file 相対パス.xlsx] [--ref 参照]
  sheetproof diff [--lang en|zh-CN|ja] --left ファイル --right ファイル [--format json|text]
  sheetproof`,
		},
	}
	if value := translations[locale][key]; value != "" {
		return value
	}
	return translations[localization.English][key]
}
