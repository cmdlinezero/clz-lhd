// Package worktree discovers Git worktrees and derives stable local hostnames.
// It does not allocate ports, start processes, or register proxy routes.
package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// Info describes a Git worktree and its proposed local domain.
type Info struct {
	Root       string
	Repository string
	Branch     string
	Main       bool
	Detached   bool
	Host       string
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func gitPath(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return canonical(path)
}

// Label normalizes arbitrary branch/repository names to DNS-safe labels.
func Label(value string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(value) {
		if r <= unicode.MaxASCII && ((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	return b.String()
}

// Hostname includes a path-derived suffix so branches with similar names do
// not normally collide. The suffix is deterministic, not a global reservation.
func Hostname(repo, branch, root string, main, detached bool) string {
	name := Label(branch)
	if main {
		name = "main"
	} else if detached {
		name = "detached"
	}
	if name == "" {
		name = "worktree"
	}
	sum := sha256.Sum256([]byte(root))
	suffix := hex.EncodeToString(sum[:])[:10]
	// Reserve 11 characters for the hyphen and hash, within the 63-byte DNS label limit.
	if len(name) > 52 {
		name = strings.TrimRight(name[:52], "-")
	}
	repoLabel := Label(repo)
	if repoLabel == "" {
		repoLabel = "repo"
	}
	if len(repoLabel) > 63 {
		repoLabel = strings.TrimRight(repoLabel[:63], "-")
	}
	return name + "-" + suffix + "." + repoLabel + ".localhost"
}

// Discover locates the worktree containing dir, including linked worktrees and
// detached HEADs. It does not modify Git or the LHD server.
func Discover(dir string) (Info, error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return Info{}, fmt.Errorf("not inside a Git worktree: %w", err)
	}
	root, err = canonical(root)
	if err != nil {
		return Info{}, err
	}
	common, err := git(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Info{}, err
	}
	common, err = gitPath(root, common)
	if err != nil {
		return Info{}, err
	}
	gitdir, err := git(root, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return Info{}, err
	}
	gitdir, err = gitPath(root, gitdir)
	if err != nil {
		return Info{}, err
	}
	branch, err := git(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	detached := err != nil
	if detached {
		branch = ""
	}
	repo := strings.TrimSuffix(filepath.Base(filepath.Dir(common)), ".git")
	if filepath.Base(common) != ".git" {
		repo = strings.TrimSuffix(filepath.Base(common), ".git")
	}
	// The common directory is normally <repository>/.git, but bare repositories
	// use <repository>.git directly.
	if repo == "" || repo == "." {
		repo = filepath.Base(root)
	}
	main := gitdir == common
	info := Info{Root: root, Repository: repo, Branch: branch, Main: main, Detached: detached}
	info.Host = Hostname(repo, branch, root, main, detached)
	return info, nil
}

// DiscoverCurrent uses the current working directory.
func DiscoverCurrent() (Info, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Info{}, err
	}
	return Discover(dir)
}
