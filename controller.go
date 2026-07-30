package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	coreapp "github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/preferences"
	"github.com/ug-tools/ugxlsx/internal/repository"
	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	modeFiles      = "files"
	modeRepository = "repository"
)

type RepositoryView struct {
	Name             string              `json:"name"`
	Path             string              `json:"path"`
	CurrentBranch    string              `json:"currentBranch"`
	Detached         bool                `json:"detached"`
	WorkspaceDirty   bool                `json:"workspaceDirty"`
	Operation        string              `json:"operation"`
	Files            []string            `json:"files"`
	Branches         []repository.Branch `json:"branches"`
	DefaultRef       string              `json:"defaultRef"`
	SelectedFile     string              `json:"selectedFile"`
	SelectedRef      string              `json:"selectedRef"`
	LeftState        string              `json:"leftState"`
	RightState       string              `json:"rightState"`
	LeftMessage      string              `json:"leftMessage"`
	RightMessage     string              `json:"rightMessage"`
	FileModified     bool                `json:"fileModified"`
	SidebarWidth     int                 `json:"sidebarWidth"`
	Notice           string              `json:"notice"`
	Loading          bool                `json:"loading"`
	LoadGeneration   uint64              `json:"loadGeneration"`
	ComparisonActive bool                `json:"comparisonActive"`
}

type RepositoryResult struct {
	Repository RepositoryView   `json:"repository"`
	Summary    *coreapp.Summary `json:"summary"`
}

type Controller struct {
	mu             sync.Mutex
	loadMu         sync.Mutex
	wg             sync.WaitGroup
	ctx            context.Context
	session        *coreapp.Session
	left           string
	right          string
	options        coreapp.Options
	loading        bool
	loadErr        string
	mode           string
	prefs          preferences.Store
	repo           *repository.Repository
	repositoryView RepositoryView
	tempRight      string
	loadGeneration uint64
	switchPrompt   func() (string, error)
}

func NewController(left, right string, options coreapp.Options) *Controller {
	return &Controller{
		left: left, right: right, options: options,
		prefs: preferences.NewStore(),
	}
}

func (c *Controller) startup(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	repositoryPath := c.options.RepositoryPath
	if repositoryPath == "" && c.left == "" && c.right == "" {
		repositoryPath = c.prefs.LastRepository()
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
				return fmt.Errorf("最近打开的仓库不可用: %w", err)
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

func (c *Controller) shutdown(_ context.Context) {
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
	answer, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "存在未保存修改",
		Message: "关闭窗口将丢失尚未保存的修改。确定关闭吗？",
		Buttons: []string{"取消", "丢弃并关闭"}, DefaultButton: "取消", CancelButton: "取消",
	})
	return err != nil || answer != "丢弃并关闭"
}

func (c *Controller) SelectAndOpen() (coreapp.Summary, error) {
	c.mu.Lock()
	ctx := c.ctx
	c.mu.Unlock()
	filter := []runtime.FileFilter{{DisplayName: "Excel 工作簿 (*.xlsx)", Pattern: "*.xlsx"}}
	left, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: "选择左侧（本地）工作簿", Filters: filter})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if left == "" {
		return coreapp.Summary{}, fmt.Errorf("已取消选择左侧文件")
	}
	right, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: "选择右侧（对比来源）工作簿", Filters: filter})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if right == "" {
		return coreapp.Summary{}, fmt.Errorf("已取消选择右侧文件")
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
	c.mu.Unlock()
	path, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "打开本地 Git 仓库",
	})
	if err != nil {
		return RepositoryResult{}, err
	}
	if path == "" {
		return RepositoryResult{}, errors.New("已取消选择仓库")
	}
	return c.OpenRepository(path)
}

func (c *Controller) OpenRepository(path string) (RepositoryResult, error) {
	return c.openRepositoryInternal(path, "", "", true, true)
}

func (c *Controller) openRepositoryInternal(
	path, selectedFile, selectedRef string, remember, confirm bool,
) (RepositoryResult, error) {
	repo, info, err := repository.Open(path)
	if err != nil {
		return RepositoryResult{}, err
	}
	if selectedFile != "" && !slices.Contains(info.Files, filepath.ToSlash(selectedFile)) {
		return RepositoryResult{}, fmt.Errorf("仓库中不存在指定的 XLSX 文件: %s", selectedFile)
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

	view := viewFromInfo(info, c.prefs.RepositoryWidth())
	if selectedRef != "" {
		branch, _ := repo.ResolveReference(selectedRef, info.Branches)
		view.SelectedRef = branch.FullName
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
		if err := c.prefs.RecordRepository(info.Root); err != nil {
			c.mu.Lock()
			c.repositoryView.Notice = "仓库已打开，但无法记录为最近仓库"
			c.mu.Unlock()
		}
	}
	if selectedFile != "" {
		return c.selectRepositoryFileLocked(filepath.ToSlash(selectedFile), selectedRef)
	}
	return c.Repository()
}

func viewFromInfo(info repository.Info, width int) RepositoryView {
	rightState := "no-ref"
	if len(info.Branches) == 0 {
		rightState = "no-branches"
	}
	return RepositoryView{
		Name: info.Name, Path: info.Root, CurrentBranch: info.CurrentBranch,
		Detached: info.Detached, WorkspaceDirty: info.Dirty, Operation: info.Operation,
		Files: append([]string{}, info.Files...), Branches: append([]repository.Branch{}, info.Branches...),
		DefaultRef: info.DefaultRef, LeftState: "no-file", RightState: rightState,
		LeftMessage:  "请先在左侧目录树中选择一个 XLSX 表格",
		RightMessage: "请先在左侧目录树中选择一个 XLSX 表格",
		SidebarWidth: width,
	}
}

func (c *Controller) Repository() (RepositoryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.repo == nil {
		return RepositoryResult{}, errors.New("尚未打开 Git 仓库")
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
	view := cloneRepositoryView(c.repositoryView)
	c.loadGeneration++
	generation := c.loadGeneration
	c.repositoryView.Loading = true
	c.repositoryView.LoadGeneration = generation
	c.mu.Unlock()
	if repo == nil {
		return RepositoryResult{}, errors.New("尚未打开 Git 仓库")
	}
	if !slices.Contains(view.Files, relative) {
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, fmt.Errorf("仓库中不存在该 XLSX 文件: %s", relative)
	}
	leftPath, err := repo.ResolveRelativePath(relative)
	if err != nil {
		c.finishRepositoryLoadError(generation)
		return RepositoryResult{}, err
	}
	if stat, err := os.Stat(leftPath); err != nil || stat.IsDir() {
		return c.installRepositoryFileFailure(
			generation, repo, relative, "missing",
			fmt.Sprintf("当前工作区中不存在该表格\n路径：%s", relative),
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
	options.LeftLabel = workspaceLabel(view)
	options.RightLabel = "对比分支（只读）"
	session, err := coreapp.OpenLeft(leftPath, options)
	if err != nil {
		return c.installRepositoryFileFailure(
			generation, repo, relative, "error",
			fmt.Sprintf("无法打开该表格\n%v", err),
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
		return RepositoryResult{}, errors.New("加载结果已被新的选择替代")
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
		return RepositoryResult{}, errors.New("加载结果已被新的选择替代")
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
		return "no-branches", "当前仓库中没有可用于对比的其他分支", "", ""
	}
	if refValue == "" {
		return "no-ref", "请选择一个用于对比的分支", "", ""
	}
	branch, err := repo.ResolveReference(refValue, view.Branches)
	if err != nil {
		return "error", err.Error(), "", ""
	}
	selectedFullRef = branch.FullName
	tempPath, err = repo.ReadReferenceFile(branch, relative)
	if repository.IsMissingFile(err) {
		return "missing",
			fmt.Sprintf("该分支中不存在此表格\n分支：%s\n路径：%s", branch.Name, relative),
			"", selectedFullRef
	}
	if err != nil {
		return "error", err.Error(), "", selectedFullRef
	}
	if err := session.ReplaceRight(tempPath, branch.Name+"（只读）"); err != nil {
		removeTemporary(tempPath)
		return "invalid",
			fmt.Sprintf("该分支中的文件无法作为 XLSX 工作簿打开\n%v", err),
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
		return RepositoryResult{}, errors.New("尚未打开 Git 仓库")
	}
	if session == nil || view.SelectedFile == "" {
		branch, err := repo.ResolveReference(refValue, view.Branches)
		if err != nil {
			c.mu.Unlock()
			return RepositoryResult{}, err
		}
		c.repositoryView.SelectedRef = branch.FullName
		c.mu.Unlock()
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
		message = fmt.Sprintf("该分支中不存在此表格\n分支：%s\n路径：%s", branch.Name, view.SelectedFile)
		if err := session.DetachRight(branch.Name + "（只读）"); err != nil {
			c.finishRepositoryLoadError(generation)
			return RepositoryResult{}, err
		}
	} else if readErr != nil {
		state, message = "error", readErr.Error()
		if err := session.DetachRight(branch.Name + "（只读）"); err != nil {
			c.finishRepositoryLoadError(generation)
			return RepositoryResult{}, err
		}
	} else if err := session.ReplaceRight(temp, branch.Name+"（只读）"); err != nil {
		removeTemporary(temp)
		temp = ""
		state = "invalid"
		message = fmt.Sprintf("该分支中的文件无法作为 XLSX 工作簿打开\n%v", err)
		_ = session.DetachRight(branch.Name + "（只读）")
	}
	c.mu.Lock()
	if generation != c.loadGeneration || session != c.session {
		c.mu.Unlock()
		removeTemporary(temp)
		return RepositoryResult{}, errors.New("加载结果已被新的选择替代")
	}
	oldTemp := c.tempRight
	c.tempRight = temp
	c.repositoryView.SelectedRef = branch.FullName
	c.repositoryView.RightState = state
	c.repositoryView.RightMessage = message
	c.repositoryView.Loading = false
	c.repositoryView.ComparisonActive = state == "ready"
	c.mu.Unlock()
	removeTemporary(oldTemp)
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
		return RepositoryResult{}, errors.New("尚未打开 Git 仓库")
	}
	info, err := repo.Refresh()
	if err != nil {
		return RepositoryResult{}, err
	}
	view := viewFromInfo(info, oldView.SidebarWidth)
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
		view.SelectedRef = ""
		view.LeftState = "no-file"
		view.RightState = "no-file"
		view.LeftMessage = "请先在左侧目录树中选择一个 XLSX 表格"
		view.RightMessage = view.LeftMessage
		view.ComparisonActive = false
		c.mu.Lock()
		old, oldTemp := c.session, c.tempRight
		c.session, c.tempRight = nil, ""
		c.repositoryView = view
		c.loadGeneration++
		c.mu.Unlock()
		if old != nil {
			_ = old.Close()
		}
		removeTemporary(oldTemp)
		return c.Repository()
	}
	if view.SelectedRef != "" {
		if _, err := repo.ResolveReference(view.SelectedRef, view.Branches); err != nil {
			view.SelectedRef = view.DefaultRef
		}
	}
	c.mu.Lock()
	c.repositoryView = view
	c.loadGeneration++
	c.repositoryView.LoadGeneration = c.loadGeneration
	c.mu.Unlock()
	return c.Repository()
}

func (c *Controller) SetRepositorySidebarWidth(width int) error {
	if err := c.prefs.RecordRepositoryWidth(width); err != nil {
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

func (c *Controller) Region(sheet string, fromRow, rowCount, fromCol, colCount int) (coreapp.Region, error) {
	session, err := c.getSession()
	if err != nil {
		return coreapp.Region{}, err
	}
	return session.Region(sheet, fromRow, rowCount, fromCol, colCount)
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
	inRepository := repo != nil
	c.mu.Unlock()
	if inRepository && view.Operation != "" && ctx != nil && ctx.Value("frontend") != nil {
		answer, dialogErr := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type: runtime.WarningDialog, Title: "Git 操作进行中",
			Message: fmt.Sprintf("仓库正在进行 %s。保存只会修改当前 XLSX 文件，不会恢复或完成 Git 操作。", view.Operation),
			Buttons: []string{"取消", "仍然保存"}, DefaultButton: "取消", CancelButton: "取消",
		})
		if dialogErr != nil {
			return coreapp.Summary{}, dialogErr
		}
		if answer != "仍然保存" {
			return session.Summary(), errors.New("已取消保存")
		}
	}
	if err := session.Save(""); err != nil {
		return coreapp.Summary{}, err
	}
	if inRepository && view.SelectedFile != "" {
		info, refreshErr := repo.Refresh()
		modified, statusErr := repo.FileModified(view.SelectedFile)
		if refreshErr == nil && statusErr == nil {
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
			}
			c.mu.Unlock()
		}
	}
	return session.Summary(), nil
}

func (c *Controller) SaveAs() (coreapp.Summary, error) {
	c.mu.Lock()
	inRepository := c.repo != nil
	c.mu.Unlock()
	if inRepository {
		return coreapp.Summary{}, errors.New("仓库模式下请使用“保存到当前工作区”")
	}
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
	target, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:                "另存为 Excel 工作簿",
		DefaultDirectory:     c.prefs.SaveDirectory(),
		DefaultFilename:      defaultFilename,
		Filters:              []runtime.FileFilter{{DisplayName: "Excel 工作簿 (*.xlsx)", Pattern: "*.xlsx"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return coreapp.Summary{}, err
	}
	if target == "" {
		return session.Summary(), nil
	}
	if err := session.Save(target); err != nil {
		return coreapp.Summary{}, err
	}
	if err := c.prefs.RecordSaveTarget(target); err != nil {
		runtime.LogWarningf(ctx, "record last Save As directory: %v", err)
	}
	return session.Summary(), nil
}

func (c *Controller) getSession() (*coreapp.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return nil, fmt.Errorf("请先选择左右工作簿或仓库中的 XLSX 文件")
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
	session, ctx := c.session, c.ctx
	c.mu.Unlock()
	if session == nil || !session.Dirty() {
		return nil
	}
	var answer string
	var err error
	if c.switchPrompt != nil {
		answer, err = c.switchPrompt()
	} else {
		if ctx == nil || ctx.Value("frontend") == nil {
			return errors.New("当前表格存在未保存的修改，无法在无界面确认的情况下切换")
		}
		answer, err = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type: runtime.QuestionDialog, Title: "当前表格存在未保存的修改",
			Message:       "当前表格存在未保存的修改。",
			Buttons:       []string{"保存并继续", "不保存并继续", "取消"},
			DefaultButton: "保存并继续", CancelButton: "取消",
		})
	}
	if err != nil {
		return err
	}
	switch answer {
	case "保存并继续":
		_, err := c.Save()
		return err
	case "不保存并继续":
		return nil
	default:
		return errors.New("已取消切换")
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
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		result, err := c.OpenRepository(first)
		if multiple && err == nil {
			c.mu.Lock()
			c.repositoryView.Notice = "已打开第一个目录，其余拖入项已忽略"
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

func workspaceLabel(view RepositoryView) string {
	if view.Detached {
		return "当前工作区 · Detached HEAD"
	}
	return "当前工作区 · " + view.CurrentBranch
}

func cloneRepositoryView(source RepositoryView) RepositoryView {
	copy := source
	copy.Files = append([]string{}, source.Files...)
	copy.Branches = append([]repository.Branch{}, source.Branches...)
	return copy
}

func removeTemporary(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
