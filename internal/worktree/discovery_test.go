package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLabelAndHostname(t *testing.T) {
	if got := Label("Feature/API_V2@TEST"); got != "feature-api-v2-test" {
		t.Fatalf("label = %q", got)
	}
	a := Hostname("My Repo", "feature/auth", "/tmp/worktree-a", false, false)
	b := Hostname("My Repo", "feature_auth", "/tmp/worktree-b", false, false)
	if a == b {
		t.Fatal("distinct worktree paths produced same hostname")
	}
	if !strings.HasSuffix(a, ".my-repo.localhost") {
		t.Fatalf("unexpected hostname: %s", a)
	}
	if a != Hostname("My Repo", "feature/auth", "/tmp/worktree-a", false, false) {
		t.Fatal("hostname unstable")
	}
	if !strings.HasPrefix(Hostname("repo", "", "/tmp/a", false, true), "detached-") {
		t.Fatal("detached naming")
	}
	if !strings.HasPrefix(Hostname("repo", "main", "/tmp/a", true, false), "main-") {
		t.Fatal("main naming")
	}
	long := Hostname(strings.Repeat("x", 200), strings.Repeat("y", 200), "/tmp/a", false, false)
	for _, label := range strings.Split(long, ".") {
		if len(label) > 63 {
			t.Fatalf("oversized DNS label: %q", label)
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestDiscoverLinkedWorktreeAndDetached(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	parent := t.TempDir()
	repo := filepath.Join(parent, "project")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial")
	main, err := Discover(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !main.Main || main.Detached || !strings.HasSuffix(main.Host, ".project.localhost") {
		t.Fatalf("main: %+v", main)
	}
	linked := filepath.Join(parent, "feature")
	runGit(t, repo, "worktree", "add", "-qb", "feature/auth", linked)
	info, err := Discover(linked)
	if err != nil {
		t.Fatal(err)
	}
	if info.Main || info.Detached || info.Branch != "feature/auth" || info.Host == main.Host {
		t.Fatalf("linked: %+v", info)
	}
	nested := filepath.Join(linked, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	fromNested, err := Discover(nested)
	if err != nil || fromNested.Host != info.Host {
		t.Fatalf("nested: %+v, %v", fromNested, err)
	}
	runGit(t, linked, "checkout", "--detach", "-q")
	detached, err := Discover(linked)
	if err != nil || !detached.Detached || !strings.HasPrefix(detached.Host, "detached-") {
		t.Fatalf("detached: %+v, %v", detached, err)
	}
	if _, err := Discover(parent); err == nil {
		t.Fatal("non-repository directory accepted")
	}
}
