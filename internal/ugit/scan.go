package ugit

import (
	"context"
	"errors"
	"slices"

	"github.com/tadazly/sheetproof/internal/repository"
)

type RepositoryCandidate struct {
	Path  string   `json:"path"`
	Tools []string `json:"tools"`
}

type RepositoryScan struct {
	Directory    string                      `json:"directory"`
	Inspected    int                         `json:"inspected"`
	Repositories []RepositoryCandidate       `json:"repositories"`
	Issues       []repository.DiscoveryIssue `json:"issues"`
	client       client
	inspections  map[string]Inspection
	executable   string
}

type RepositoryRepair struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func ScanRepositories(ctx context.Context, directory, executable string) (RepositoryScan, error) {
	c, err := newClient("")
	if err != nil {
		return RepositoryScan{}, err
	}
	return c.scanRepositories(ctx, directory, executable)
}

func (c client) scanRepositories(ctx context.Context, directory, executable string) (RepositoryScan, error) {
	if err := validateExecutablePath(executable); err != nil {
		return RepositoryScan{}, err
	}
	discovery, err := repository.DiscoverWithGit(ctx, directory, c.gitPath)
	if err != nil {
		return RepositoryScan{}, err
	}
	result := RepositoryScan{
		Directory: discovery.Directory, Repositories: []RepositoryCandidate{}, Issues: discovery.Issues,
		client: c, inspections: make(map[string]Inspection), executable: executable,
	}
	seen := make(map[string]bool)
	for _, root := range discovery.Roots {
		if err := ctx.Err(); err != nil {
			return RepositoryScan{}, err
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		c.repositoryRoot = root
		inspection, err := c.inspect(ctx, executable)
		if err != nil {
			if ctx.Err() != nil {
				return RepositoryScan{}, ctx.Err()
			}
			result.Issues = append(result.Issues, repository.DiscoveryIssue{Path: root, Message: err.Error()})
			continue
		}
		result.Inspected++
		if !needsLocalRepair(inspection) {
			continue
		}
		result.inspections[root] = inspection
		// Keep collection fields as JSON arrays even when a legacy command cannot
		// be parsed into an executable path.
		tools := append([]string{}, existingPaths(inspection.localEntries)...)
		result.Repositories = append(result.Repositories, RepositoryCandidate{Path: root, Tools: tools})
	}
	return result, nil
}

func needsLocalRepair(inspection Inspection) bool {
	if !inspection.LocalOverride {
		return false
	}
	var local []Entry
	for _, entry := range inspection.Existing {
		if entry.Scope == "local" {
			local = append(local, entry)
		}
	}
	desired := desiredEntries(inspection.ExecutablePath)
	for i := range desired {
		desired[i].Scope = "local"
	}
	return !entriesEqual(local, desired) || !directDiffFirst(local)
}

// ValidateSelection only accepts unique repositories from this scan. The caller
// must obtain confirmation before Repair, which rechecks each snapshot.
func (scan RepositoryScan) ValidateSelection(paths []string) error {
	if len(paths) == 0 {
		return errors.New("请选择需要修复的仓库")
	}
	seen := make(map[string]bool)
	for _, path := range paths {
		if _, ok := scan.inspections[path]; !ok || seen[path] {
			return errors.New("仓库不在本次扫描结果中，请重新扫描")
		}
		seen[path] = true
	}
	return nil
}

// Repair is per-repository transactional: one failure does not discard successful
// repairs elsewhere. Global XLSX registration is included in each transaction.
func (scan RepositoryScan) Repair(ctx context.Context, paths []string) ([]RepositoryRepair, error) {
	if err := scan.ValidateSelection(paths); err != nil {
		return nil, err
	}
	results := make([]RepositoryRepair, 0, len(paths))
	for _, path := range paths {
		result := RepositoryRepair{Path: path, Status: "failed"}
		before := scan.inspections[path]
		root, err := repository.FindRootWithGit(ctx, path, scan.client.gitPath)
		if err != nil {
			result.Message = err.Error()
		} else if root != before.RepositoryRoot {
			result.Status = "changed"
		} else {
			c := scan.client
			c.repositoryRoot = root
			current, inspectErr := c.inspect(ctx, scan.executable)
			switch {
			case inspectErr != nil:
				result.Message = inspectErr.Error()
			case !needsLocalRepair(current):
				// A linked worktree can share a config already repaired in this batch.
				result.Status = "unchanged"
			case !slices.Equal(before.localEntries, current.localEntries):
				result.Status = "changed"
			default:
				if _, err := c.configureChecked(ctx, scan.executable, before.localEntries); errors.Is(err, errConfigurationChanged) {
					result.Status = "changed"
				} else if err != nil {
					result.Message = err.Error()
				} else {
					result.Status = "repaired"
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}
