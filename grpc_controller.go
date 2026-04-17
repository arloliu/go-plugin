// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"time"

	"github.com/hashicorp/go-plugin/internal/plugin"
)

// GRPCControllerServer handles shutdown calls to terminate the server when the
// plugin client is closed.
type grpcControllerServer struct {
	server *GRPCServer
}

// gracefulStopTimeout bounds how long the server will wait for in-flight
// RPCs to drain before escalating to a hard Stop. Client.Kill gives the
// plugin a 2s window to exit, and CmdAttachedRunner.Wait polls liveness
// on a 1s tick, so the exit-notification budget is ~1s. A tight default
// here (100ms) lets fast RPCs finish returning while still leaving the
// bulk of the exit budget for the pidWait poll to notice the exit.
// Declared as a var so host integrators who set a longer Kill grace can
// also widen this.
var gracefulStopTimeout = 100 * time.Millisecond

// Shutdown stops the grpc server. It prefers a graceful stop so that
// in-flight RPCs (for example a long equipment command mid-flight) can
// finish cleanly, with a bounded fallback to a hard Stop if graceful
// draining doesn't complete within gracefulStopTimeout.
//
// GracefulStop cannot be invoked synchronously from a gRPC handler: it
// waits for all RPCs — including this one — to return, which would
// deadlock. Run it in a goroutine and let the handler return immediately.
// The server's Serve() goroutine closes DoneCh when the underlying
// grpc.Server exits, which is what allows the plugin process to end.
func (s *grpcControllerServer) Shutdown(ctx context.Context, _ *plugin.Empty) (*plugin.Empty, error) {
	resp := &plugin.Empty{}

	go func() {
		stopped := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(gracefulStopTimeout):
			// Graceful drain exceeded its budget — force the server
			// down so the plugin process can exit and the host's
			// supervisor can move on.
			s.server.Stop()
			<-stopped
		}
	}()

	return resp, nil
}
