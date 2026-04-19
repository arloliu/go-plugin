// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/yamux"
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

// TestSetInternalLogger_RoutesBrokerErrors proves the routing covers not
// just the stream.go site but the broker sites too: a MuxBroker.Accept
// timeout used to emit via log.Printf and now must land in the installed
// hclog pipeline. If a future refactor misses a site this test catches it.
func TestSetInternalLogger_RoutesBrokerErrors(t *testing.T) {
	var buf syncBuffer
	newLogger := hclog.New(&hclog.LoggerOptions{
		Output: &buf,
		Level:  hclog.Trace,
	})

	prev := libLog()
	SetInternalLogger(newLogger)
	defer SetInternalLogger(prev)

	// Drive MuxBroker.AcceptAndServe with a nil session; it will fail
	// when Accept times out on an unknown ID, which emits through
	// libLog().Error. Using BrokerTimeout shrinking keeps this fast.
	prevTO := BrokerTimeout
	BrokerTimeout = 100 * time.Millisecond
	defer func() { BrokerTimeout = prevTO }()

	a, b := net.Pipe()
	defer func() { _ = a.Close() }()
	defer func() { _ = b.Close() }()
	go func() {
		// accept client connection; do nothing else so Accept on the
		// other side has no matching Dial.
		_, _ = yamux.Server(b, nil)
	}()
	sess, err := yamux.Client(a, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = sess.Close() }()

	br := newMuxBroker(sess)
	go br.Run()

	// Blocks until BrokerTimeout, then logs through libLog().
	br.AcceptAndServe(8888, struct{}{})

	out := buf.String()
	if !strings.Contains(out, "plugin acceptAndServe error") {
		t.Fatalf("expected broker error to reach installed logger; got %q", out)
	}
}

// TestSetInternalLogger_ConcurrentSwapRaceClean runs a swap-and-read
// race under -race to guarantee atomic.Value-backed storage. Before
// this design Hot-swapping the logger from two goroutines would have
// tripped the race detector.
func TestSetInternalLogger_ConcurrentSwapRaceClean(t *testing.T) {
	prev := libLog()
	defer SetInternalLogger(prev)

	l1 := hclog.NewNullLogger()
	l2 := hclog.NewNullLogger()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range 200 {
			SetInternalLogger(l1)
			SetInternalLogger(l2)
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			_ = libLog() // read
		}
	}()
	wg.Wait()
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
