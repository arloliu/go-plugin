// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"net"
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestClient_getGRPCMuxer_FailurePersists makes the first Dial fail and
// verifies the error is returned on every subsequent call, not just the
// first. Before the fix, sync.Once silently succeeded on later calls and
// the dialer fell back to the non-muxed path without surfacing an error.
func TestClient_getGRPCMuxer_FailurePersists(t *testing.T) {
	// Address that cannot be dialled: reserved 0.0.0.0:1 will reject.
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}

	c := &Client{
		logger: hclog.NewNullLogger(),
		config: &ClientConfig{
			GRPCBrokerMultiplex: true,
		},
		protocol: ProtocolGRPC,
	}

	_, err1 := c.getGRPCMuxer(addr)
	if err1 == nil {
		t.Fatalf("expected first getGRPCMuxer call to fail when dialling %v", addr)
	}

	_, err2 := c.getGRPCMuxer(addr)
	if err2 == nil {
		t.Fatalf("second getGRPCMuxer call returned nil; init failure must be persisted")
	}
	if err1.Error() != err2.Error() {
		t.Fatalf("expected persistent error; got first=%q second=%q", err1, err2)
	}
}
