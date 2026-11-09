package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/tadazly/sheetproof/internal/ugit"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type UGitRepositoryScanResult struct {
	ScanID    uint64              `json:"scanID"`
	Cancelled bool                `json:"cancelled"`
	Scan      ugit.RepositoryScan `json:"scan"`
}

type UGitRepositoryRepairResult struct {
	Cancelled bool                    `json:"cancelled"`
	Results   []ugit.RepositoryRepair `json:"results"`
}

// Serializes configuration operations while allowing cancellation and shutdown.
func (c *Controller) beginUGitOperation() (context.Context, string, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	locale := c.options.Locale
	if c.shuttingDown || !c.ugitMu.TryLock() {
		return nil, locale, nil, errors.New(uiText(locale,
			"Another UGit operation is running.", "UGit 操作尚未结束。", "UGit の処理が実行中です。"))
	}
	parent := c.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	c.ugitOperationCancel = cancel
	c.wg.Add(1)
	return ctx, locale, func() {
		cancel()
		c.mu.Lock()
		c.ugitOperationCancel, c.ugitScanCancel = nil, nil
		c.mu.Unlock()
		c.ugitMu.Unlock()
		c.wg.Done()
	}, nil
}

func requireUGitDesktop(ctx context.Context, locale string) error {
	if ctx.Value("frontend") == nil {
		return errors.New(uiText(locale,
			"UGit setup requires the desktop interface.", "请在桌面界面中配置 UGit。", "UGit の設定はデスクトップ画面で行ってください。"))
	}
	return nil
}

func (c *Controller) ScanUGitRepositories() (UGitRepositoryScanResult, error) {
	ctx, locale, finish, err := c.beginUGitOperation()
	if err != nil {
		return UGitRepositoryScanResult{}, err
	}
	defer finish()
	if err := requireUGitDesktop(ctx, locale); err != nil {
		return UGitRepositoryScanResult{}, err
	}
	c.mu.Lock()
	c.ugitScanID++
	id := c.ugitScanID
	c.ugitScan = nil
	c.ugitScanCancel = c.ugitOperationCancel
	c.mu.Unlock()
	executable, err := ugit.CurrentExecutablePath()
	if err != nil {
		return UGitRepositoryScanResult{}, err
	}
	directory, err := runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: uiText(locale,
		"Choose a directory to scan for repositories", "选择要扫描的仓库目录", "リポジトリを検索するフォルダーを選択")})
	if err != nil {
		return UGitRepositoryScanResult{}, err
	}
	if directory == "" || ctx.Err() != nil {
		return UGitRepositoryScanResult{Cancelled: true}, nil
	}
	scan, err := ugit.ScanRepositories(ctx, directory, executable)
	if ctx.Err() != nil {
		return UGitRepositoryScanResult{Cancelled: true}, nil
	}
	if err != nil {
		return UGitRepositoryScanResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil {
		return UGitRepositoryScanResult{Cancelled: true}, nil
	}
	c.ugitScan = &scan
	return UGitRepositoryScanResult{ScanID: id, Scan: scan}, nil
}

func (c *Controller) CancelUGitRepositoryScan() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ugitScanCancel != nil {
		c.ugitScanCancel()
	}
}

func (c *Controller) RepairUGitRepositories(scanID uint64, paths []string) (UGitRepositoryRepairResult, error) {
	ctx, locale, finish, err := c.beginUGitOperation()
	if err != nil {
		return UGitRepositoryRepairResult{}, err
	}
	defer finish()
	if err := requireUGitDesktop(ctx, locale); err != nil {
		return UGitRepositoryRepairResult{}, err
	}
	c.mu.Lock()
	scan := c.ugitScan
	valid := scan != nil && scanID == c.ugitScanID
	c.mu.Unlock()
	if !valid {
		return UGitRepositoryRepairResult{}, errors.New(uiText(locale,
			"Scan results expired. Scan again.", "扫描结果已失效，请重新扫描。", "検索結果が無効です。再検索してください。"))
	}
	if err := scan.ValidateSelection(paths); err != nil {
		return UGitRepositoryRepairResult{}, err
	}
	confirm := uiText(locale, "Repair selected repositories", "修复所选仓库", "選択したリポジトリを修復")
	cancel := uiText(locale, "Cancel", "取消", "キャンセル")
	answer, err := c.ask(ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: confirm, Buttons: []string{cancel, confirm},
		DefaultButton: cancel, CancelButton: cancel,
		Message: fmt.Sprintf(uiText(locale,
			"Repair XLSX tool associations in the %d selected repositories under:\n%s\n\nGlobal XLSX entries will also point to this app. Wildcard (*.*) tools and associations for other file types are preserved. Repository configurations changed since scanning will be skipped. Restart UGit afterward.",
			"将修复以下目录中已勾选的 %d 个仓库的 XLSX 工具关联：\n%s\n\n同时将全局 XLSX 入口指向当前应用。保留通用（*.*）工具及其他文件类型的关联。扫描后发生配置变化的仓库将跳过。完成后请重启 UGit。",
			"次のフォルダー内で選択した %d 件のリポジトリの XLSX 関連付けを修復します：\n%s\n\nグローバルの XLSX 設定もこのアプリに変更します。汎用（*.*）ツールとほかの形式は保持します。検索後に設定が変わったリポジトリはスキップします。完了後に UGit を再起動してください。"), len(paths), scan.Directory),
	})
	if err != nil {
		return UGitRepositoryRepairResult{}, err
	}
	if answer != confirm {
		return UGitRepositoryRepairResult{Cancelled: true}, nil
	}
	results, err := scan.Repair(ctx, paths)
	c.mu.Lock()
	c.ugitScan = nil // A completed batch must be rescanned before another write.
	c.mu.Unlock()
	return UGitRepositoryRepairResult{Results: results}, err
}
