// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package plugin

import (
	"syscall"
	"testing"
	"time"
)

// TestClientConfig_DisableProcessGroupKill_WiresThroughToCmdRunner
// verifies that setting the flag on ClientConfig actually disables the
// Setpgid in the subprocess. Without this test the field could be
// accepted by ClientConfig but silently ignored on the CmdRunner path.
func TestClientConfig_DisableProcessGroupKill_WiresThroughToCmdRunner(t *testing.T) {
	process := helperProcess("mock")
	c := NewClient(&ClientConfig{
		Cmd:                     process,
		HandshakeConfig:         testHandshake,
		Plugins:                 testPluginMap,
		DisableProcessGroupKill: true,
	})
	defer c.Kill()

	if _, err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if process.SysProcAttr != nil && process.SysProcAttr.Setpgid {
		t.Fatalf("DisableProcessGroupKill=true must not enable Setpgid; got %+v", process.SysProcAttr)
	}

	// Sanity: the process is alive but not a group leader.
	if process.Process == nil {
		t.Fatalf("process did not start")
	}
	pgid, err := syscall.Getpgid(process.Process.Pid)
	if err != nil {
		t.Fatalf("Getpgid: %v", err)
	}
	if pgid == process.Process.Pid {
		t.Fatalf("process is a pgroup leader (pgid==pid=%d); opt-out not honoured", pgid)
	}
}

// TestClientConfig_DisableProcessGroupKill_Default confirms the default
// value enables the process-group path. Pairs with the above so a
// refactor of either direction is caught.
func TestClientConfig_DisableProcessGroupKill_Default(t *testing.T) {
	process := helperProcess("mock")
	c := NewClient(&ClientConfig{
		Cmd:             process,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
	})
	defer c.Kill()

	if _, err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if process.SysProcAttr == nil || !process.SysProcAttr.Setpgid {
		t.Fatalf("default must enable Setpgid; got %+v", process.SysProcAttr)
	}
}

// TestClientConfig_ShutdownTimeout_RespectsShorterValue verifies that a
// shorter ShutdownTimeout shortens the force-kill window in Client.Kill.
// Uses the "mock" helper which launches a process that does not respond
// to graceful shutdown within any reasonable window, so Kill always
// force-kills — the only observable contract is how long we waited
// before giving up. Passing this test requires the ShutdownTimeout
// field to actually replace the old 2s literal.
func TestClientConfig_ShutdownTimeout_RespectsShorterValue(t *testing.T) {
	// Shrink the default too so we can't accidentally pass by happy
	// coincidence if the config field is ignored and the default
	// happens to be small.
	prevDefault := defaultShutdownTimeout
	defaultShutdownTimeout = 5 * time.Second
	defer func() { defaultShutdownTimeout = prevDefault }()

	process := helperProcess("test-interface")
	c := NewClient(&ClientConfig{
		Cmd:             process,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
		ShutdownTimeout: 200 * time.Millisecond,
	})

	if _, err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := c.Client(); err != nil {
		t.Fatalf("Client: %v", err)
	}

	start := time.Now()
	c.Kill()
	elapsed := time.Since(start)

	// Expect well under defaultShutdownTimeout (5s). The configured
	// 200ms plus cleanup overhead should still fit in ~2s.
	if elapsed > 2*time.Second {
		t.Fatalf("Kill respected the 5s default, not the 200ms config; elapsed=%v", elapsed)
	}
}
