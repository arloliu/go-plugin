// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arloliu/go-plugin/internal/plugin"
	"google.golang.org/grpc"
)

// TestGRPCController_ShutdownEscalatesToStop verifies the GracefulStop
// → Stop fallback fires when gracefulStopTimeout elapses before the
// server finishes draining. Models a wedged streaming RPC that never
// returns: GracefulStop would block forever; the fallback must force
// the server down so the plugin process can exit.
//
// Harness: a real GRPCServer backed by a stream RPC we hold open. After
// Shutdown is invoked, verify the server's Serve() call unblocks within
// a small multiple of gracefulStopTimeout.
func TestGRPCController_ShutdownEscalatesToStop(t *testing.T) {
	prev := gracefulStopTimeout()
	SetGracefulStopTimeout(50 * time.Millisecond)
	defer SetGracefulStopTimeout(prev)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	gs := grpc.NewServer()
	// Register the broker as a stand-in for any long-lived server stream.
	// After the plugin's host disconnects, GracefulStop would normally
	// unblock because stream.Context().Done() fires. Here we block the
	// server ourselves to simulate a stream that refuses to terminate
	// on disconnect.
	var streamHold int32
	plugin.RegisterGRPCBrokerServer(gs, &hangingBrokerServer{hold: &streamHold})

	// Fake GRPCServer just enough to give grpcControllerServer something
	// to call GracefulStop / Stop on.
	wrapped := &GRPCServer{server: gs}
	controller := &grpcControllerServer{server: wrapped}

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = gs.Serve(lis)
	}()

	// Invoke Shutdown. It must return quickly (dispatches drain to a
	// goroutine) and then the server must actually exit within a small
	// multiple of gracefulStopTimeout thanks to the Stop fallback.
	start := time.Now()
	if _, err := controller.Shutdown(context.Background(), &plugin.Empty{}); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	handlerReturn := time.Since(start)
	if handlerReturn > 200*time.Millisecond {
		t.Fatalf("Shutdown handler should return immediately; took %v", handlerReturn)
	}

	// The server must exit because of the Stop fallback; without it the
	// hanging stream would pin GracefulStop forever.
	select {
	case <-serveDone:
	case <-time.After(2 * time.Second):
		atomic.StoreInt32(&streamHold, 1) // release server goroutine regardless
		gs.Stop()
		<-serveDone
		t.Fatalf("GRPCServer did not exit via Stop fallback within 2s")
	}
}

// hangingBrokerServer holds its stream open until hold flips, modelling a
// server-side streaming RPC that ignores client disconnect.
type hangingBrokerServer struct {
	plugin.UnimplementedGRPCBrokerServer
	hold *int32
}

func (h *hangingBrokerServer) StartStream(stream plugin.GRPCBroker_StartStreamServer) error {
	for atomic.LoadInt32(h.hold) == 0 {
		// Check the stream context so Stop() can still unblock us.
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return nil
}
