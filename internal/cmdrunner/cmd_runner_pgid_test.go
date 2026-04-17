// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cmdrunner

import (
	"bufio"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
)

// TestCmdRunner_KillProcessGroup verifies that Kill reaps a grandchild
// process (one the plugin forked). Before the process-group change this
// was orphaned on SIGKILL. The test is POSIX-only; Windows still uses
// the single-PID kill path until Job Objects are added.
func TestCmdRunner_KillProcessGroup(t *testing.T) {
	// Launch `sh -c` as a stand-in for a plugin process. It forks a
	// background `sleep 30`, prints the child PID, then waits. This is
	// the same shape as a plugin that shells out to external tooling.
	script := "sleep 30 & echo $! && wait"
	cmd := exec.Command("sh", "-c", script)

	logger := hclog.NewNullLogger()
	r, err := NewCmdRunner(logger, cmd)
	if err != nil {
		t.Fatalf("NewCmdRunner: %v", err)
	}

	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Read the child PID from the script's stdout.
	scanner := bufio.NewScanner(r.Stdout())
	if !scanner.Scan() {
		t.Fatalf("did not read child PID from script: %v", scanner.Err())
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		t.Fatalf("parse child PID: %v", err)
	}

	// Sanity check: child is alive.
	if err := syscall.Kill(childPID, 0); err != nil {
		t.Fatalf("child PID %d not alive before Kill: %v", childPID, err)
	}

	// Kill the plugin and let the waiter collect the exit.
	if err := r.Kill(context.Background()); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	_ = r.Wait(context.Background())

	// Poll: the child must disappear within a short window. Before the
	// fix this pid remained alive for the full sleep.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err != nil {
			// ESRCH = no such process → success.
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Cleanup: don't leak a real sleep process from a failing test.
	_ = syscall.Kill(childPID, syscall.SIGKILL)
	t.Fatalf("grandchild PID %d survived Kill; process-group kill did not reach it", childPID)
}
