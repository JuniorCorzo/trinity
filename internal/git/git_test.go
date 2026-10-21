package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func runCmd(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed in %s: %v\nOutput: %s", name, args, dir, err, string(out))
	}
	return string(out)
}

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runCmd(t, dir, "git", "init", "-b", "main")
	runCmd(t, dir, "git", "config", "user.email", "dev@example.com")
	runCmd(t, dir, "git", "config", "user.name", "Dev TUI Tester")

	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "add", "README.md")
	runCmd(t, dir, "git", "commit", "-m", "Initial commit")

	return dir
}

func TestGitInfo_StandardRepo(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	info, err := GetInfo(repoDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.IsGit {
		t.Fatalf("expected IsGit to be true")
	}
	if info.Branch != "main" {
		t.Errorf("expected branch main, got %s", info.Branch)
	}
	if info.IsLinked {
		t.Errorf("expected IsLinked to be false in standard repo")
	}
	if info.Commit == "" {
		t.Errorf("expected commit hash to be non-empty")
	}
	if info.Dirty {
		t.Errorf("expected clean repository, got dirty")
	}

	// Make dirty
	file := filepath.Join(repoDir, "README.md")
	_ = os.WriteFile(file, []byte("# Test Repo Modified\n"), 0644)

	infoDirty, _ := GetInfo(repoDir)
	if !infoDirty.Dirty {
		t.Errorf("expected dirty status after modifying tracked file")
	}
	if !HasChanged(info, infoDirty) {
		t.Errorf("expected HasChanged to return true when dirty status changes")
	}
}

func TestGitInfo_BranchSwitch(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	info1, err := GetInfo(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	// Switch branch
	runCmd(t, repoDir, "git", "checkout", "-b", "feature/login")

	info2, err := GetInfo(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	if info2.Branch != "feature/login" {
		t.Errorf("expected branch feature/login, got %s", info2.Branch)
	}
	if !HasChanged(info1, info2) {
		t.Errorf("expected HasChanged to return true after branch switch")
	}
}

func TestGitInfo_Worktrees(t *testing.T) {
	mainDir := setupTestGitRepo(t)
	parentDir := filepath.Dir(mainDir)
	wtDir := filepath.Join(parentDir, "linked-wt-feat")

	// Add worktree
	runCmd(t, mainDir, "git", "worktree", "add", wtDir, "-b", "feature/worktree-branch")

	// Inspect linked worktree directory
	infoWT, err := GetInfo(wtDir)
	if err != nil {
		t.Fatalf("unexpected error inspecting worktree: %v", err)
	}

	if !infoWT.IsGit {
		t.Fatalf("expected IsGit to be true for worktree")
	}
	if !infoWT.IsLinked {
		t.Errorf("expected IsLinked to be true for linked worktree")
	}
	if !infoWT.IsWorktree {
		t.Errorf("expected IsWorktree to be true")
	}
	if infoWT.Branch != "feature/worktree-branch" {
		t.Errorf("expected branch feature/worktree-branch, got %s", infoWT.Branch)
	}
	if infoWT.WorktreeName != "linked-wt-feat" {
		t.Errorf("expected WorktreeName linked-wt-feat, got %s", infoWT.WorktreeName)
	}

	// Worktrees list should contain both main and linked worktree
	if len(infoWT.Worktrees) < 2 {
		t.Errorf("expected at least 2 worktrees, got %d", len(infoWT.Worktrees))
	}

	// Verify main repo also lists worktrees
	infoMain, err := GetInfo(mainDir)
	if err != nil {
		t.Fatal(err)
	}
	if !infoMain.IsWorktree {
		t.Errorf("expected main repo to recognize worktrees exist")
	}
	if infoMain.IsLinked {
		t.Errorf("expected main repo IsLinked to be false")
	}
	if len(infoMain.Worktrees) < 2 {
		t.Errorf("expected at least 2 worktrees from main repo, got %d", len(infoMain.Worktrees))
	}
}

func TestGitInfo_NonGitDirectory(t *testing.T) {
	dir := t.TempDir()
	info, err := GetInfo(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.IsGit {
		t.Errorf("expected IsGit to be false for non-git directory")
	}
}

func TestGitInfo_FallbackInspect(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	fallback := fallbackInspect(repoDir)
	if !fallback.IsGit {
		t.Errorf("expected fallbackInspect to detect git directory")
	}
	if fallback.Branch != "main" {
		t.Errorf("expected branch main, got %s", fallback.Branch)
	}
}
