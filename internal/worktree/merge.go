package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MergeResult contains the outcome of a worktree merge operation.
type MergeResult struct {
	EpicID           string
	Branch           string
	BaseBranch       string
	WorktreePath     string
	ConflictOccurred bool
	Message          string
}

// MergeWorktree executes the worktree merge protocol for an epic.
// Returns MergeResult with details about the merge operation.
//
// Protocol:
//  1. Verify worktree is clean (no uncommitted changes)
//  2. cd to worktree path
//  3. git rebase <parent_branch>
//  4. git checkout <parent_branch>
//  5. git merge --ff-only <worktree_branch>
//  6. If step 5 fails (parent moved): retry from step 3
//
// If conflicts occur during rebase, returns error with ConflictOccurred=true.
func MergeWorktree(epicID, worktreeBranch, parentBranch, worktreePath string) (*MergeResult, error) {
	result := &MergeResult{
		EpicID:       epicID,
		Branch:       worktreeBranch,
		BaseBranch:   parentBranch,
		WorktreePath: worktreePath,
	}

	// Step 1: Verify worktree is clean
	if err := verifyCleanWorktree(worktreePath); err != nil {
		return result, fmt.Errorf("worktree not clean: %w", err)
	}

	// Step 2-3: Rebase onto parent branch
	if err := rebaseOntoParent(worktreePath, worktreeBranch, parentBranch); err != nil {
		// Check if it's a conflict
		if isRebaseConflict(worktreePath) {
			result.ConflictOccurred = true
			result.Message = "Rebase conflicts detected - manual resolution required"
			return result, fmt.Errorf("rebase conflicts: %w", err)
		}
		return result, fmt.Errorf("rebase failed: %w", err)
	}

	// Step 4-5: Checkout parent and fast-forward merge
	// Retry up to 3 times if ff-only fails (parent may have moved)
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := fastForwardMerge(worktreePath, worktreeBranch, parentBranch)
		if err == nil {
			result.Message = "Merge successful"
			return result, nil
		}

		// Check if it's a non-fast-forward error
		if !isNonFastForward(err) {
			return result, fmt.Errorf("merge failed: %w", err)
		}

		// Parent moved - retry rebase
		if attempt < maxRetries {
			result.Message = fmt.Sprintf("Parent branch moved, retrying rebase (attempt %d/%d)", attempt, maxRetries)
			if err := rebaseOntoParent(worktreePath, worktreeBranch, parentBranch); err != nil {
				if isRebaseConflict(worktreePath) {
					result.ConflictOccurred = true
					result.Message = "Rebase conflicts after parent moved - manual resolution required"
					return result, fmt.Errorf("rebase conflicts on retry: %w", err)
				}
				return result, fmt.Errorf("rebase retry failed: %w", err)
			}
		} else {
			return result, fmt.Errorf("exceeded max retries: parent branch keeps moving")
		}
	}

	return result, fmt.Errorf("merge failed after %d attempts", maxRetries)
}

// verifyCleanWorktree checks if the worktree has uncommitted changes.
func verifyCleanWorktree(worktreePath string) error {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("uncommitted changes in worktree:\n%s", string(output))
	}

	return nil
}

// rebaseOntoParent rebases the worktree branch onto the parent branch.
func rebaseOntoParent(worktreePath, worktreeBranch, parentBranch string) error {
	// First ensure we're on the worktree branch
	checkoutCmd := exec.Command("git", "checkout", worktreeBranch)
	checkoutCmd.Dir = worktreePath
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s failed: %w\n%s", worktreeBranch, err, string(output))
	}

	// Rebase onto parent
	cmd := exec.Command("git", "rebase", parentBranch)
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}

	return nil
}

// fastForwardMerge checks out parent branch and performs ff-only merge.
func fastForwardMerge(worktreePath, worktreeBranch, parentBranch string) error {
	// Checkout parent branch
	checkoutCmd := exec.Command("git", "checkout", parentBranch)
	checkoutCmd.Dir = worktreePath
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s failed: %w\n%s", parentBranch, err, string(output))
	}

	// Merge with ff-only
	mergeCmd := exec.Command("git", "merge", "--ff-only", worktreeBranch)
	mergeCmd.Dir = worktreePath
	output, err := mergeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, string(output))
	}

	return nil
}

// isRebaseConflict checks if a rebase is in progress (indicating conflicts).
func isRebaseConflict(worktreePath string) bool {
	rebaseMergePath := filepath.Join(worktreePath, ".git", "rebase-merge")
	rebaseApplyPath := filepath.Join(worktreePath, ".git", "rebase-apply")

	_, err1 := os.Stat(rebaseMergePath)
	_, err2 := os.Stat(rebaseApplyPath)

	return err1 == nil || err2 == nil
}

// isNonFastForward checks if the error is due to non-fast-forward merge.
func isNonFastForward(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "not possible to fast-forward") ||
		strings.Contains(errMsg, "non-fast-forward")
}
