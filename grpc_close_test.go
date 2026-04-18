// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/arloliu/go-plugin/internal/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// noopStreamer satisfies the streamer interface used by GRPCBroker.
// Recv blocks forever; Send errors; Close is a no-op. Sufficient for
// wiring a broker into test code that only invokes broker.Close.
type noopStreamer struct{ done chan struct{} }

func (n *noopStreamer) Send(*plugin.ConnInfo) error { return context.Canceled }
func (n *noopStreamer) Recv() (*plugin.ConnInfo, error) {
	<-n.done
	return nil, context.Canceled
}
func (n *noopStreamer) Close() { close(n.done) }

// hangingControllerServer blocks every Shutdown call until ctx is
// cancelled. It models a plugin whose controller RPC handler is wedged.
type hangingControllerServer struct {
	plugin.UnimplementedGRPCControllerServer
}

func (hangingControllerServer) Shutdown(ctx context.Context, _ *plugin.Empty) (*plugin.Empty, error) {
	<-ctx.Done()
	return &plugin.Empty{}, ctx.Err()
}

// TestGRPCClient_Close_BoundedByPingTimeout verifies that a wedged
// Shutdown RPC cannot hang Close() forever — the Shutdown call is
// bounded by effectivePingTimeout. Before 593e46c the call used
// c.doneCtx, which is unbounded until the plugin exits; a wedged
// plugin would make every Kill hang.
func TestGRPCClient_Close_BoundedByPingTimeout(t *testing.T) {
	prev := defaultPingTimeout
	defaultPingTimeout = 300 * time.Millisecond
	defer func() { defaultPingTimeout = prev }()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	plugin.RegisterGRPCControllerServer(srv, hangingControllerServer{})

	srvDone := make(chan struct{})
	go func() { defer close(srvDone); _ = srv.Serve(lis) }()

	conn, err := grpc.Dial(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		srv.Stop()
		<-srvDone
		t.Fatalf("dial: %v", err)
	}

	// Give Close the pieces it expects: a broker whose Close works and
	// a real-but-hanging controller.
	c := &GRPCClient{
		Conn:    conn,
		doneCtx: t.Context(),
		broker: &GRPCBroker{
			streamer: &noopStreamer{done: make(chan struct{})},
			doneCh:   make(chan struct{}),
		},
		controller: plugin.NewGRPCControllerClient(conn),
	}

	start := time.Now()
	_ = c.Close()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("Close blocked past the bounded timeout: %v", elapsed)
	}

	srv.Stop()
	<-srvDone
}
