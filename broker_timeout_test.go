// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/arloliu/go-plugin/internal/grpcmux"
	iplugin "github.com/arloliu/go-plugin/internal/plugin"
	"github.com/hashicorp/yamux"
)

// blockingStreamer is a test streamer that never delivers messages. Used
// to drive GRPCBroker.Dial into its timeout branch without a real peer.
type blockingStreamer struct {
	done chan struct{}
}

func (s *blockingStreamer) Send(*iplugin.ConnInfo) error { return nil }
func (s *blockingStreamer) Recv() (*iplugin.ConnInfo, error) {
	<-s.done
	return nil, errors.New("streamer closed")
}
func (s *blockingStreamer) Close() { close(s.done) }

// disabledMuxer reports Enabled() == false so GRPCBroker.Dial takes the
// non-muxed code path. The embedded interface supplies the other methods;
// they are never called.
type disabledMuxer struct {
	grpcmux.GRPCMuxer
}

func (disabledMuxer) Enabled() bool { return false }

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

// TestBrokerTimeout_DialReapsClientStream verifies GRPCBroker.Dial
// reaps its pending clientStreams entry when BrokerTimeout fires. Before
// the fix, a Dial(id) that never received a ConnInfo left the map entry
// in place for the life of the broker — a slow leak for long-running
// hosts with misbehaving plugins.
func TestBrokerTimeout_DialReapsClientStream(t *testing.T) {
	prev := BrokerTimeout
	BrokerTimeout = 100 * time.Millisecond
	defer func() { BrokerTimeout = prev }()

	s := &blockingStreamer{done: make(chan struct{})}
	defer s.Close()

	b := newGRPCBroker(s, nil, UnixSocketConfig{}, nil, disabledMuxer{})

	_, err := b.Dial(9999)
	if err == nil {
		t.Fatal("expected Dial to time out")
	}
	if !errors.Is(err, ErrBrokerTimeout) {
		t.Fatalf("expected ErrBrokerTimeout, got %v", err)
	}

	b.Lock()
	defer b.Unlock()
	if _, ok := b.clientStreams[9999]; ok {
		t.Fatal("clientStreams entry not reaped after Dial timeout")
	}
}
