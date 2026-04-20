// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

// TestBrokerTimeout_MuxBrokerAcceptRespectsVar verifies MuxBroker.Accept
// honours the plugin.BrokerTimeout package var. Before fc3913e the 5s
// timeout was hard-coded; mutating the var proves the value is read at
// call time. We drive MuxBroker.Accept against an ID no peer will ever
// Dial, shorten BrokerTimeout, and assert the call returns within the
// configured window.
func TestBrokerTimeout_MuxBrokerAcceptRespectsVar(t *testing.T) {
	prev := BrokerTimeout
	BrokerTimeout = 200 * time.Millisecond
	defer func() { BrokerTimeout = prev }()

	// Pair of pipes so we can create a yamux session without a real
	// network round-trip.
	aConn, bConn := net.Pipe()
	defer func() { _ = aConn.Close() }()
	defer func() { _ = bConn.Close() }()

	// Server side of yamux runs on bConn; MuxBroker wraps the session.
	go func() {
		srv, err := yamux.Server(bConn, nil)
		if err != nil {
			return
		}
		// Accept and hold; never write the matching ID, so Accept on the
		// other side has to time out.
		for {
			s, err := srv.AcceptStream()
			if err != nil {
				return
			}
			_ = s // leak until pipe closes
		}
	}()

	cliSess, err := yamux.Client(aConn, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = cliSess.Close() }()

	broker := newMuxBroker(cliSess)
	go broker.Run()

	start := time.Now()
	_, err = broker.Accept(9999) // no peer will ever Dial this ID.
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected Accept to time out")
	}
	if !errors.Is(err, ErrBrokerTimeout) {
		t.Fatalf("expected errors.Is(err, ErrBrokerTimeout); got %v", err)
	}
	// Allow generous slack above the configured window; the literal
	// 5s we replaced would make this fail loudly.
	if elapsed > 2*time.Second {
		t.Fatalf("BrokerTimeout override ignored; elapsed=%v", elapsed)
	}
}
