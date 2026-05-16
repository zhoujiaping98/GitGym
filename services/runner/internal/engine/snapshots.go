package engine

import (
	"os/exec"
	"strings"
	"time"
)

type Snapshot struct {
	HeadCommit    string
	BranchName    string
	StatusSummary []string
	CapturedAt    time.Time
}

func CaptureSnapshot(workspacePath string) (Snapshot, error) {
	headBytes, err := exec.Command("git", "-C", workspacePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return Snapshot{}, err
	}

	branchBytes, err := exec.Command("git", "-C", workspacePath, "branch", "--show-current").Output()
	if err != nil {
		return Snapshot{}, err
	}

	statusBytes, err := exec.Command("git", "-C", workspacePath, "status", "--short").Output()
	if err != nil {
		return Snapshot{}, err
	}

	status := strings.TrimSpace(string(statusBytes))
	summary := []string{}
	if status != "" {
		summary = strings.Split(status, "\n")
	}

	return Snapshot{
		HeadCommit:    strings.TrimSpace(string(headBytes)),
		BranchName:    strings.TrimSpace(string(branchBytes)),
		StatusSummary: summary,
		CapturedAt:    time.Now().UTC(),
	}, nil
}
