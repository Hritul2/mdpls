package root

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindUsesGitRoot(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	deep := filepath.Join(repo, "docs", "guide")
	mustMkdir(t, deep)
	if got := Find(deep); !same(t, got, repo) {
		t.Fatalf("Find = %q, want %q", got, repo)
	}
}

func TestFindGitFileWorktree(t *testing.T) {
	// In a git worktree/submodule .git is a file, not a directory.
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Find(repo); !same(t, got, repo) {
		t.Fatalf("Find = %q, want %q", got, repo)
	}
}

func TestFindFallsBackToStart(t *testing.T) {
	dir := t.TempDir()
	if got := Find(dir); !same(t, got, dir) {
		t.Fatalf("Find = %q, want %q", got, dir)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func same(t *testing.T, a, b string) bool {
	t.Helper()
	ra, _ := filepath.EvalSymlinks(a)
	rb, _ := filepath.EvalSymlinks(b)
	return ra == rb
}
