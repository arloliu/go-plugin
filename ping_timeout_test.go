// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"net"
	"net/rpc"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

// TestRPCClient_PingTimeout verifies Ping() returns ErrPingTimeout when the
// control-side RPC server never responds. Without the timeout, a wedged
// plugin would hang the caller forever.
func TestRPCClient_PingTimeout(t *testing.T) {
	// Shorten the timeout for a fast test. Package-level var set in
	// grpc_client.go; shared between both protocol paths. Restore after.
	prev := defaultPingTimeout
	defaultPingTimeout = 200 * time.Millisecond
	defer func() { defaultPingTimeout = prev }()

	clientConn, serverConn := net.Pipe()

	// Start a yamux server that accepts streams but never responds to
	// any RPC calls. This simulates a plugin whose control loop is
	// wedged.
	srvDone := make(chan struct{})
	go func() {
		defer close(srvDone)
		sess, err := yamux.Server(serverConn, nil)
		if err != nil {
			return
		}
		defer func() { _ = sess.Close() }()
		for {
			stream, err := sess.AcceptStream()
			if err != nil {
				return
			}
			// Hold the stream open, consume bytes silently, never write
			// back. This models a control goroutine that is alive enough
			// to keep the stream open but not to serve requests.
			go func(s net.Conn) {
				buf := make([]byte, 1024)
				for {
					if _, err := s.Read(buf); err != nil {
						return
					}
				}
			}(stream)
		}
	}()

	mux, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = mux.Close() }()

	ctrl, err := mux.Open()
	if err != nil {
		t.Fatalf("mux.Open: %v", err)
	}

	c := &RPCClient{
		control: rpc.NewClient(ctrl),
	}

	start := time.Now()
	err = c.Ping()
	elapsed := time.Since(start)

	if !errors.Is(err, ErrPingTimeout) {
		t.Fatalf("expected ErrPingTimeout, got %v", err)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("Ping blocked too long: %v", elapsed)
	}

	// Cleanup.
	_ = mux.Close()
	_ = serverConn.Close()
	select {
	case <-srvDone:
	case <-time.After(2 * time.Second):
		t.Log("server goroutine did not exit; leaking for test teardown")
	}
}

// TestRPCClient_PingTimeout_ClientConfigOverride verifies that a per-client
// PingTimeout set via ClientConfig is honoured even when the package-level
// default is long. The supervisor use-case is a very short ping budget
// (e.g. 500ms) for specific plugins without changing the global default.
func TestRPCClient_PingTimeout_ClientConfigOverride(t *testing.T) {
	// Keep the package default generous so we can tell the per-client
	// value is the one being respected.
	prev := defaultPingTimeout
	defaultPingTimeout = 30 * time.Second
	defer func() { defaultPingTimeout = prev }()

	clientConn, serverConn := net.Pipe()
	srvDone := make(chan struct{})
	go func() {
		defer close(srvDone)
		sess, err := yamux.Server(serverConn, nil)
		if err != nil {
			return
		}
		defer func() { _ = sess.Close() }()
		for {
			stream, err := sess.AcceptStream()
			if err != nil {
				return
			}
			go func(s net.Conn) {
				buf := make([]byte, 1024)
				for {
					if _, err := s.Read(buf); err != nil {
						return
					}
				}
			}(stream)
		}
	}()

	mux, err := yamux.Client(clientConn, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = mux.Close() }()

	ctrl, err := mux.Open()
	if err != nil {
		t.Fatalf("mux.Open: %v", err)
	}

	c := &RPCClient{
		control:     rpc.NewClient(ctrl),
		pingTimeout: 200 * time.Millisecond,
	}

	start := time.Now()
	err = c.Ping()
	elapsed := time.Since(start)

	if !errors.Is(err, ErrPingTimeout) {
		t.Fatalf("expected ErrPingTimeout, got %v", err)
	}
	// Per-client 200ms must have won; a failing test would take the
	// package-level 30s.
	if elapsed > 2*time.Second {
		t.Fatalf("per-client PingTimeout not respected; elapsed=%v", elapsed)
	}

	_ = mux.Close()
	_ = serverConn.Close()
	<-srvDone
}
