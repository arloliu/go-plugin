// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// hangingHealthServer blocks every Check call until ctx is cancelled. It
// models a plugin whose gRPC health goroutine is wedged — the exact
// scenario the Ping timeout is there to guard against.
type hangingHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (hangingHealthServer) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

// startHangingGRPCServer returns a dialled client conn to a gRPC server
// whose only Health.Check impl hangs until cancel. Caller must cancel
// and close the returned conn.
func startHangingGRPCServer(t *testing.T) (*grpc.ClientConn, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, hangingHealthServer{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		srv.Stop()
		<-done
		t.Fatalf("dial: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
		<-done
	}
	return conn, cleanup
}

// TestGRPCClient_Ping_TimeoutReturnsError verifies the gRPC Ping path is
// bounded by the package-level defaultPingTimeout when no per-client
// override is set. A hanging Health.Check would otherwise block the
// caller forever.
func TestGRPCClient_Ping_TimeoutReturnsError(t *testing.T) {
	prev := defaultPingTimeout
	defaultPingTimeout = 200 * time.Millisecond
	defer func() { defaultPingTimeout = prev }()

	conn, cleanup := startHangingGRPCServer(t)
	defer cleanup()

	c := &GRPCClient{Conn: conn}

	start := time.Now()
	err := c.Ping()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Ping to return error on hanging health server")
	}
	// gRPC surfaces context-deadline as a status code.
	if code := status.Code(err); code != codes.DeadlineExceeded && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v (code=%v)", err, code)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Ping blocked too long: %v", elapsed)
	}
}

// TestGRPCClient_Ping_ClientConfigOverride verifies a per-client
// PingTimeout beats the package default. Pins the default high and the
// per-client value low; a failing test (not honoring per-client) would
// take the high default.
func TestGRPCClient_Ping_ClientConfigOverride(t *testing.T) {
	prev := defaultPingTimeout
	defaultPingTimeout = 30 * time.Second
	defer func() { defaultPingTimeout = prev }()

	conn, cleanup := startHangingGRPCServer(t)
	defer cleanup()

	c := &GRPCClient{
		Conn:        conn,
		pingTimeout: 200 * time.Millisecond,
	}

	start := time.Now()
	err := c.Ping()
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected Ping to return error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("per-client PingTimeout not respected; elapsed=%v", elapsed)
	}
}

// TestGRPCClient_effectivePingTimeout_ZeroFallsBackToDefault guards the
// zero-value fallback path. Callers that leave ClientConfig.PingTimeout
// unset must get defaultPingTimeout rather than an effectively-instant
// zero-duration timeout.
func TestGRPCClient_effectivePingTimeout_ZeroFallsBackToDefault(t *testing.T) {
	prev := defaultPingTimeout
	defaultPingTimeout = 7 * time.Second
	defer func() { defaultPingTimeout = prev }()

	c := &GRPCClient{pingTimeout: 0}
	if got := c.effectivePingTimeout(); got != 7*time.Second {
		t.Fatalf("zero pingTimeout should fall back to defaultPingTimeout; got %v", got)
	}

	c.pingTimeout = 123 * time.Millisecond
	if got := c.effectivePingTimeout(); got != 123*time.Millisecond {
		t.Fatalf("non-zero pingTimeout not respected; got %v", got)
	}
}
