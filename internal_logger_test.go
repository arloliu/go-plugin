// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	hclog "github.com/hashicorp/go-hclog"
)

// TestSetInternalLogger_RoutesCopyStreamErrors exercises the SetInternalLogger
// seam. Previously these library-internal sites wrote to stdlib log.Printf;
// now they go through hclog so host log pipelines can ingest them.
func TestSetInternalLogger_RoutesCopyStreamErrors(t *testing.T) {
	var buf syncBuffer
	newLogger := hclog.New(&hclog.LoggerOptions{
		Output: &buf,
		Level:  hclog.Trace,
	})

	prev := libLog()
	SetInternalLogger(newLogger)
	defer SetInternalLogger(prev)

	// Trigger a known internal log: copyStream with nil src.
	copyStream("stdout", new(bytes.Buffer), nil)

	out := buf.String()
	if !strings.Contains(out, "stream copy aborted") {
		t.Fatalf("expected installed logger to receive internal log; got %q", out)
	}
}

// TestSetInternalLogger_IgnoresNil protects the nil-argument contract.
// A caller passing nil must not clobber the active logger.
func TestSetInternalLogger_IgnoresNil(t *testing.T) {
	before := libLog()
	SetInternalLogger(nil)
	if libLog() != before {
		t.Fatalf("nil logger must be ignored")
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}
