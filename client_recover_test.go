// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/arloliu/go-plugin/runner"
)

// panicWriter panics on every Write. Used to prove that a misbehaving
// user-supplied io.Writer cannot take the host down through the stderr
// log pump.
type panicWriter struct{}

func (panicWriter) Write(p []byte) (int, error) { panic("intentional test panic") }

// recordWriter just captures written bytes under a mutex so the test can
// inspect that the recover() branch was exercised without racing.
type recordWriter struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (r *recordWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

func (r *recordWriter) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// TestLogStderr_PanicRecovered feeds data into the logStderr goroutine via
// an io.Pipe, configures ClientConfig.Stderr to a writer that panics, and
// asserts the host process is not killed. Before the recover() was added,
// a panic in any of the user-supplied writers or hclog call paths would
// crash the host.
func TestLogStderr_PanicRecovered(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()

	logs := &recordWriter{}
	c := &Client{
		config: &ClientConfig{
			Stderr:              panicWriter{},
			PluginLogBufferSize: defaultPluginLogBufferSize,
		},
		logger: hclog.New(&hclog.LoggerOptions{
			Output: logs,
			Level:  hclog.Trace,
		}),
	}
	c.clientWaitGroup.Add(1)
	c.pipesWaitGroup.Add(1)

	done := make(chan struct{})
	go func() {
		c.logStderr("fake-plugin", pr)
		close(done)
	}()

	// Feed one line; panicWriter.Write panics on receipt.
	if _, err := pw.Write([]byte("hello\n")); err != nil {
		t.Fatalf("write to pipe: %v", err)
	}

	// The goroutine should return (recovered), not crash the process.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("logStderr did not exit after panic; process would have been at risk")
	}

	if !strings.Contains(logs.String(), "panic in plugin stderr log pump") {
		t.Fatalf("expected recovered-panic log line, got: %s", logs.String())
	}
}

// ensure package compiles if tests reference this error alias
var _ = errors.New

// panicReadCloser panics on first Read. Used to drive a fake runner
// whose Stdout() returns it — this exercises the production scanner
// goroutine inside Client.Start, not a stand-in reproduction.
type panicReadCloser struct{ fired bool }

func (p *panicReadCloser) Read([]byte) (int, error) {
	if !p.fired {
		p.fired = true
		panic("intentional scanner panic")
	}
	return 0, io.EOF
}
func (p *panicReadCloser) Close() error { return nil }

// fakeRunner is a minimal runner.Runner whose lifecycle methods
// support driving Client.Start far enough to spin up the stderr pump
// and stdout scanner goroutines. After Start the test kills the runner
// which unblocks the Wait call.
type fakeRunner struct {
	stdout io.ReadCloser
	stderr io.ReadCloser
	dead   chan struct{}
}

func newFakeRunner(stdout io.ReadCloser) *fakeRunner {
	return &fakeRunner{
		stdout: stdout,
		stderr: io.NopCloser(strings.NewReader("")),
		dead:   make(chan struct{}),
	}
}

func (f *fakeRunner) Start(ctx context.Context) error { return nil }
func (f *fakeRunner) Wait(ctx context.Context) error  { <-f.dead; return nil }
func (f *fakeRunner) Kill(ctx context.Context) error {
	select {
	case <-f.dead:
	default:
		close(f.dead)
	}
	return nil
}
func (f *fakeRunner) Stdout() io.ReadCloser { return f.stdout }
func (f *fakeRunner) Stderr() io.ReadCloser { return f.stderr }
func (f *fakeRunner) Name() string          { return "fake-runner" }
func (f *fakeRunner) ID() string            { return "fake-1" }
func (f *fakeRunner) Diagnose(ctx context.Context) string {
	return ""
}
func (f *fakeRunner) PluginToHost(n, a string) (string, string, error) { return n, a, nil }
func (f *fakeRunner) HostToPlugin(n, a string) (string, string, error) { return n, a, nil }

// TestStdoutScanner_PanicRecovered_Integration drives Client.Start with
// a runner whose Stdout() panics on first Read. The production scanner
// goroutine must recover the panic and log it; the host must not crash.
// Start itself will return an error (handshake fails because stdout
// yields no valid line), but the key assertion is that the recover
// path fired.
func TestStdoutScanner_PanicRecovered_Integration(t *testing.T) {
	logs := &recordWriter{}
	logger := hclog.New(&hclog.LoggerOptions{
		Output: logs,
		Level:  hclog.Trace,
	})

	// RunnerFunc injects our fake runner so Client.Start uses it.
	runnerFunc := func(_ hclog.Logger, _ *exec.Cmd, _ string) (runner.Runner, error) {
		return newFakeRunner(&panicReadCloser{}), nil
	}

	c := NewClient(&ClientConfig{
		HandshakeConfig: HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "TEST",
			MagicCookieValue: "TEST",
		},
		Plugins:      PluginSet{},
		RunnerFunc:   runnerFunc,
		Logger:       logger,
		StartTimeout: 500 * time.Millisecond,
	})

	// Start will fail (no valid handshake line), but not before the
	// scanner goroutine has taken the panic.
	_, _ = c.Start()
	c.Kill()

	// Recover is logged via the Client's own logger, which we installed.
	if !strings.Contains(logs.String(), "panic in plugin stdout scanner") {
		t.Fatalf("expected scanner recover log; got %q", logs.String())
	}
}
