// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"encoding/binary"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arloliu/go-plugin/internal/grpcmux"
	iplugin "github.com/arloliu/go-plugin/internal/plugin"
	hclog "github.com/hashicorp/go-hclog"
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

// TestMuxBrokerRun_ClosesOverflowStream verifies that Run closes an extra
// stream immediately when a pending ID already has an unread stream queued.
// Prior to the fix, Run spawned timeoutWait even when the buffer was full and
// left the extra stream hanging until timeout instead of rejecting it promptly.
func TestMuxBrokerRun_ClosesOverflowStream(t *testing.T) {
	aConn, bConn := net.Pipe()
	defer func() { _ = aConn.Close() }()
	defer func() { _ = bConn.Close() }()

	srvSess, err := yamux.Server(bConn, nil)
	if err != nil {
		t.Fatalf("yamux.Server: %v", err)
	}
	defer func() { _ = srvSess.Close() }()

	cliSess, err := yamux.Client(aConn, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = cliSess.Close() }()

	broker := newMuxBroker(cliSess)
	go broker.Run()

	const id = 777

	openTaggedStream := func() net.Conn {
		t.Helper()
		s, err := srvSess.OpenStream()
		if err != nil {
			t.Fatalf("OpenStream: %v", err)
		}
		if err := binary.Write(s, binary.LittleEndian, uint32(id)); err != nil {
			_ = s.Close()
			t.Fatalf("write stream id: %v", err)
		}
		return s
	}

	first := openTaggedStream()
	defer func() { _ = first.Close() }()
	second := openTaggedStream()
	defer func() { _ = second.Close() }()

	// The second stream should be closed promptly because the first occupies
	// the single-slot buffer and no Accept has drained it yet.
	if err := second.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var oneByte [1]byte
	_, err = second.Read(oneByte[:])
	if err == nil {
		t.Fatal("expected overflow stream to be closed")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("overflow stream was not closed promptly: %v", err)
	}

	// The first stream must remain usable and be acknowledged normally.
	accepted, err := broker.Accept(id)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	defer func() { _ = accepted.Close() }()

	if err := first.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var ack uint32
	if err := binary.Read(first, binary.LittleEndian, &ack); err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack != id {
		t.Fatalf("ack = %d; want %d", ack, id)
	}
}

// scriptedStreamer replays a fixed sequence of ConnInfo messages to Recv
// and then blocks until Close. calls reports how many Recv invocations
// have completed, which tests use to wait for Run to drain the script
// instead of sleeping blindly.
type scriptedStreamer struct {
	msgs  []*iplugin.ConnInfo
	calls atomic.Int32
	done  chan struct{}
	once  sync.Once
}

func newScriptedStreamer(msgs ...*iplugin.ConnInfo) *scriptedStreamer {
	return &scriptedStreamer{msgs: msgs, done: make(chan struct{})}
}

func (s *scriptedStreamer) Send(*iplugin.ConnInfo) error { return nil }

func (s *scriptedStreamer) Recv() (*iplugin.ConnInfo, error) {
	i := int(s.calls.Add(1) - 1)
	if i < len(s.msgs) {
		return s.msgs[i], nil
	}
	<-s.done
	return nil, errors.New("scripted streamer closed")
}

func (s *scriptedStreamer) Close() { s.once.Do(func() { close(s.done) }) }

// TestGRPCBrokerRun_DropsOverflowMessage verifies GRPCBroker.Run drops a
// duplicate message for an id whose single-slot buffer is already full,
// emits a Warn log for the drop, and that a subsequent message for a
// different id still routes normally. Prior to the fix the overflow was
// silent and Run spawned a redundant timeoutWait for the dropped message.
// The log assertion is the primary regression signal: pre-fix emitted
// nothing, so a future refactor that silently drops again would miss the
// warn line.
func TestGRPCBrokerRun_DropsOverflowMessage(t *testing.T) {
	var buf syncBuffer
	testLogger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Warn})
	prevLogger := libLog()
	SetInternalLogger(testLogger)
	defer SetInternalLogger(prevLogger)

	s := newScriptedStreamer(
		&iplugin.ConnInfo{ServiceId: 42},
		&iplugin.ConnInfo{ServiceId: 42},
		&iplugin.ConnInfo{ServiceId: 99},
	)
	defer s.Close()

	b := newGRPCBroker(s, nil, UnixSocketConfig{}, nil, disabledMuxer{})
	go b.Run()

	// Wait until Run has consumed all scripted messages and is blocked
	// on the fourth Recv() call. At that point both messages for id=42
	// have traversed Run's select, so the overflow branch has fired.
	deadline := time.Now().Add(time.Second)
	for int(s.calls.Load()) < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("broker.Run did not drain scripted messages; calls=%d", s.calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}

	b.Lock()
	p42, ok42 := b.clientStreams[42]
	p99, ok99 := b.clientStreams[99]
	b.Unlock()
	if !ok42 {
		t.Fatal("clientStreams entry missing for id=42")
	}
	if !ok99 {
		t.Fatal("clientStreams entry missing for id=99; later messages should still route")
	}

	// The first message for id=42 must be queued; the second must have
	// been dropped (not double-queued) so the second receive does not
	// succeed.
	select {
	case msg := <-p42.ch:
		if msg.ServiceId != 42 {
			t.Fatalf("unexpected service id in first message: %d", msg.ServiceId)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first message for id=42 not queued")
	}
	select {
	case msg := <-p42.ch:
		t.Fatalf("overflow message for id=42 was not dropped: %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	// id=99 is an independent pending slot; its message must be there.
	select {
	case msg := <-p99.ch:
		if msg.ServiceId != 99 {
			t.Fatalf("unexpected service id: %d", msg.ServiceId)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("message for id=99 not queued")
	}

	// The overflow branch must emit a Warn line so operators can
	// correlate a silently missing Dial to the plugin-protocol anomaly.
	out := buf.String()
	if !strings.Contains(out, "grpc broker: dropping duplicate message") {
		t.Fatalf("expected overflow warn log; got: %q", out)
	}
	if !strings.Contains(out, "service_id=42") {
		t.Fatalf("expected service_id=42 in log line; got: %q", out)
	}
}

// TestGRPCBrokerRun_DropsOverflowServerKnock covers the serverKnock
// branch of Run, which is structurally parallel to the client-stream
// case but routes to a separate map (serverStreams), is kept alive for
// the life of the broker (no timeoutWait reaps it), and is the path a
// multiplexed listener receives knocks on. The overflow contract is the
// same: the duplicate knock must be dropped and logged with
// server_knock=true, and the first knock must remain queued for the
// listener to consume.
func TestGRPCBrokerRun_DropsOverflowServerKnock(t *testing.T) {
	var buf syncBuffer
	testLogger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Warn})
	prevLogger := libLog()
	SetInternalLogger(testLogger)
	defer SetInternalLogger(prevLogger)

	// Knock=true Ack=false → serverKnock branch.
	s := newScriptedStreamer(
		&iplugin.ConnInfo{ServiceId: 42, Knock: &iplugin.ConnInfo_Knock{Knock: true}},
		&iplugin.ConnInfo{ServiceId: 42, Knock: &iplugin.ConnInfo_Knock{Knock: true}},
	)
	defer s.Close()

	b := newGRPCBroker(s, nil, UnixSocketConfig{}, nil, disabledMuxer{})
	go b.Run()

	// Wait until both scripted knocks have been consumed and Run is
	// blocked on the third Recv().
	deadline := time.Now().Add(time.Second)
	for int(s.calls.Load()) < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("broker.Run did not drain scripted knocks; calls=%d", s.calls.Load())
		}
		time.Sleep(5 * time.Millisecond)
	}

	b.Lock()
	p, ok := b.serverStreams[42]
	_, inClients := b.clientStreams[42]
	b.Unlock()
	if !ok {
		t.Fatal("serverStreams entry missing for id=42")
	}
	if inClients {
		t.Fatal("server knock must not land in clientStreams")
	}

	// First knock must be queued; second must have been dropped.
	select {
	case msg := <-p.ch:
		if msg.Knock == nil || !msg.Knock.Knock || msg.Knock.Ack {
			t.Fatalf("unexpected first message: %+v", msg)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first knock not queued")
	}
	select {
	case msg := <-p.ch:
		t.Fatalf("overflow knock was not dropped: %+v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	out := buf.String()
	if !strings.Contains(out, "grpc broker: dropping duplicate message") {
		t.Fatalf("expected overflow warn log; got: %q", out)
	}
	if !strings.Contains(out, "server_knock=true") {
		t.Fatalf("expected server_knock=true in log line; got: %q", out)
	}
}

// TestMuxBrokerRun_LogsOverflow mirrors the GRPCBroker assertion for
// MuxBroker: the buffer-full branch must emit a Warn log identifying
// the dropped stream id. Pre-fix this branch was silent.
func TestMuxBrokerRun_LogsOverflow(t *testing.T) {
	var buf syncBuffer
	testLogger := hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Warn})
	prevLogger := libLog()
	SetInternalLogger(testLogger)
	defer SetInternalLogger(prevLogger)

	aConn, bConn := net.Pipe()
	defer func() { _ = aConn.Close() }()
	defer func() { _ = bConn.Close() }()

	srvSess, err := yamux.Server(bConn, nil)
	if err != nil {
		t.Fatalf("yamux.Server: %v", err)
	}
	defer func() { _ = srvSess.Close() }()

	cliSess, err := yamux.Client(aConn, nil)
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer func() { _ = cliSess.Close() }()

	broker := newMuxBroker(cliSess)
	go broker.Run()

	const id = 555
	open := func() net.Conn {
		t.Helper()
		s, err := srvSess.OpenStream()
		if err != nil {
			t.Fatalf("OpenStream: %v", err)
		}
		if err := binary.Write(s, binary.LittleEndian, uint32(id)); err != nil {
			_ = s.Close()
			t.Fatalf("write id: %v", err)
		}
		return s
	}

	first := open()
	defer func() { _ = first.Close() }()
	second := open()
	defer func() { _ = second.Close() }()

	// Let Run process both streams. Reading second.Read blocks until the
	// broker closes it, giving us a deterministic sync point for the log
	// emission that happened in the same iteration.
	_ = second.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var oneByte [1]byte
	_, _ = second.Read(oneByte[:])

	out := buf.String()
	if !strings.Contains(out, "mux broker: dropped duplicate stream") {
		t.Fatalf("expected overflow warn log; got: %q", out)
	}
	if !strings.Contains(out, "id=555") {
		t.Fatalf("expected id=555 in log line; got: %q", out)
	}
}
