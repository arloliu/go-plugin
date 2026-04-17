// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cmdrunner

import (
	"context"
	"os/exec"
	"syscall"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestCmdRunner_DisableProcessGroup verifies that the opt-out path keeps
// SysProcAttr.Setpgid off and leaves Kill to signal the plugin PID only.
// Host integrators that need TTY-interactive plugin children rely on this
// backward-compatible behaviour.
func TestCmdRunner_DisableProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 5")
	r, err := NewCmdRunner(hclog.NewNullLogger(), cmd)
	if err != nil {
		t.Fatalf("NewCmdRunner: %v", err)
	}
	r.SetDisableProcessGroup(true)

	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = r.Kill(context.Background())
		_ = r.Wait(context.Background())
	}()

	if cmd.SysProcAttr == nil {
		return // zero-valued attrs: Setpgid is false as required.
	}
	if cmd.SysProcAttr.Setpgid {
		t.Fatalf("SetDisableProcessGroup(true) should leave Setpgid unset; got true")
	}
}

// TestCmdRunner_DefaultEnablesProcessGroup protects the new default. If a
// refactor silently stops setting Setpgid, orphaned subprocesses return.
func TestCmdRunner_DefaultEnablesProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 5")
	r, err := NewCmdRunner(hclog.NewNullLogger(), cmd)
	if err != nil {
		t.Fatalf("NewCmdRunner: %v", err)
	}

	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = r.Kill(context.Background())
		_ = r.Wait(context.Background())
	}()

	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatalf("default runner must enable Setpgid; got %+v", cmd.SysProcAttr)
	}

	// Sanity: the started process should actually be a process group
	// leader, i.e. getpgid(pid) == pid.
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("Getpgid: %v", err)
	}
	if pgid != cmd.Process.Pid {
		t.Fatalf("expected plugin to be a process-group leader (pgid==pid=%d), got pgid=%d", cmd.Process.Pid, pgid)
	}
}
