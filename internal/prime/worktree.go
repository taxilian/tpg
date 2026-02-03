package prime

import (
	"os"
	"strings"

	"github.com/taxilian/tpg/internal/worktree"
)

// WorktreeContext contains information about current worktree
type WorktreeContext struct {
	IsWorktree      bool   // Are we in a worktree?
	WorktreePath    string // Path to worktree root
	Branch          string // Current branch name
	IsFeatureBranch bool   // Is this a feature/epic branch?
	AssociatedEpic  string // Epic ID if branch matches pattern
}

// DetectWorktree checks if current directory is in a worktree
func DetectWorktree() (*WorktreeContext, error) {
	ctx := &WorktreeContext{}

	wctx, err := worktree.DetectContext("")
	if err != nil {
		return nil, err
	}
	if wctx.RepoRoot == "" {
		return ctx, nil
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Determine if current directory is within a worktree
	if wctx.InWorktree {
		ctx.IsWorktree = true
		ctx.WorktreePath = wctx.WorktreeRoot
		ctx.Branch = wctx.CurrentBranch
	}

	// Fallback: if not in worktree but on a worktree branch, detect that branch exists
	if !ctx.IsWorktree {
		worktrees, err := worktree.ListWorktrees(wctx.RepoRoot)
		if err == nil {
			if path, ok := worktrees[wctx.CurrentBranch]; ok {
				ctx.WorktreePath = path
				ctx.Branch = wctx.CurrentBranch
			}
		}
	}

	if ctx.Branch != "" {
		ctx.IsFeatureBranch = strings.HasPrefix(ctx.Branch, "feature/ep-") || strings.HasPrefix(ctx.Branch, "refs/heads/feature/ep-")
		ctx.AssociatedEpic = extractEpicID(ctx.Branch)
	}

	// If cwd is within a known worktree path, mark as worktree
	if ctx.WorktreePath != "" && worktree.IsWithinDir(cwd, ctx.WorktreePath) {
		ctx.IsWorktree = true
	}

	return ctx, nil
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
