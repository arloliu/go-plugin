// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package cmdrunner

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the plugin subprocess in its own process group
// so that Kill can signal the entire process tree the plugin spawns. Without
// this, a kill -9 to the plugin leaves any children (e.g. ssh, socat, or
// equipment driver subprocesses) orphaned and reparented to PID 1 — at
// restart-storm scale this accumulates zombies and leaks resources.
//
// If the caller has already supplied their own SysProcAttr we preserve it
// and just turn on Setpgid. We skip Setpgid when Setsid is already
// requested — a session leader cannot call setpgid on itself (it returns
// EPERM), and Setsid already gives us the new process group we need for
// group-wide kill.
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Setsid {
		return
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup sends SIGKILL to the process group whose group ID equals
// pid (guaranteed by configureProcessGroup setting Setpgid). A negative pid
// to syscall.Kill addresses the whole group.
func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
