// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
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
