package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	coreapp "github.com/tadazly/sheetproof/internal/app"
	"github.com/tadazly/sheetproof/internal/diff"
	"github.com/tadazly/sheetproof/internal/localization"
	"github.com/tadazly/sheetproof/internal/preferences"
	"github.com/tadazly/sheetproof/internal/repository"
	"github.com/tadazly/sheetproof/internal/ugit"
	"github.com/tadazly/sheetproof/internal/workbook"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	modeFiles      = "files"
	modeRepository = "repository"
)

type RepositoryView struct {
	Name               string              `json:"name"`
	Path               string              `json:"path"`
	CurrentBranch      string              `json:"currentBranch"`
	Detached           bool                `json:"detached"`
	WorkspaceDirty     bool                `json:"workspaceDirty"`
	Operation          string              `json:"operation"`
	Files              []string            `json:"files"`
	DifferenceFiles    []string            `json:"differenceFiles"`
	DifferenceIndexing bool                `json:"differenceIndexing"`
	Branches           []repository.Branch `json:"branches"`
	DefaultRef         string              `json:"defaultRef"`
	SelectedFile       string              `json:"selectedFile"`
	SelectedRef        string              `json:"selectedRef"`
	LeftState          string              `json:"leftState"`
	RightState         string              `json:"rightState"`
	LeftMessage        string              `json:"leftMessage"`
	RightMessage       string              `json:"rightMessage"`
	FileModified       bool                `json:"fileModified"`
	SidebarWidth       int                 `json:"sidebarWidth"`
	Notice             string              `json:"notice"`
	Loading            bool                `json:"loading"`
	LoadGeneration     uint64              `json:"loadGeneration"`
	ComparisonActive   bool                `json:"comparisonActive"`
}

type RepositoryResult struct {
	Repository RepositoryView   `json:"repository"`
	Summary    *coreapp.Summary `json:"summary"`
}

type RecentRepository struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

type Controller struct {
	mu                        sync.Mutex
	loadMu                    sync.Mutex
	prefsMu                   sync.Mutex
	wg                        sync.WaitGroup
	ctx                       context.Context
	session                   *coreapp.Session
	left                      string
	right                     string
	options                   coreapp.Options
	loading                   bool
	loadErr                   string
	mode                      string
	prefs                     preferences.Store
	repo                      *repository.Repository
	repositoryView            RepositoryView
	tempRight                 string
	loadGeneration            uint64
	differenceIndexGeneration uint64
	differenceIndexCancel     context.CancelFunc
	shuttingDown              bool
	switchPrompt              func() (string, error)
	dialog                    func(context.Context, runtime.MessageDialogOptions) (string, error)
}

func NewController(left, right string, options coreapp.Options) *Controller {
	return &Controller{
		left: left, right: right, options: options,
		prefs: preferences.NewStore(), dialog: showChoiceDialog,
	}
}

func (c *Controller) ask(ctx context.Context, options runtime.MessageDialogOptions) (string, error) {
	if c.dialog != nil {
		return c.dialog(ctx, options)
	}
	return showChoiceDialog(ctx, options)
}

func (c *Controller) startup(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	repositoryPath := c.options.RepositoryPath
	if repositoryPath == "" && c.left == "" && c.right == "" {
		c.prefsMu.Lock()
		repositoryPath = c.prefs.LastRepository()
		c.prefsMu.Unlock()
	}
	shouldLoadFiles := c.left != "" && c.right != ""
	shouldLoadRepository := !shouldLoadFiles && repositoryPath != ""
	c.loading = shouldLoadFiles || shouldLoadRepository
	c.mu.Unlock()

	if ctx.Value("events") != nil {
		runtime.OnFileDrop(ctx, c.handleFileDrop)
	}
	if shouldLoadFiles {
		c.runStartup(func() error {
			c.openInitialFiles()
			return nil
		})
		return
	}
	if shouldLoadRepository {
		c.runStartup(func() error {
			_, err := c.openRepositoryInternal(
				repositoryPath, c.options.RepositoryFile, c.options.RepositoryRef, true, false,
			)
			if err != nil && c.options.RepositoryPath == "" {
				return fmt.Errorf("%s: %w", uiText(c.options.Locale,
					"The most recently opened repository is unavailable",
					"最近打开的仓库不可用",
					"前回開いたリポジトリを利用できません",
				), err)
			}
			return err
		})
	}
}

func (c *Controller) runStartup(task func() error) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		err := task()
		c.mu.Lock()
		c.loading = false
		if err != nil {
			c.loadErr = err.Error()
		}
		c.mu.Unlock()
	}()
}

func (c *Controller) openInitialFiles() {
	c.mu.Lock()
	left, right, options := c.left, c.right, c.options
	c.mu.Unlock()
	session, err := coreapp.Open(left, right, options)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.loadErr = err.Error()
		return
	}
	c.session = session
	c.mode = modeFiles
}

type BootstrapState struct {
	Loading    bool            `json:"loading"`
	HasSession bool            `json:"hasSession"`
	Error      string          `json:"error"`
	Mode       string          `json:"mode"`
	Repository *RepositoryView `json:"repository,omitempty"`
}

type UGitConfigurationResult struct {
	Configured     bool   `json:"configured"`
	Changed        bool   `json:"changed"`
	Cancelled      bool   `json:"cancelled"`
	ExecutablePath string `json:"executablePath"`
	Message        string `json:"message"`
}

func (c *Controller) LanguagePreference() string {
	if c.options.Locale != "" {
		return c.options.Locale
	}
	c.prefsMu.Lock()
	defer c.prefsMu.Unlock()
	return c.prefs.LanguagePreference()
}

func (c *Controller) SetLanguagePreference(preference string) error {
	c.prefsMu.Lock()
	defer c.prefsMu.Unlock()
	return c.prefs.RecordLanguagePreference(preference)
}

func (c *Controller) SetRuntimeLocale(value string) error {
	locale, err := localization.Parse(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.options.Locale = string(locale)
	c.mu.Unlock()
	return nil
}

type ExternalReloadResult struct {
	Summary    coreapp.Summary `json:"summary"`
	Repository *RepositoryView `json:"repository,omitempty"`
	Notice     string          `json:"notice"`
}

func (c *Controller) Bootstrap() BootstrapState {
	c.mu.Lock()
	defer c.mu.Unlock()
	state := BootstrapState{
		Loading: c.loading, HasSession: c.session != nil, Error: c.loadErr, Mode: c.mode,
	}
	if c.repo != nil {
		view := cloneRepositoryView(c.repositoryView)
		state.Repository = &view
	}
	return state
}

func (c *Controller) ConfigureUGit() (UGitConfigurationResult, error) {
	locale := c.options.Locale
	executablePath, err := ugit.CurrentExecutablePath()
	if err != nil {
		return UGitConfigurationResult{}, err
	}
	c.mu.Lock()
	ctx := c.ctx
	c.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	inspection, err := ugit.Inspect(ctx, executablePath)
	if err != nil {
		return UGitConfigurationResult{}, err
	}
	if inspection.Configured {
		return UGitConfigurationResult{
			Configured: true, ExecutablePath: executablePath,
			Message: uiText(locale,
				"UGit's *.xlsx diff and merge tools already point to this app. No update is needed.",
				"UGit 的 *.xlsx 差异与合并工具已指向当前应用，无需更新。",
				"UGit の *.xlsx 差分・マージツールは、すでにこのアプリを参照しています。更新は不要です。",
			) + "\n" + ugitConfigurationContext(inspection, locale),
		}, nil
	}
	if ctx.Value("frontend") == nil {
		return UGitConfigurationResult{}, errors.New(uiText(locale,
			"UGit configuration cannot be confirmed without the desktop interface.",
			"无界面模式下无法确认修改 UGit 配置。",
			"画面を表示しないモードでは、UGit 設定の変更を確認できません。",
		))
	}

	message := fmt.Sprintf(
		uiText(locale,
			"Set this app as UGit's diff and merge tool for *.xlsx files:\n%s\n\n%s\n\nOnly *.xlsx settings will change. Other file types are not affected.",
			"将把当前应用设为 UGit 的 *.xlsx 差异与合并工具：\n%s\n\n%s\n\n只会替换 *.xlsx 相关配置，不影响其他文件类型。",
			"このアプリを UGit の *.xlsx 差分・マージツールに設定します：\n%s\n\n%s\n\n変更するのは *.xlsx の設定だけです。ほかのファイル形式には影響しません。",
		),
		executablePath, ugitConfigurationContext(inspection, locale),
	)
	if inspection.NeedsUpdate {
		if len(inspection.ExistingPaths) > 0 {
			message += uiText(locale, "\n\nOld paths to replace:\n• ", "\n\n需要替换的旧路径：\n• ", "\n\n置き換える古いパス：\n• ") + strings.Join(inspection.ExistingPaths, "\n• ")
		} else {
			message += uiText(locale,
				"\n\nThe existing *.xlsx tool settings are incomplete or incompatible and will be repaired.",
				"\n\n现有 *.xlsx 工具配置不完整或不兼容，将一并修复。",
				"\n\n既存の *.xlsx ツール設定が不完全または互換性のない状態です。あわせて修復します。",
			)
		}
	}
	cancel := uiText(locale, "Cancel", "取消", "キャンセル")
	configure := uiText(locale, "Configure UGit", "配置 UGit", "UGit を設定")
	answer, err := c.ask(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: configure,
		Message: message, Buttons: []string{cancel, configure},
		DefaultButton: configure, CancelButton: cancel,
	})
	if err != nil {
		return UGitConfigurationResult{}, err
	}
	if answer != configure {
		return UGitConfigurationResult{
			Cancelled: true, ExecutablePath: executablePath,
			Message: uiText(locale, "UGit settings were not changed.", "未修改 UGit 配置。", "UGit の設定は変更されていません。"),
		}, nil
	}
	updated, err := ugit.Configure(ctx, executablePath)
	if err != nil {
		return UGitConfigurationResult{}, fmt.Errorf("%s: %w", uiText(locale, "configure UGit", "配置 UGit", "UGit の設定"), err)
	}
	return UGitConfigurationResult{
		Configured: updated.Configured, Changed: true, ExecutablePath: executablePath,
		Message: uiText(locale,
			"UGit's *.xlsx diff and merge tools have been updated. If UGit is running, restart it before testing.",
			"UGit 的 *.xlsx 差异与合并工具已更新。如果 UGit 正在运行，请重启后再测试。",
			"UGit の *.xlsx 差分・マージツールを更新しました。UGit を起動中の場合は、再起動してから確認してください。",
		) + "\n" + ugitConfigurationContext(updated, locale),
	}, nil
}

func ugitConfigurationContext(inspection ugit.Inspection, locale string) string {
	origins := uiText(locale, "No existing *.xlsx settings", "尚无 *.xlsx 配置", "既存の *.xlsx 設定なし")
	if len(inspection.ConfigOrigins) > 0 {
		origins = strings.Join(inspection.ConfigOrigins, uiText(locale, ", ", "、", "、"))
	}
	return fmt.Sprintf(uiText(locale, "Git: %s\n*.xlsx setting source: %s", "Git：%s\n*.xlsx 配置来源：%s", "Git：%s\n*.xlsx 設定の参照元：%s"), inspection.GitPath, origins)
}

func (c *Controller) shutdown(_ context.Context) {
	c.mu.Lock()
	c.shuttingDown = true
	c.differenceIndexGeneration++
	cancelDifferenceIndex := c.differenceIndexCancel
	c.differenceIndexCancel = nil
	c.mu.Unlock()
	if cancelDifferenceIndex != nil {
		cancelDifferenceIndex()
	}
	c.wg.Wait()
	c.mu.Lock()
	session, temp := c.session, c.tempRight
	c.session, c.tempRight = nil, ""
	c.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
	removeTemporary(temp)
}

func (c *Controller) beforeClose(ctx context.Context) bool {
	c.mu.Lock()
	session := c.session
	c.mu.Unlock()
	if session == nil || !session.Dirty() {
		return false
	}
	locale := c.options.Locale
	save := uiText(locale, "Save and continue", "保存并继续", "保存して続行")
	discard := uiText(locale, "Continue without saving", "不保存并继续", "保存せずに続行")
	cancel := uiText(locale, "Cancel", "取消", "キャンセル")
	answer, err := c.ask(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: uiText(locale, "Unsaved changes", "有未保存的修改", "未保存の変更"),
		Message:       uiText(locale, "This workbook has unsaved changes.", "当前工作簿有未保存的修改。", "このブックには未保存の変更があります。"),
		Buttons:       []string{save, discard, cancel},
		DefaultButton: save, CancelButton: cancel,
	})
	if err != nil {
		return true
	}
	switch answer {
	case save:
		_, err := c.Save()
		return err != nil
	case discard:
		return false
	default:
		return true
	}
}

func (c *Controller) SelectAndOpen() (coreapp.Summary, error) {
	c.mu.Lock()
	ctx := c.ctx
	c.mu.Unlock()
	locale := c.options.Locale
	filter := []runtime.FileFilter{{DisplayName: uiText(locale, "Excel workbook (*.xlsx)", "Excel 工作簿 (*.xlsx)", "Excel ブック (*.xlsx)"), Pattern: "*.xlsx"}}
	left, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: uiText(locale, "Select the left (local) workbook", "选择左侧（本地）工作簿", "左側（ローカル）のブックを選択"), Filters: filter})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if left == "" {
		return coreapp.Summary{}, fmt.Errorf("%s", uiText(locale, "Left file selection was cancelled.", "已取消选择左侧文件。", "左側ファイルの選択をキャンセルしました。"))
	}
	right, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: uiText(locale, "Select the right (comparison) workbook", "选择右侧（对比来源）工作簿", "右側（比較元）のブックを選択"), Filters: filter})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if right == "" {
		return coreapp.Summary{}, fmt.Errorf("%s", uiText(locale, "Right file selection was cancelled.", "已取消选择右侧文件。", "右側ファイルの選択をキャンセルしました。"))
	}
	return c.OpenFiles(left, right)
}

func (c *Controller) OpenFiles(left, right string) (coreapp.Summary, error) {
	session, err := coreapp.Open(left, right, c.options)
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := c.confirmSessionSwitch(); err != nil {
		_ = session.Close()
		return coreapp.Summary{}, err
	}
	c.mu.Lock()
	old, oldTemp := c.session, c.tempRight
	c.session = session
	c.left, c.right = left, right
	c.tempRight = ""
	c.repo = nil
	c.repositoryView = RepositoryView{}
	c.mode = modeFiles
	c.loadErr = ""
	c.loading = false
	c.loadGeneration++
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	removeTemporary(oldTemp)
	return session.Summary(), nil
}

func (c *Controller) SelectRepository() (RepositoryResult, error) {
	c.mu.Lock()
	ctx := c.ctx
	locale := c.options.Locale
	c.mu.Unlock()
	path, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: uiText(locale, "Open local Git repository", "打开本地 Git 仓库", "ローカルの Git リポジトリを開く"),
	})
	if err != nil {
		return RepositoryResult{}, err
	}
	if path == "" {
		return RepositoryResult{}, errors.New(uiText(locale, "Repository selection was cancelled.", "已取消选择仓库。", "リポジトリの選択をキャンセルしました。"))
	}
	return c.OpenRepository(path)
}

func (c *Controller) RecentRepositories() []RecentRepository {
	c.prefsMu.Lock()
	paths := c.prefs.RecentRepositories()
	c.prefsMu.Unlock()
	result := make([]RecentRepository, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		result = append(result, RecentRepository{
			Name:      filepath.Base(path),
			Path:      path,
			Available: err == nil && info.IsDir(),
		})
	}
	return result
}

func (c *Controller) OpenRepository(path string) (RepositoryResult, error) {
	return c.openRepositoryInternal(path, "", "", true, true)
}

func (c *Controller) openRepositoryInternal(
	path, selectedFile, selectedRef string, remember, confirm bool,
) (RepositoryResult, error) {
	locale := c.options.Locale
	repo, info, err := repository.Open(path)
	if err != nil {
		return RepositoryResult{}, err
	}
	if selectedFile != "" && !slices.Contains(info.Files, filepath.ToSlash(selectedFile)) {
		return RepositoryResult{}, fmt.Errorf(uiText(locale,
			"The repository does not contain the specified XLSX file: %s",
			"仓库中不存在指定的 XLSX 文件：%s",
			"指定された XLSX ファイルはリポジトリにありません：%s"), selectedFile)
	}
	if selectedRef != "" {
		if _, err := repo.ResolveReference(selectedRef, info.Branches); err != nil {
			return RepositoryResult{}, err
		}
	}
	if confirm {
		if err := c.confirmSessionSwitch(); err != nil {
			return RepositoryResult{}, err
		}
	}
	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	c.prefsMu.Lock()
	repositoryWidth := c.prefs.RepositoryWidth()
	preferredRef := c.prefs.RepositoryRef(info.Root)
	c.prefsMu.Unlock()
	view := viewFromInfo(info, repositoryWidth, locale)
	if selectedRef != "" {
		branch, _ := repo.ResolveReference(selectedRef, info.Branches)
		view.SelectedRef = branch.FullName
	} else if preferredRef != "" {
		if branch, resolveErr := repo.ResolveReference(preferredRef, info.Branches); resolveErr == nil {
			view.SelectedRef = branch.FullName
		}
	}
	if view.SelectedRef == "" {
		view.SelectedRef = view.DefaultRef
	}
	var differenceBranch repository.Branch
	var differenceSignature string
	if view.SelectedRef != "" {
		differenceBranch, _ = repo.ResolveReference(view.SelectedRef, info.Branches)
		differenceFiles, signature, cached, diffErr := c.prepareRepositoryDifferenceIndex(
			repo, differenceBranch, view.Files, true,
		)
		if diffErr != nil {
			view.Notice = fmt.Sprintf(uiText(locale,
				"The repository opened, but the changed-workbook list could not be loaded: %v",
				"仓库已打开，但差异表加载失败：%v",
				"リポジトリは開きましたが、差分のあるブック一覧を読み込めませんでした：%v"), diffErr)
		} else {
			view.DifferenceFiles = differenceFiles
			view.DifferenceIndexing = !cached
			differenceSignature = signature
		}
	}
	c.mu.Lock()
	old, oldTemp := c.session, c.tempRight
	c.session, c.tempRight = nil, ""
	c.repo = repo
	c.repositoryView = view
	c.mode = modeRepository
	c.loadErr = ""
	c.loading = false
	c.loadGeneration++
	c.repositoryView.LoadGeneration = c.loadGeneration
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	removeTemporary(oldTemp)
	if remember {
		c.prefsMu.Lock()
		preferenceErr := c.prefs.RecordRepository(info.Root)
		c.prefsMu.Unlock()
		if preferenceErr != nil {
			c.mu.Lock()
			c.repositoryView.Notice = appendNotice(c.repositoryView.Notice, uiText(locale,
				"The repository opened, but it could not be added to recent repositories.",
				"仓库已打开，但无法记录为最近仓库。",
				"リポジトリは開きましたが、最近使ったリポジトリに記録できませんでした。"))
			c.mu.Unlock()
		}
		c.recordRepositoryRef(info.Root, view.SelectedRef)
	}
	if view.DifferenceIndexing {
		c.startRepositoryDifferenceIndex(repo, differenceBranch, view.Files, differenceSignature)
	}
	if selectedFile != "" {
		return c.selectRepositoryFileLocked(filepath.ToSlash(selectedFile), selectedRef)
	}
	return c.Repository()
}

func viewFromInfo(info repository.Info, width int, locale string) RepositoryView {
	rightState := "no-ref"
	if len(info.Branches) == 0 {
		rightState = "no-branches"
	}
	return RepositoryView{
		Name: info.Name, Path: info.Root, CurrentBranch: info.CurrentBranch,
		Detached: info.Detached, WorkspaceDirty: info.Dirty, Operation: info.Operation,
		Files: append([]string{}, info.Files...), DifferenceFiles: []string{},
		Branches:   append([]repository.Branch{}, info.Branches...),
		DefaultRef: info.DefaultRef, LeftState: "no-file", RightState: rightState,
		LeftMessage: uiText(locale,
			"Select an XLSX file from the repository tree.",
			"请先在左侧目录树中选择一个 XLSX 表格。",
			"リポジトリツリーから XLSX ファイルを選択してください。"),
		RightMessage: uiText(locale,
			"Select an XLSX file from the repository tree.",
			"请先在左侧目录树中选择一个 XLSX 表格。",
			"リポジトリツリーから XLSX ファイルを選択してください。"),
		SidebarWidth: width,
	}
}

func (c *Controller) Repository() (RepositoryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.repo == nil {
		return RepositoryResult{}, errors.New(uiText(c.options.Locale, "No Git repository is open.", "尚未打开 Git 仓库。", "Git リポジトリが開かれていません。"))
	}
	result := RepositoryResult{Repository: cloneRepositoryView(c.repositoryView)}
	if c.session != nil {
		summary := c.session.Summary()
		result.Summary = &summary
	}
	return result, nil
}

func (c *Controller) SelectRepositoryFile(relative string) (RepositoryResult, error) {
	if err := c.confirmRepositoryFileSwitch(relative); err != nil {
		return RepositoryResult{}, err
	}
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	return c.selectRepositoryFileLocked(filepath.ToSlash(relative), "")
}

func (c *Controller) selectRepositoryFileLocked(relative, requestedRef string) (RepositoryResult, error) {
	c.mu.Lock()
	repo := c.repo
	locale := c.options.Locale
	view := cloneRepositoryView(c.repositoryView)
	c.loadGeneration++
	generation := c.loadGeneration
	c.repositoryView.Loading = true
	c.repositoryView.LoadGeneration = generation
	c.mu.Unlock()
	if repo == nil {
		return RepositoryResult{}, errors.New(uiText(locale, "No Git repository is open.", "尚未打开 Git 仓库。", "Git リポジトリが開かれていません。"))
	}
	if !slices.Contains(view.Files, relative) {
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, fmt.Errorf(uiText(locale,
			"The repository does not contain this XLSX file: %s",
			"仓库中不存在该 XLSX 文件：%s",
			"この XLSX ファイルはリポジトリにありません：%s"), relative)
	}
	leftPath, err := repo.ResolveRelativePath(relative)
	if err != nil {
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, err
	}
	if stat, err := os.Stat(leftPath); err != nil || stat.IsDir() {
		return c.installRepositoryFileFailure(
			generation, repo, relative, "missing",
			fmt.Sprintf(uiText(locale,
				"This workbook is missing from the worktree.\nPath: %s",
				"当前工作区中不存在该表格。\n路径：%s",
				"このブックはワークツリーにありません。\nパス：%s"), relative),
		)
	}
	refValue := requestedRef
	if refValue == "" && view.SelectedRef != "" {
		refValue = view.SelectedRef
	}
	if refValue == "" {
		refValue = view.DefaultRef
	}
	options := c.options
	options.Output = ""
	options.ReadonlyLeft = false
	options.LeftLabel = workspaceLabel(view, c.options.Locale)
	options.RightLabel = comparisonRevisionLabel(c.options.Locale)
	session, err := coreapp.OpenLeft(leftPath, options)
	if err != nil {
		return c.installRepositoryFileFailure(
			generation, repo, relative, "error",
			fmt.Sprintf(uiText(locale,
				"This workbook could not be opened.\n%v",
				"无法打开该表格。\n%v",
				"このブックを開けませんでした。\n%v"), err),
		)
	}
	rightState, rightMessage, tempPath, selectedFullRef := c.loadRight(repo, view, session, refValue, relative)
	modified, statusErr := repo.FileModified(relative)
	if statusErr != nil {
		_ = session.Close()
		removeTemporary(tempPath)
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, statusErr
	}
	c.mu.Lock()
	if generation != c.loadGeneration || repo != c.repo {
		c.mu.Unlock()
		_ = session.Close()
		removeTemporary(tempPath)
		return RepositoryResult{}, errors.New(uiText(locale,
			"A newer selection replaced this load request.",
			"加载结果已被新的选择替代。",
			"別の項目が選択されたため、この読み込み結果は破棄されました。"))
	}
	old, oldTemp := c.session, c.tempRight
	c.session, c.tempRight = session, tempPath
	c.repositoryView.SelectedFile = relative
	c.repositoryView.SelectedRef = selectedFullRef
	c.repositoryView.LeftState = "ready"
	c.repositoryView.LeftMessage = ""
	c.repositoryView.RightState = rightState
	c.repositoryView.RightMessage = rightMessage
	c.repositoryView.FileModified = modified
	c.repositoryView.Loading = false
	c.repositoryView.ComparisonActive = rightState == "ready"
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	removeTemporary(oldTemp)
	return c.Repository()
}

func (c *Controller) installRepositoryFileFailure(
	generation uint64,
	repo *repository.Repository,
	relative, state, message string,
) (RepositoryResult, error) {
	c.mu.Lock()
	if generation != c.loadGeneration || repo != c.repo {
		c.mu.Unlock()
		return RepositoryResult{}, errors.New(uiText(c.options.Locale,
			"A newer selection replaced this load request.",
			"加载结果已被新的选择替代。",
			"別の項目が選択されたため、この読み込み結果は破棄されました。"))
	}
	old, oldTemp := c.session, c.tempRight
	c.session, c.tempRight = nil, ""
	c.repositoryView.SelectedFile = relative
	c.repositoryView.LeftState = state
	c.repositoryView.LeftMessage = message
	c.repositoryView.RightState = state
	c.repositoryView.RightMessage = message
	c.repositoryView.Loading = false
	c.repositoryView.ComparisonActive = false
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	removeTemporary(oldTemp)
	return c.Repository()
}

func (c *Controller) loadRight(
	repo *repository.Repository,
	view RepositoryView,
	session *coreapp.Session,
	refValue, relative string,
) (state, message, tempPath, selectedFullRef string) {
	if len(view.Branches) == 0 {
		return "no-branches", uiText(c.options.Locale,
			"This repository has no other revision to compare.",
			"当前仓库中没有可用于对比的其他版本。",
			"このリポジトリには比較できるほかのリビジョンがありません。"), "", ""
	}
	if refValue == "" {
		return "no-ref", uiText(c.options.Locale,
			"Select a Git revision to compare.",
			"请选择一个用于对比的 Git 版本。",
			"比較する Git リビジョンを選択してください。"), "", ""
	}
	branch, err := repo.ResolveReference(refValue, view.Branches)
	if err != nil {
		return "error", err.Error(), "", ""
	}
	selectedFullRef = branch.FullName
	tempPath, err = repo.ReadReferenceFile(branch, relative)
	if repository.IsMissingFile(err) {
		return "missing",
			fmt.Sprintf(uiText(c.options.Locale,
				"This workbook is not present in the selected revision.\nRevision: %s\nPath: %s",
				"所选版本中不存在此表格。\n版本：%s\n路径：%s",
				"選択したリビジョンにこのブックはありません。\nリビジョン：%s\nパス：%s"), branch.Name, relative),
			"", selectedFullRef
	}
	if err != nil {
		return "error", err.Error(), "", selectedFullRef
	}
	if err := session.ReplaceRight(tempPath, readOnlyRevisionLabel(branch.Name, c.options.Locale)); err != nil {
		removeTemporary(tempPath)
		return "invalid",
			fmt.Sprintf(uiText(c.options.Locale,
				"The file in the selected revision is not a readable XLSX workbook.\n%v",
				"所选版本中的文件无法作为 XLSX 工作簿打开。\n%v",
				"選択したリビジョンのファイルを XLSX ブックとして開けません。\n%v"), err),
			"", selectedFullRef
	}
	return "ready", "", tempPath, selectedFullRef
}

func (c *Controller) SelectRepositoryRef(refValue string) (RepositoryResult, error) {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	c.mu.Lock()
	repo, session := c.repo, c.session
	view := cloneRepositoryView(c.repositoryView)
	if repo == nil {
		c.mu.Unlock()
		return RepositoryResult{}, errors.New(uiText(c.options.Locale, "No Git repository is open.", "尚未打开 Git 仓库。", "Git リポジトリが開かれていません。"))
	}
	if session == nil || view.SelectedFile == "" {
		branch, err := repo.ResolveReference(refValue, view.Branches)
		if err != nil {
			c.mu.Unlock()
			return RepositoryResult{}, err
		}
		c.mu.Unlock()
		differenceFiles, signature, cached, diffErr := c.prepareRepositoryDifferenceIndex(
			repo, branch, view.Files, true,
		)
		c.mu.Lock()
		if repo != c.repo {
			c.mu.Unlock()
			return RepositoryResult{}, errors.New(uiText(c.options.Locale,
				"A newer selection replaced this load request.",
				"加载结果已被新的选择替代。",
				"別の項目が選択されたため、この読み込み結果は破棄されました。"))
		}
		c.repositoryView.SelectedRef = branch.FullName
		if diffErr != nil {
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				fmt.Sprintf(uiText(c.options.Locale,
					"The comparison revision changed, but the changed-workbook list could not be loaded: %v",
					"已切换对比版本，但差异表加载失败：%v",
					"比較するリビジョンを変更しましたが、差分のあるブック一覧を読み込めませんでした：%v"), diffErr),
			)
		} else {
			c.repositoryView.DifferenceFiles = differenceFiles
			c.repositoryView.DifferenceIndexing = !cached
		}
		root := c.repositoryView.Path
		c.mu.Unlock()
		c.recordRepositoryRef(root, branch.FullName)
		if diffErr == nil && !cached {
			c.startRepositoryDifferenceIndex(repo, branch, view.Files, signature)
		}
		return c.Repository()
	}
	c.loadGeneration++
	generation := c.loadGeneration
	c.repositoryView.Loading = true
	c.repositoryView.LoadGeneration = generation
	c.mu.Unlock()
	branch, err := repo.ResolveReference(refValue, view.Branches)
	if err != nil {
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, err
	}
	temp, readErr := repo.ReadReferenceFile(branch, view.SelectedFile)
	state, message := "ready", ""
	if repository.IsMissingFile(readErr) {
		state = "missing"
		message = fmt.Sprintf(uiText(c.options.Locale,
			"This workbook is not present in the selected revision.\nRevision: %s\nPath: %s",
			"所选版本中不存在此表格。\n版本：%s\n路径：%s",
			"選択したリビジョンにこのブックはありません。\nリビジョン：%s\nパス：%s"), branch.Name, view.SelectedFile)
		if err := session.DetachRight(readOnlyRevisionLabel(branch.Name, c.options.Locale)); err != nil {
			c.finishRepositoryLoadError(generation)
			return RepositoryResult{}, err
		}
	} else if readErr != nil {
		state, message = "error", readErr.Error()
		if err := session.DetachRight(readOnlyRevisionLabel(branch.Name, c.options.Locale)); err != nil {
			c.finishRepositoryLoadError(generation)
			return RepositoryResult{}, err
		}
	} else if err := session.ReplaceRight(temp, readOnlyRevisionLabel(branch.Name, c.options.Locale)); err != nil {
		removeTemporary(temp)
		temp = ""
		state = "invalid"
		message = fmt.Sprintf(uiText(c.options.Locale,
			"The file in the selected revision is not a readable XLSX workbook.\n%v",
			"所选版本中的文件无法作为 XLSX 工作簿打开。\n%v",
			"選択したリビジョンのファイルを XLSX ブックとして開けません。\n%v"), err)
		_ = session.DetachRight(readOnlyRevisionLabel(branch.Name, c.options.Locale))
	}
	differenceFiles, signature, cached, diffErr := c.prepareRepositoryDifferenceIndex(
		repo, branch, view.Files, true,
	)
	c.mu.Lock()
	if generation != c.loadGeneration || session != c.session {
		c.mu.Unlock()
		removeTemporary(temp)
		return RepositoryResult{}, errors.New(uiText(c.options.Locale,
			"A newer selection replaced this load request.",
			"加载结果已被新的选择替代。",
			"別の項目が選択されたため、この読み込み結果は破棄されました。"))
	}
	oldTemp := c.tempRight
	c.tempRight = temp
	c.repositoryView.SelectedRef = branch.FullName
	c.repositoryView.RightState = state
	c.repositoryView.RightMessage = message
	c.repositoryView.Loading = false
	c.repositoryView.ComparisonActive = state == "ready"
	if diffErr != nil {
		c.repositoryView.Notice = appendNotice(
			c.repositoryView.Notice,
			fmt.Sprintf(uiText(c.options.Locale,
				"The comparison revision changed, but the changed-workbook list could not be loaded: %v",
				"已切换对比版本，但差异表加载失败：%v",
				"比較するリビジョンを変更しましたが、差分のあるブック一覧を読み込めませんでした：%v"), diffErr),
		)
	} else {
		c.repositoryView.DifferenceFiles = differenceFiles
		c.repositoryView.DifferenceIndexing = !cached
	}
	root := c.repositoryView.Path
	c.mu.Unlock()
	removeTemporary(oldTemp)
	c.recordRepositoryRef(root, branch.FullName)
	if diffErr == nil && !cached {
		c.startRepositoryDifferenceIndex(repo, branch, view.Files, signature)
	}
	return c.Repository()
}

func (c *Controller) RefreshRepository() (RepositoryResult, error) {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	c.mu.Lock()
	repo := c.repo
	oldView := cloneRepositoryView(c.repositoryView)
	c.mu.Unlock()
	if repo == nil {
		return RepositoryResult{}, errors.New(uiText(c.options.Locale, "No Git repository is open.", "尚未打开 Git 仓库。", "Git リポジトリが開かれていません。"))
	}
	info, err := repo.Refresh()
	if err != nil {
		return RepositoryResult{}, err
	}
	view := viewFromInfo(info, oldView.SidebarWidth, c.options.Locale)
	view.SelectedFile = oldView.SelectedFile
	view.SelectedRef = oldView.SelectedRef
	view.LeftState = oldView.LeftState
	view.RightState = oldView.RightState
	view.LeftMessage = oldView.LeftMessage
	view.RightMessage = oldView.RightMessage
	view.FileModified = oldView.FileModified
	view.ComparisonActive = oldView.ComparisonActive
	if view.SelectedFile != "" && !slices.Contains(view.Files, view.SelectedFile) {
		view.SelectedFile = ""
		view.LeftState = "no-file"
		view.RightState = "no-file"
		view.LeftMessage = uiText(c.options.Locale,
			"Select an XLSX file from the repository tree.",
			"请先在左侧目录树中选择一个 XLSX 表格。",
			"リポジトリツリーから XLSX ファイルを選択してください。")
		view.RightMessage = view.LeftMessage
		view.ComparisonActive = false
		if view.SelectedRef != "" {
			if _, err := repo.ResolveReference(view.SelectedRef, view.Branches); err != nil {
				view.SelectedRef = view.DefaultRef
			}
		}
		if view.SelectedRef == "" {
			view.SelectedRef = view.DefaultRef
		}
		var differenceBranch repository.Branch
		var differenceSignature string
		startDifferenceIndex := false
		if view.SelectedRef != "" {
			differenceBranch, _ = repo.ResolveReference(view.SelectedRef, view.Branches)
			differenceFiles, signature, cached, diffErr := c.prepareRepositoryDifferenceIndex(
				repo, differenceBranch, view.Files, true,
			)
			if diffErr != nil {
				view.Notice = appendNotice(view.Notice, fmt.Sprintf(uiText(c.options.Locale,
					"The repository refreshed, but the changed-workbook list could not be loaded: %v",
					"仓库已刷新，但差异表加载失败：%v",
					"リポジトリを更新しましたが、差分のあるブック一覧を読み込めませんでした：%v"), diffErr))
			} else {
				view.DifferenceFiles = differenceFiles
				view.DifferenceIndexing = !cached
				differenceSignature = signature
				startDifferenceIndex = !cached
			}
		}
		c.mu.Lock()
		old, oldTemp := c.session, c.tempRight
		c.session, c.tempRight = nil, ""
		c.repositoryView = view
		c.loadGeneration++
		c.repositoryView.LoadGeneration = c.loadGeneration
		root, selectedRef := c.repositoryView.Path, c.repositoryView.SelectedRef
		c.mu.Unlock()
		if old != nil {
			_ = old.Close()
		}
		removeTemporary(oldTemp)
		c.recordRepositoryRef(root, selectedRef)
		if startDifferenceIndex {
			c.startRepositoryDifferenceIndex(repo, differenceBranch, view.Files, differenceSignature)
		}
		return c.Repository()
	}
	if view.SelectedRef != "" {
		if _, err := repo.ResolveReference(view.SelectedRef, view.Branches); err != nil {
			view.SelectedRef = view.DefaultRef
		}
	}
	if view.SelectedRef == "" {
		view.SelectedRef = view.DefaultRef
	}
	var differenceBranch repository.Branch
	var differenceSignature string
	startDifferenceIndex := false
	if view.SelectedRef != "" {
		differenceBranch, _ = repo.ResolveReference(view.SelectedRef, view.Branches)
		differenceFiles, signature, cached, diffErr := c.prepareRepositoryDifferenceIndex(
			repo, differenceBranch, view.Files, true,
		)
		if diffErr != nil {
			view.Notice = appendNotice(view.Notice, fmt.Sprintf(uiText(c.options.Locale,
				"The repository refreshed, but the changed-workbook list could not be loaded: %v",
				"仓库已刷新，但差异表加载失败：%v",
				"リポジトリを更新しましたが、差分のあるブック一覧を読み込めませんでした：%v"), diffErr))
		} else {
			view.DifferenceFiles = differenceFiles
			view.DifferenceIndexing = !cached
			differenceSignature = signature
			startDifferenceIndex = !cached
		}
	}
	c.mu.Lock()
	c.repositoryView = view
	c.loadGeneration++
	c.repositoryView.LoadGeneration = c.loadGeneration
	root, selectedRef := c.repositoryView.Path, c.repositoryView.SelectedRef
	c.mu.Unlock()
	c.recordRepositoryRef(root, selectedRef)
	if startDifferenceIndex {
		c.startRepositoryDifferenceIndex(repo, differenceBranch, view.Files, differenceSignature)
	}
	return c.Repository()
}

func (c *Controller) SetRepositorySidebarWidth(width int) error {
	c.prefsMu.Lock()
	err := c.prefs.RecordRepositoryWidth(width)
	c.prefsMu.Unlock()
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.repo != nil {
		c.repositoryView.SidebarWidth = width
	}
	c.mu.Unlock()
	return nil
}

func (c *Controller) Summary() (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) SetRowAlignment(mode string) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.SetRowAlignment(coreapp.RowAlignmentMode(mode)); err != nil {
		return session.Summary(), err
	}
	return session.Summary(), nil
}

func (c *Controller) SetKeyColumn(sheet string, column int) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.SetKeyColumn(sheet, column); err != nil {
		return session.Summary(), err
	}
	return session.Summary(), nil
}

func (c *Controller) CheckExternalChanges() (coreapp.ExternalChanges, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.ExternalChanges{}, err
	}
	changes, err := session.ExternalChanges()
	if err != nil {
		return coreapp.ExternalChanges{}, err
	}
	c.mu.Lock()
	stillCurrent := session == c.session
	c.mu.Unlock()
	if !stillCurrent {
		return coreapp.ExternalChanges{}, nil
	}
	return changes, nil
}

func (c *Controller) ReloadExternal(side string) (ExternalReloadResult, error) {
	c.loadMu.Lock()
	session, err := c.getSession()
	if err != nil {
		c.loadMu.Unlock()
		return ExternalReloadResult{}, err
	}
	locale := session.Summary().Options.Locale
	switch side {
	case "left":
		err = session.ReloadLeft()
	case "right":
		err = session.ReloadRight()
	default:
		err = fmt.Errorf(uiText(locale,
			"Unknown reload source: %s",
			"未知的重载来源：%s",
			"再読み込み元が不明です：%s"), side)
	}
	if err != nil {
		c.loadMu.Unlock()
		return ExternalReloadResult{}, err
	}
	c.mu.Lock()
	stillCurrent := session == c.session
	inRepository := c.repo != nil
	c.mu.Unlock()
	c.loadMu.Unlock()
	if !stillCurrent {
		return ExternalReloadResult{}, errors.New(uiText(locale,
			"A newer selection replaced this reload request.",
			"重载结果已被新的选择替代。",
			"別の項目が選択されたため、この再読み込み結果は破棄されました。"))
	}

	if side == "left" && inRepository {
		result, refreshErr := c.RefreshRepository()
		if refreshErr != nil {
			return ExternalReloadResult{}, fmt.Errorf("%s: %w", uiText(locale,
				"The workbook was reloaded, but the repository status could not be refreshed",
				"表格已重载，但仓库状态刷新失败",
				"ブックは再読み込みされましたが、リポジトリの状態を更新できませんでした"), refreshErr)
		}
		if result.Summary == nil {
			return ExternalReloadResult{}, errors.New(uiText(locale,
				"The workbook was reloaded, but the current session is no longer available.",
				"表格已重载，但当前会话已不可用。",
				"ブックは再読み込みされましたが、現在のセッションは利用できなくなりました。"))
		}
		view := result.Repository
		return ExternalReloadResult{
			Summary: *result.Summary, Repository: &view,
			Notice: externalReloadNotice(side, result.Summary.Options.ReadonlyLeft, result.Summary.Options.Locale),
		}, nil
	}

	summary := session.Summary()
	result := ExternalReloadResult{
		Summary: summary,
		Notice:  externalReloadNotice(side, summary.Options.ReadonlyLeft, summary.Options.Locale),
	}
	c.mu.Lock()
	if c.repo != nil {
		view := cloneRepositoryView(c.repositoryView)
		result.Repository = &view
	}
	c.mu.Unlock()
	return result, nil
}

func externalReloadNotice(side string, readonlyLeft bool, locale string) string {
	if side == "right" {
		return uiText(locale,
			"The read-only workbook on the right changed outside SheetProof. The latest version has been reloaded.",
			"右侧只读工作簿已被其他程序修改，现已重新载入磁盘上的最新版本。",
			"右側の読み取り専用ブックがほかのアプリで変更されたため、ディスク上の最新版を再読み込みしました。",
		)
	}
	if readonlyLeft {
		return uiText(locale,
			"The read-only workbook on the left changed outside SheetProof. The latest version has been reloaded.",
			"左侧只读工作簿已被其他程序修改，现已重新载入磁盘上的最新版本。",
			"左側の読み取り専用ブックがほかのアプリで変更されたため、ディスク上の最新版を再読み込みしました。",
		)
	}
	return uiText(locale,
		"The workbook on the left has been reloaded from disk.",
		"左侧工作簿已重新载入磁盘上的最新版本。",
		"左側のブックをディスクから再読み込みしました。",
	)
}

func (c *Controller) Region(sheet string, fromRow, rowCount, fromCol, colCount int) (coreapp.Region, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Region{}, err
	}
	return session.Region(sheet, fromRow, rowCount, fromCol, colCount)
}

func (c *Controller) FilteredRegion(
	sheet string,
	statuses []diff.RowStatus,
	fromRow, rowCount, fromCol, colCount int,
) (coreapp.Region, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Region{}, err
	}
	return session.FilteredRegion(sheet, statuses, fromRow, rowCount, fromCol, colCount)
}

func (c *Controller) Differences(sheet string, offset, limit int) (any, error) {
	session, err := c.getSession()
	if err != nil {
		return nil, err
	}
	return session.Differences(sheet, offset, limit)
}

func (c *Controller) CopyRightToLeft(sheet string, row, col int) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.CopyRightToLeft(workbook.CellRef{Sheet: sheet, Row: row, Col: col}); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) CopyRightToLeftMany(sheet string, cells []coreapp.CellCoordinate) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.CopyRightToLeftMany(sheet, cells); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) CopyRowsRightToLeft(sheet string, rows []int) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.CopyRowsRightToLeft(sheet, rows); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) AppendRowsRightToLeft(
	sheet string,
	rows []int,
	ids []string,
) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if _, err := session.AppendRowsRightToLeft(sheet, rows, ids); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) EditLeft(sheet string, row, col int, value, valueType string) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.EditLeft(workbook.CellRef{Sheet: sheet, Row: row, Col: col}, value, valueType); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) ClearLeftSelection(
	sheet string,
	startRow, endRow, startCol, endCol int,
	rows []int,
) (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.ClearLeftSelection(sheet, startRow, endRow, startCol, endCol, rows); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) Undo() (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	if err := session.Undo(); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) Save() (coreapp.Summary, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	c.mu.Lock()
	view, ctx := c.repositoryView, c.ctx
	repo := c.repo
	locale := c.options.Locale
	inRepository := repo != nil
	c.mu.Unlock()
	incrementalRef := ""
	stableInputsBefore := ""
	if inRepository &&
		view.SelectedFile != "" &&
		view.SelectedRef != "" &&
		!view.DifferenceIndexing {
		if branch, resolveErr := repo.ResolveReference(view.SelectedRef, view.Branches); resolveErr == nil {
			if signature, signatureErr := repo.DifferenceIndexSignatureExcluding(
				branch, view.Files, view.SelectedFile,
			); signatureErr == nil {
				incrementalRef = branch.FullName
				stableInputsBefore = signature
			}
		}
	}
	if inRepository && view.Operation != "" && ctx != nil && ctx.Value("frontend") != nil {
		cancel := uiText(locale, "Cancel", "取消", "キャンセル")
		saveAnyway := uiText(locale, "Save anyway", "仍然保存", "このまま保存")
		answer, dialogErr := c.ask(ctx, runtime.MessageDialogOptions{
			Type: runtime.WarningDialog, Title: uiText(locale, "Git operation in progress", "Git 操作正在进行", "Git 操作の実行中"),
			Message: fmt.Sprintf(uiText(locale,
				"The repository is currently running %s. Saving changes only the current XLSX file; it does not complete or recover the Git operation.",
				"仓库正在进行 %s。保存只会修改当前 XLSX 文件，不会完成或恢复该 Git 操作。",
				"リポジトリで %s が進行中です。保存しても現在の XLSX ファイルが更新されるだけで、Git 操作の完了や復旧は行いません。",
			), view.Operation),
			Buttons: []string{cancel, saveAnyway}, DefaultButton: cancel, CancelButton: cancel,
		})
		if dialogErr != nil {
			return coreapp.Summary{}, dialogErr
		}
		if answer != saveAnyway {
			return session.Summary(), errors.New(uiText(locale, "Save was cancelled.", "已取消保存。", "保存をキャンセルしました。"))
		}
	}
	if err := session.Save(""); err != nil {
		return coreapp.Summary{}, err
	}
	savedSummary := session.Summary()
	if inRepository && view.SelectedFile != "" {
		info, refreshErr := repo.Refresh()
		modified, statusErr := repo.FileModified(view.SelectedFile)
		if refreshErr == nil && statusErr == nil {
			selectedRef := view.SelectedRef
			if selectedRef != "" {
				if _, resolveErr := repo.ResolveReference(selectedRef, info.Branches); resolveErr != nil {
					selectedRef = info.DefaultRef
				}
			}
			if selectedRef == "" {
				selectedRef = info.DefaultRef
			}
			differenceFiles := []string{}
			var differenceBranch repository.Branch
			var differenceSignature string
			startDifferenceIndex := false
			var diffErr error
			cacheWarning := ""
			if selectedRef != "" {
				differenceBranch, _ = repo.ResolveReference(selectedRef, info.Branches)
				var cached bool
				differenceFiles, differenceSignature, cached, diffErr =
					c.prepareRepositoryDifferenceIndex(repo, differenceBranch, info.Files, true)
				if diffErr == nil &&
					!cached &&
					incrementalRef == differenceBranch.FullName &&
					stableInputsBefore != "" &&
					!view.DifferenceIndexing {
					stableInputsAfter, stableErr := repo.DifferenceIndexSignatureExcluding(
						differenceBranch, info.Files, view.SelectedFile,
					)
					if stableErr == nil && stableInputsAfter == stableInputsBefore {
						selectedDiffers := view.RightState == "ready" && !savedSummary.Diff.Equal
						differenceFiles = updateDifferenceFileMembership(
							info.Files,
							view.DifferenceFiles,
							view.SelectedFile,
							selectedDiffers,
						)
						c.prefsMu.Lock()
						cacheErr := c.prefs.RecordRepositoryIndex(
							repo.Root(),
							differenceBranch.FullName,
							differenceSignature,
							differenceFiles,
						)
						c.prefsMu.Unlock()
						if cacheErr != nil {
							cacheWarning = fmt.Sprintf(uiText(locale,
								"The changed-workbook result was updated, but it could not be cached: %v",
								"当前表格的差异表结果已更新，但无法缓存：%v",
								"差分のあるブック一覧は更新されましたが、キャッシュに保存できませんでした：%v"), cacheErr)
						}
						cached = true
					}
				}
				startDifferenceIndex = diffErr == nil && !cached
			}
			c.mu.Lock()
			if repo == c.repo {
				c.repositoryView.FileModified = modified
				c.repositoryView.WorkspaceDirty = info.Dirty
				c.repositoryView.Operation = info.Operation
				c.repositoryView.CurrentBranch = info.CurrentBranch
				c.repositoryView.Detached = info.Detached
				c.repositoryView.Files = append([]string{}, info.Files...)
				c.repositoryView.Branches = append([]repository.Branch{}, info.Branches...)
				c.repositoryView.DefaultRef = info.DefaultRef
				c.repositoryView.SelectedRef = selectedRef
				if diffErr != nil {
					c.repositoryView.Notice = appendNotice(
						c.repositoryView.Notice,
						fmt.Sprintf(uiText(locale,
							"The workbook was saved, but the changed-workbook list could not be refreshed: %v",
							"保存成功，但差异表刷新失败：%v",
							"ブックは保存されましたが、差分のあるブック一覧を更新できませんでした：%v"), diffErr),
					)
				} else {
					c.repositoryView.DifferenceFiles = differenceFiles
					c.repositoryView.DifferenceIndexing = startDifferenceIndex
					if cacheWarning != "" {
						c.repositoryView.Notice = appendNotice(
							c.repositoryView.Notice,
							cacheWarning,
						)
					}
				}
			}
			c.mu.Unlock()
			c.recordRepositoryRef(info.Root, selectedRef)
			if startDifferenceIndex {
				c.startRepositoryDifferenceIndex(
					repo, differenceBranch, info.Files, differenceSignature,
				)
			}
		}
	}
	return savedSummary, nil
}

func (c *Controller) SaveAs() (coreapp.Summary, error) {
	c.mu.Lock()
	inRepository := c.repo != nil
	c.mu.Unlock()
	session, err := c.getSession()
	if err != nil {
		return coreapp.Summary{}, err
	}
	summary := session.Summary()
	defaultFilename := filepath.Base(summary.Diff.LeftFile)
	if defaultFilename == "." || defaultFilename == string(filepath.Separator) || defaultFilename == "" {
		defaultFilename = "workbook.xlsx"
	}
	c.mu.Lock()
	ctx := c.ctx
	c.mu.Unlock()
	c.prefsMu.Lock()
	defaultDirectory := c.prefs.SaveDirectory()
	c.prefsMu.Unlock()
	target, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:                uiText(summary.Options.Locale, "Save workbook as", "工作簿另存为", "ブックに名前を付けて保存"),
		DefaultDirectory:     defaultDirectory,
		DefaultFilename:      defaultFilename,
		Filters:              []runtime.FileFilter{{DisplayName: uiText(summary.Options.Locale, "Excel workbook (*.xlsx)", "Excel 工作簿 (*.xlsx)", "Excel ブック (*.xlsx)"), Pattern: "*.xlsx"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if target == "" {
		return session.Summary(), nil
	}
	if inRepository {
		if err := session.Export(target); err != nil {
			return coreapp.Summary{}, err
		}
		c.mu.Lock()
		if c.repo != nil {
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				uiText(summary.Options.Locale, "Copy exported: ", "已导出副本：", "コピーを書き出しました：")+target,
			)
		}
		c.mu.Unlock()
	} else {
		if err := session.Save(target); err != nil {
			return coreapp.Summary{}, err
		}
	}
	c.prefsMu.Lock()
	preferenceErr := c.prefs.RecordSaveTarget(target)
	c.prefsMu.Unlock()
	if preferenceErr != nil {
		runtime.LogWarningf(ctx, "record last Save As directory: %v", preferenceErr)
	}
	return session.Summary(), nil
}

func (c *Controller) getSession() (*coreapp.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, fmt.Errorf("%s", uiText(c.options.Locale,
			"Open two workbooks or select an XLSX file from a repository first.",
			"请先打开左右工作簿，或从仓库中选择 XLSX 文件。",
			"先に 2 つのブックを開くか、リポジトリから XLSX ファイルを選択してください。",
		))
	}
	return c.session, nil
}

func (c *Controller) confirmRepositoryFileSwitch(relative string) error {
	c.mu.Lock()
	current := c.repositoryView.SelectedFile
	c.mu.Unlock()
	if current == "" || current == filepath.ToSlash(relative) {
		return nil
	}
	return c.confirmSessionSwitch()
}

func (c *Controller) confirmSessionSwitch() error {
	c.mu.Lock()
	session, ctx, locale := c.session, c.ctx, c.options.Locale
	c.mu.Unlock()
	if session == nil || !session.Dirty() {
		return nil
	}
	var answer string
	var err error
	save := uiText(locale, "Save and continue", "保存并继续", "保存して続行")
	discard := uiText(locale, "Continue without saving", "不保存并继续", "保存せずに続行")
	cancel := uiText(locale, "Cancel", "取消", "キャンセル")
	if c.switchPrompt != nil {
		answer, err = c.switchPrompt()
	} else {
		if ctx == nil || ctx.Value("frontend") == nil {
			return errors.New(uiText(locale,
				"This workbook has unsaved changes. Switching cannot be confirmed without the desktop interface.",
				"当前工作簿有未保存的修改；无界面模式下无法确认是否切换。",
				"このブックには未保存の変更があります。画面を表示しないモードでは切り替えを確認できません。",
			))
		}
		answer, err = c.ask(ctx, runtime.MessageDialogOptions{
			Type: runtime.QuestionDialog, Title: uiText(locale, "Unsaved changes", "有未保存的修改", "未保存の変更"),
			Message:       uiText(locale, "This workbook has unsaved changes.", "当前工作簿有未保存的修改。", "このブックには未保存の変更があります。"),
			Buttons:       []string{save, discard, cancel},
			DefaultButton: save, CancelButton: cancel,
		})
	}
	if err != nil {
		return err
	}
	switch answer {
	case save, "保存并继续":
		_, err := c.Save()
		return err
	case discard, "不保存并继续":
		return nil
	default:
		return errors.New(uiText(locale, "Switch was cancelled.", "已取消切换。", "切り替えをキャンセルしました。"))
	}
}

func (c *Controller) finishRepositoryLoadError(generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if generation == c.loadGeneration {
		c.repositoryView.Loading = false
	}
}

func (c *Controller) handleFileDrop(_ int, _ int, paths []string) {
	if len(paths) == 0 {
		return
	}
	first := paths[0]
	multiple := len(paths) > 1
	c.mu.Lock()
	ctx := c.ctx
	locale := c.options.Locale
	c.mu.Unlock()
	if ctx != nil && ctx.Value("events") != nil {
		runtime.EventsEmit(ctx, "repository-drop-started")
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		result, err := c.OpenRepository(first)
		if multiple && err == nil {
			c.mu.Lock()
			c.repositoryView.Notice = uiText(locale,
				"The first folder was opened; the other dropped items were ignored.",
				"已打开第一个目录，其余拖入项已忽略。",
				"最初のフォルダーを開きました。ほかのドロップ項目は無視されました。")
			result.Repository.Notice = c.repositoryView.Notice
			c.mu.Unlock()
		}
		c.mu.Lock()
		ctx := c.ctx
		if err != nil {
			c.loadErr = err.Error()
		}
		c.mu.Unlock()
		if ctx != nil && ctx.Value("events") != nil {
			if err != nil {
				runtime.EventsEmit(ctx, "repository-drop-result", nil, err.Error())
			} else {
				runtime.EventsEmit(ctx, "repository-drop-result", result, "")
			}
		}
	}()
}

func workspaceLabel(view RepositoryView, locale string) string {
	prefix := "Current worktree"
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		prefix = "当前工作区"
	case localization.Japanese:
		prefix = "現在のワークツリー"
	}
	if view.Detached {
		return prefix + " · Detached HEAD"
	}
	return prefix + " · " + view.CurrentBranch
}

func comparisonRevisionLabel(locale string) string {
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		return "对比版本（只读）"
	case localization.Japanese:
		return "比較対象のリビジョン（読み取り専用）"
	default:
		return "Comparison revision (read-only)"
	}
}

func readOnlyRevisionLabel(name, locale string) string {
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		return name + "（只读）"
	case localization.Japanese:
		return name + "（読み取り専用）"
	default:
		return name + " (read-only)"
	}
}

func cloneRepositoryView(source RepositoryView) RepositoryView {
	copy := source
	copy.Files = append([]string{}, source.Files...)
	copy.DifferenceFiles = append([]string{}, source.DifferenceFiles...)
	copy.Branches = append([]repository.Branch{}, source.Branches...)
	return copy
}

func (c *Controller) prepareRepositoryDifferenceIndex(
	repo *repository.Repository,
	branch repository.Branch,
	files []string,
	useCache bool,
) ([]string, string, bool, error) {
	signature, err := repo.DifferenceIndexSignature(branch, files)
	if err != nil {
		return nil, "", false, err
	}
	if useCache {
		c.prefsMu.Lock()
		cached, exists := c.prefs.RepositoryIndex(repo.Root(), branch.FullName, signature)
		c.prefsMu.Unlock()
		if exists {
			return cached, signature, true, nil
		}
	}
	return nil, signature, false, nil
}

func (c *Controller) startRepositoryDifferenceIndex(
	repo *repository.Repository,
	branch repository.Branch,
	files []string,
	signature string,
) {
	c.mu.Lock()
	if c.repo != repo || c.repositoryView.SelectedRef != branch.FullName {
		c.mu.Unlock()
		return
	}
	if c.shuttingDown {
		c.mu.Unlock()
		return
	}
	if c.differenceIndexCancel != nil {
		c.differenceIndexCancel()
	}
	parent := c.ctx
	if parent == nil {
		parent = context.Background()
	}
	indexContext, cancel := context.WithCancel(parent)
	c.differenceIndexCancel = cancel
	c.differenceIndexGeneration++
	generation := c.differenceIndexGeneration
	locale := c.options.Locale
	c.repositoryView.DifferenceFiles = []string{}
	c.repositoryView.DifferenceIndexing = true
	c.wg.Add(1)
	c.mu.Unlock()
	go func() {
		defer func() {
			cancel()
			c.wg.Done()
		}()
		result, skipped, err := exactRepositoryDifferenceFiles(indexContext, repo, branch, files, locale)
		cacheWarning := ""
		if err == nil {
			currentSignature, signatureErr := repo.DifferenceIndexSignatureContext(indexContext, branch, files)
			if signatureErr != nil {
				err = signatureErr
			} else if currentSignature != signature {
				err = errors.New(uiText(locale,
					"The repository changed while the changed-workbook list was being built. Refresh and try again.",
					"建立差异表索引期间仓库内容发生变化，请刷新后重试。",
					"差分のあるブック一覧の作成中にリポジトリが変更されました。更新してからやり直してください。"))
			}
		}
		if err == nil && indexContext.Err() == nil {
			c.prefsMu.Lock()
			cacheErr := c.prefs.RecordRepositoryIndex(
				repo.Root(), branch.FullName, signature, result,
			)
			c.prefsMu.Unlock()
			if cacheErr != nil {
				cacheWarning = fmt.Sprintf(uiText(locale,
					"The changed-workbook list was built, but it could not be cached: %v",
					"差异表索引已建立，但无法缓存：%v",
					"差分のあるブック一覧は作成されましたが、キャッシュに保存できませんでした：%v"), cacheErr)
			}
		}

		c.mu.Lock()
		defer c.mu.Unlock()
		if generation != c.differenceIndexGeneration ||
			c.repo != repo ||
			c.repositoryView.SelectedRef != branch.FullName {
			return
		}
		c.differenceIndexCancel = nil
		c.repositoryView.DifferenceIndexing = false
		if err != nil {
			c.repositoryView.DifferenceFiles = []string{}
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				fmt.Sprintf(uiText(locale,
					"The changed-workbook list could not be built: %v",
					"差异表索引建立失败：%v",
					"差分のあるブック一覧を作成できませんでした：%v"), err),
			)
			return
		}
		c.repositoryView.DifferenceFiles = result
		if len(skipped) > 0 {
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				formatSkippedDifferenceIndexFiles(skipped, locale),
			)
		}
		if cacheWarning != "" {
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				cacheWarning,
			)
		}
	}()
}

func exactRepositoryDifferenceFiles(
	ctx context.Context,
	repo *repository.Repository,
	branch repository.Branch,
	files []string,
	locale string,
) ([]string, []string, error) {
	candidates, err := repo.ChangedCommonXLSXContext(ctx, branch, files)
	if err != nil {
		return nil, nil, err
	}
	result := make([]string, 0, len(candidates))
	skipped := make([]string, 0)
	for _, relative := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		left, resolveErr := repo.ResolveRelativePath(relative)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		right, readErr := repo.ReadReferenceFileContext(ctx, branch, relative)
		if readErr != nil {
			return nil, nil, readErr
		}
		session, openErr := coreapp.OpenContext(ctx, left, right, coreapp.Options{ReadonlyLeft: true})
		if openErr != nil {
			removeTemporary(right)
			if skippableDifferenceIndexWorkbookError(openErr) {
				skipped = append(skipped, relative)
				continue
			}
			return nil, nil, fmt.Errorf("%s: %w", fmt.Sprintf(uiText(locale,
				"compare %s",
				"比较 %s",
				"%s を比較"), relative), openErr)
		}
		summary := session.Summary()
		closeErr := session.Close()
		removeTemporary(right)
		if closeErr != nil {
			return nil, nil, fmt.Errorf("%s: %w", fmt.Sprintf(uiText(locale,
				"close %s",
				"关闭 %s",
				"%s を閉じる"), relative), closeErr)
		}
		if !summary.Diff.Equal {
			result = append(result, relative)
		}
	}
	return result, skipped, nil
}

func skippableDifferenceIndexWorkbookError(err error) bool {
	return workbook.HasCode(err, workbook.ErrCorrupt) ||
		workbook.HasCode(err, workbook.ErrUnsupported) ||
		workbook.HasCode(err, workbook.ErrNoSheets)
}

func formatSkippedDifferenceIndexFiles(files []string, locale string) string {
	const visibleLimit = 3
	visible := files
	if len(visible) > visibleLimit {
		visible = visible[:visibleLimit]
	}
	separator := uiText(locale, ", ", "、", "、")
	message := fmt.Sprintf(uiText(locale,
		"The changed-workbook list skipped unreadable XLSX files: %s",
		"差异表索引已跳过无法解析的 XLSX：%s",
		"差分のあるブック一覧では、読み込めない XLSX ファイルを除外しました：%s"),
		strings.Join(visible, separator))
	if remaining := len(files) - len(visible); remaining > 0 {
		message += fmt.Sprintf(uiText(locale,
			" and %d more",
			" 等 %d 个文件",
			" ほか %d 件"), remaining)
	}
	return message + uiText(locale,
		". Fix the files, then refresh the repository to check again.",
		"；修复文件后刷新仓库即可重新检查。",
		"。ファイルを修正し、リポジトリを更新すると再確認できます。")
}

func (c *Controller) recordRepositoryRef(root, ref string) {
	if root == "" || ref == "" {
		return
	}
	c.prefsMu.Lock()
	err := c.prefs.RecordRepositoryRef(root, ref)
	c.prefsMu.Unlock()
	if err != nil {
		c.mu.Lock()
		if c.repo != nil && c.repositoryView.Path == root {
			c.repositoryView.Notice = appendNotice(
				c.repositoryView.Notice,
				uiText(c.options.Locale,
					"The comparison revision was selected, but the preference could not be saved.",
					"已选择对比版本，但无法记录该版本偏好。",
					"比較するリビジョンは選択されましたが、設定を保存できませんでした。"),
			)
		}
		c.mu.Unlock()
	}
}

func appendNotice(current, addition string) string {
	if current == "" {
		return addition
	}
	return current + "\n" + addition
}

func uiText(locale, english, chinese, japanese string) string {
	switch localization.Normalize(locale) {
	case localization.SimplifiedChinese:
		return chinese
	case localization.Japanese:
		return japanese
	default:
		return english
	}
}

func updateDifferenceFileMembership(
	repositoryFiles []string,
	current []string,
	selected string,
	differs bool,
) []string {
	included := make(map[string]struct{}, len(current)+1)
	for _, relative := range current {
		if relative != selected {
			included[relative] = struct{}{}
		}
	}
	if differs {
		included[selected] = struct{}{}
	}
	result := make([]string, 0, len(included))
	for _, relative := range repositoryFiles {
		if _, exists := included[relative]; exists {
			result = append(result, relative)
		}
	}
	return result
}

func removeTemporary(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
