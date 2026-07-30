package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	coreapp "github.com/ug-tools/ugxlsx/internal/app"
	"github.com/ug-tools/ugxlsx/internal/preferences"
	"github.com/ug-tools/ugxlsx/internal/workbook"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Controller struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	ctx     context.Context
	session *coreapp.Session
	left    string
	right   string
	options coreapp.Options
	loading bool
	loadErr string
	prefs   preferences.Store
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
	shouldLoad := c.left != "" && c.right != ""
	c.loading = shouldLoad
	c.mu.Unlock()
	if shouldLoad {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.openInitial()
		}()
	}
}

func (c *Controller) openInitial() {
	c.mu.Lock()
	left, right, options := c.left, c.right, c.options
	c.mu.Unlock()
	session, err := coreapp.Open(left, right, options)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loading = false
	if err != nil {
		c.loadErr = err.Error()
		return
	}
	c.session = session
}

type BootstrapState struct {
	Loading    bool   `json:"loading"`
	HasSession bool   `json:"hasSession"`
	Error      string `json:"error"`
}

func (c *Controller) Bootstrap() BootstrapState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return BootstrapState{
		Loading: c.loading, HasSession: c.session != nil, Error: c.loadErr,
	}
}

func (c *Controller) shutdown(_ context.Context) {
	c.wg.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != nil {
		_ = c.session.Close()
	}
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
	c.mu.Lock()
	old := c.session
	c.session = session
	c.left, c.right = left, right
	c.loadErr = ""
	c.loading = false
	c.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return session.Summary(), nil
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
	if err := session.Save(""); err != nil {
		return coreapp.Summary{}, err
	}
	return session.Summary(), nil
}

func (c *Controller) SaveAs() (coreapp.Summary, error) {
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
		return nil, fmt.Errorf("请先选择左右工作簿")
	}
	return c.session, nil
}
