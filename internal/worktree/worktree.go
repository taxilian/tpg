package worktree

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Context contains git repository and worktree information for a directory.
type Context struct {
	RepoRoot      string
	GitDir        string
	WorktreeRoot  string
	InWorktree    bool
	CurrentBranch string
}

// DetectContext inspects the filesystem to determine repo/worktree context.
// It performs only file operations and does not invoke git.
func DetectContext(startDir string) (*Context, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		startDir = cwd
	}

	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				branch, _ := readHeadBranch(filepath.Join(gitPath, "HEAD"))
				return &Context{
					RepoRoot:      dir,
					GitDir:        gitPath,
					CurrentBranch: branch,
				}, nil
			}

			gitDir, repoRoot, inWorktree, err := parseGitFile(gitPath)
			if err != nil {
				return nil, err
			}
			branch, _ := readHeadBranch(filepath.Join(gitDir, "HEAD"))
			ctx := &Context{
				RepoRoot:      repoRoot,
				GitDir:        gitDir,
				InWorktree:    inWorktree,
				CurrentBranch: branch,
			}
			if inWorktree {
				ctx.WorktreeRoot = filepath.Dir(gitPath)
			}
			return ctx, nil
		}

		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to check %s: %w", gitPath, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return &Context{}, nil
		}
		dir = parent
	}
}

// ListWorktrees returns a map of branch name to worktree root path.
// repoRoot should be the main repository root (parent of .git directory).
func ListWorktrees(repoRoot string) (map[string]string, error) {
	worktrees := map[string]string{}
	if repoRoot == "" {
		return worktrees, nil
	}

	worktreesDir := filepath.Join(repoRoot, ".git", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return worktrees, nil
		}
		return nil, fmt.Errorf("failed to read worktrees directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		headPath := filepath.Join(worktreesDir, entry.Name(), "HEAD")
		branch, err := readHeadBranch(headPath)
		if err != nil || branch == "" {
			continue
		}

		gitdirPath := filepath.Join(worktreesDir, entry.Name(), "gitdir")
		gitDir, err := readGitdirPointer(gitdirPath)
		if err != nil || gitDir == "" {
			continue
		}
		worktreeRoot := filepath.Dir(gitDir)
		worktrees[branch] = worktreeRoot
	}

	return worktrees, nil
}

// IsWithinDir reports whether path is within dir (or equal to dir).
func IsWithinDir(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}

func readHeadBranch(headPath string) (string, error) {
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return "", nil
	}
	const prefix = "ref:"
	if !strings.HasPrefix(line, prefix) {
		return "", nil
	}
	ref := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	ref = strings.TrimPrefix(ref, "refs/heads/")
	return ref, nil
}

func parseGitFile(gitFilePath string) (gitDir string, repoRoot string, inWorktree bool, err error) {
	file, err := os.Open(gitFilePath)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to open .git file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", "", false, fmt.Errorf("failed to read .git file: %w", err)
		}
		return "", "", false, fmt.Errorf(".git file is empty")
	}

	line := strings.TrimSpace(scanner.Text())
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false, fmt.Errorf("malformed .git file: missing gitdir prefix")
	}
	gitDir = strings.TrimSpace(line[len(prefix):])
	if gitDir == "" {
		return "", "", false, fmt.Errorf("malformed .git file: empty gitdir path")
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(filepath.Dir(gitFilePath), gitDir)
	}
	gitDir = filepath.Clean(gitDir)

	worktreesMarker := string(filepath.Separator) + "worktrees" + string(filepath.Separator)
	modulesMarker := string(filepath.Separator) + "modules" + string(filepath.Separator)

	mainGitDir := gitDir
	if idx := strings.Index(gitDir, worktreesMarker); idx >= 0 {
		inWorktree = true
		mainGitDir = gitDir[:idx]
	} else if idx := strings.Index(gitDir, modulesMarker); idx >= 0 {
		mainGitDir = gitDir[:idx]
	}

	repoRoot = filepath.Dir(mainGitDir)
	return gitDir, repoRoot, inWorktree, nil
}

func readGitdirPointer(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return "", nil
	}
	if !filepath.IsAbs(line) {
		line = filepath.Join(filepath.Dir(path), line)
	}
	return filepath.Clean(line), nil
}
