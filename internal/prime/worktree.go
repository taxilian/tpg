package prime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeContext contains information about current worktree
type WorktreeContext struct {
	IsWorktree      bool   // Are we in a worktree?
	WorktreePath    string // Path to worktree root
	Branch          string // Current branch name
	IsFeatureBranch bool   // Is this a feature/epic branch?
	AssociatedEpic  string // Epic ID if branch matches pattern
}

// worktreeInfo represents a single worktree from git worktree list
type worktreeInfo struct {
	Path   string
	Branch string
}

// DetectWorktree checks if current directory is in a worktree
func DetectWorktree() (*WorktreeContext, error) {
	ctx := &WorktreeContext{}

	// Get git worktree list
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		// Not a git repo or no worktrees
		return ctx, nil
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Parse worktree list to find if cwd is in a worktree
	worktrees := parseWorktreeList(string(output))
	for _, wt := range worktrees {
		// Check if current directory is within this worktree
		if strings.HasPrefix(cwd, wt.Path) && cwd != wt.Path {
			// We're in a worktree subdirectory, but check if it's the main repo
			// Main repo is typically the first entry and has no branch prefix
			continue
		}
		if cwd == wt.Path || strings.HasPrefix(cwd, wt.Path+string(filepath.Separator)) {
			// Check if this is the main worktree (no branch means main repo)
			if wt.Branch == "" {
				continue
			}
			ctx.IsWorktree = true
			ctx.WorktreePath = wt.Path
			ctx.Branch = wt.Branch
			ctx.IsFeatureBranch = strings.HasPrefix(wt.Branch, "feature/ep-") || strings.HasPrefix(wt.Branch, "refs/heads/feature/ep-")
			ctx.AssociatedEpic = extractEpicID(wt.Branch)
			break
		}
	}

	return ctx, nil
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) []worktreeInfo {
	var worktrees []worktreeInfo
	var current worktreeInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = worktreeInfo{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = worktreeInfo{
				Path: strings.TrimPrefix(line, "worktree "),
			}
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			// Remove refs/heads/ prefix if present
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		} else if strings.HasPrefix(line, "HEAD ") {
			// Detached HEAD state - we'll skip this worktree
			current.Branch = ""
		}
	}

	// Add the last worktree if any
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees
}

// extractEpicID extracts epic ID from branch name
// Pattern: feature/ep-xxx-... or feature/ep-xxx
func extractEpicID(branch string) string {
	// Remove refs/heads/ prefix if present
	branch = strings.TrimPrefix(branch, "refs/heads/")

	// Pattern: feature/ep-xxx-... or feature/ep-xxx
	if !strings.HasPrefix(branch, "feature/ep-") {
		return ""
	}

	// Remove "feature/" prefix
	id := strings.TrimPrefix(branch, "feature/")

	// Extract ep-xxx part (up to first dash after ep- or end of string)
	parts := strings.SplitN(id, "-", 3)
	if len(parts) >= 2 && parts[0] == "ep" {
		return "ep-" + parts[1]
	}

	return ""
}
