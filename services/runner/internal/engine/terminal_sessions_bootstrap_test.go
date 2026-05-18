package engine

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestPosixShellStartupSpecRequiresBash(t *testing.T) {
	lookups := make([]string, 0, 2)
	lookPath := func(file string) (string, error) {
		lookups = append(lookups, file)
		if file == "bash" {
			return "", exec.ErrNotFound
		}
		if file == "sh" {
			return "/bin/sh", nil
		}
		return "", errors.New("unexpected lookup")
	}

	_, _, _, err := posixShellStartupSpec(lookPath)
	if err == nil {
		t.Fatal("expected bash startup requirement error when bash is unavailable")
	}
	if !strings.Contains(err.Error(), "bash") {
		t.Fatalf("expected error to mention bash requirement, got %v", err)
	}
	if len(lookups) != 1 || lookups[0] != "bash" {
		t.Fatalf("expected only bash lookup before failing, got %v", lookups)
	}
}
