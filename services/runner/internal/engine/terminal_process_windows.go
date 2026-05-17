//go:build windows

package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureTerminalCommand(cmd *exec.Cmd) {
}

func terminateTerminalProcessTree(process *os.Process) error {
	if process == nil {
		return nil
	}

	cmd := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", process.Pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if killErr := process.Kill(); killErr == nil || errors.Is(killErr, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("taskkill /T /F /PID %d: %w: %s", process.Pid, err, string(output))
	}

	return nil
}
