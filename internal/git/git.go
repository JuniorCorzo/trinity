package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrNotGitRepo = errors.New("not a git repository")
)

// WorktreeInfo holds metadata for a single git worktree
type WorktreeInfo struct {
	Path      string `json:"path"`
	Head      string `json:"head"`
	Branch    string `json:"branch"`
	IsCurrent bool   `json:"isCurrent"`
	IsMain    bool   `json:"isMain"`
	Locked    bool   `json:"locked,omitempty"`
}

// GitInfo holds complete Git and Worktree status for a directory
type GitInfo struct {
	IsGit        bool           `json:"isGit"`
	IsWorktree   bool           `json:"isWorktree"`
	IsLinked     bool           `json:"isLinked"`
	CurrentDir   string         `json:"currentDir"`
	WorktreeName string         `json:"worktreeName"`
	Branch       string         `json:"branch"`
	Commit       string         `json:"commit"`
	CommitMsg    string         `json:"commitMsg"`
	Dirty        bool           `json:"dirty"`
	RepoRoot     string         `json:"repoRoot"`
	GitDir       string         `json:"gitDir"`
	CommonDir    string         `json:"commonDir"`
	Worktrees    []WorktreeInfo `json:"worktrees"`
}

// GetInfo retrieves Git and Worktree metadata for target directory
func GetInfo(rootDir string) (*GitInfo, error) {
	return GetInfoWithTimeout(rootDir, 800*time.Millisecond)
}

// GetInfoWithTimeout retrieves Git metadata with a maximum deadline
func GetInfoWithTimeout(rootDir string, timeout time.Duration) (*GitInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		absRoot = rootDir
	}

	// 1. Verify if inside a git work tree
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = absRoot
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		// Fallback to manual inspection of .git file or directory
		return fallbackInspect(absRoot), nil
	}

	info := &GitInfo{
		IsGit:      true,
		CurrentDir: absRoot,
		Worktrees:  make([]WorktreeInfo, 0),
	}

	// Top level working directory
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		info.RepoRoot = strings.TrimSpace(string(out))
	} else {
		info.RepoRoot = absRoot
	}

	// Git dir (could be /path/.git or /path/.git/worktrees/<name>)
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		gitDir := strings.TrimSpace(string(out))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(info.RepoRoot, gitDir)
		}
		info.GitDir = filepath.Clean(gitDir)
	}

	// Common git dir (root repository .git)
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		commonDir := strings.TrimSpace(string(out))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(info.RepoRoot, commonDir)
		}
		info.CommonDir = filepath.Clean(commonDir)
	}

	// Determine if this is a linked worktree:
	// In linked worktrees, .git is a file, or gitDir is inside .git/worktrees/
	dotGitPath := filepath.Join(info.RepoRoot, ".git")
	if fi, err := os.Stat(dotGitPath); err == nil && !fi.IsDir() {
		info.IsWorktree = true
		info.IsLinked = true
	} else if info.GitDir != "" && info.CommonDir != "" && info.GitDir != info.CommonDir {
		info.IsWorktree = true
		info.IsLinked = true
	}

	if info.IsLinked {
		info.WorktreeName = filepath.Base(info.RepoRoot)
	}

	// Current branch
	cmd = exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		info.Branch = strings.TrimSpace(string(out))
	}
	if info.Branch == "" {
		// Detached HEAD fallback
		cmd = exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
		cmd.Dir = absRoot
		if out, err = cmd.Output(); err == nil {
			info.Branch = fmt.Sprintf("detached:%s", strings.TrimSpace(string(out)))
		} else {
			info.Branch = "unknown"
		}
	}

	// Commit SHA
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		info.Commit = strings.TrimSpace(string(out))
	}

	// Commit message
	cmd = exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%s")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		info.CommitMsg = strings.TrimSpace(string(out))
	}

	// Dirty status: tracked modifications
	cmd = exec.CommandContext(ctx, "git", "status", "--porcelain", "-uno")
	cmd.Dir = absRoot
	if out, err = cmd.Output(); err == nil {
		info.Dirty = len(bytes.TrimSpace(out)) > 0
	}

	// Worktree list
	wts, err := listWorktrees(ctx, absRoot, info.RepoRoot)
	if err == nil && len(wts) > 0 {
		info.Worktrees = wts
		if len(wts) > 1 {
			info.IsWorktree = true
		}
	}

	return info, nil
}

// listWorktrees runs `git worktree list --porcelain` and parses the entries
func listWorktrees(ctx context.Context, currentDir, repoRoot string) ([]WorktreeInfo, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = currentDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []WorktreeInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var current WTBuilder
	isFirst := true

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if current.Path != "" {
				current.IsMain = isFirst
				isFirst = false
				current.IsCurrent = filepath.Clean(current.Path) == filepath.Clean(repoRoot) ||
					filepath.Clean(current.Path) == filepath.Clean(currentDir)
				results = append(results, current.Build())
				current = WTBuilder{}
			}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		key := parts[0]
		val := ""
		if len(parts) > 1 {
			val = parts[1]
		}

		switch key {
		case "worktree":
			current.Path = val
		case "HEAD":
			if len(val) > 7 {
				current.Head = val[:7]
			} else {
				current.Head = val
			}
		case "branch":
			branch := strings.TrimPrefix(val, "refs/heads/")
			current.Branch = branch
		case "locked":
			current.Locked = true
		case "bare":
			if current.Branch == "" {
				current.Branch = "(bare)"
			}
		case "detached":
			if current.Branch == "" {
				current.Branch = fmt.Sprintf("detached:%s", current.Head)
			}
		}
	}

	if current.Path != "" {
		current.IsMain = isFirst
		current.IsCurrent = filepath.Clean(current.Path) == filepath.Clean(repoRoot) ||
			filepath.Clean(current.Path) == filepath.Clean(currentDir)
		results = append(results, current.Build())
	}

	return results, nil
}

type WTBuilder struct {
	Path      string
	Head      string
	Branch    string
	IsCurrent bool
	IsMain    bool
	Locked    bool
}

func (b WTBuilder) Build() WorktreeInfo {
	branch := b.Branch
	if branch == "" {
		branch = "HEAD"
	}
	return WorktreeInfo{
		Path:      b.Path,
		Head:      b.Head,
		Branch:    branch,
		IsCurrent: b.IsCurrent,
		IsMain:    b.IsMain,
		Locked:    b.Locked,
	}
}

// fallbackInspect reads filesystem .git file or folder if git CLI isn't responsive
func fallbackInspect(dir string) *GitInfo {
	cur := dir
	for i := 0; i < 6; i++ {
		gitPath := filepath.Join(cur, ".git")
		fi, err := os.Stat(gitPath)
		if err == nil {
			if fi.IsDir() {
				// Regular repo
				headContent, _ := os.ReadFile(filepath.Join(gitPath, "HEAD"))
				branch := parseBranchFromHead(string(headContent))
				return &GitInfo{
					IsGit:        true,
					IsWorktree:   false,
					RepoRoot:     cur,
					GitDir:       gitPath,
					CommonDir:    gitPath,
					Branch:       branch,
					WorktreeName: filepath.Base(cur),
				}
			}
			// Linked worktree (.git file)
			data, err := os.ReadFile(gitPath)
			if err == nil && strings.HasPrefix(string(data), "gitdir:") {
				targetGitDir := strings.TrimSpace(strings.TrimPrefix(string(data), "gitdir:"))
				if !filepath.IsAbs(targetGitDir) {
					targetGitDir = filepath.Join(cur, targetGitDir)
				}
				headContent, _ := os.ReadFile(filepath.Join(targetGitDir, "HEAD"))
				branch := parseBranchFromHead(string(headContent))
				return &GitInfo{
					IsGit:        true,
					IsWorktree:   true,
					IsLinked:     true,
					RepoRoot:     cur,
					GitDir:       targetGitDir,
					Branch:       branch,
					WorktreeName: filepath.Base(cur),
				}
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	return &GitInfo{IsGit: false}
}

func parseBranchFromHead(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	if len(content) > 7 {
		return content[:7]
	}
	return content
}

// HasChanged compares two GitInfo states to determine if branch, commit, dirty, or worktree changed
func HasChanged(oldInfo, newInfo *GitInfo) bool {
	if oldInfo == nil || newInfo == nil {
		return oldInfo != newInfo
	}
	if oldInfo.IsGit != newInfo.IsGit {
		return true
	}
	if oldInfo.Branch != newInfo.Branch {
		return true
	}
	if oldInfo.Commit != newInfo.Commit {
		return true
	}
	if oldInfo.IsWorktree != newInfo.IsWorktree {
		return true
	}
	if oldInfo.WorktreeName != newInfo.WorktreeName {
		return true
	}
	if oldInfo.Dirty != newInfo.Dirty {
		return true
	}
	if len(oldInfo.Worktrees) != len(newInfo.Worktrees) {
		return true
	}
	return false
}
