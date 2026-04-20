// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"io"
	"sync"
	"testing"

	"github.com/arloliu/go-plugin/internal/plugin"
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
)

// fakeStdioClient implements plugin.GRPCStdio_StreamStdioClient by
// embedding the interface (nil) — only Recv is exercised by Run.
type fakeStdioClient struct {
	grpc.ClientStream
	chunks []*plugin.StdioData
	idx    int
}

func (f *fakeStdioClient) Recv() (*plugin.StdioData, error) {
	if f.idx >= len(f.chunks) {
		return nil, io.EOF
	}
	d := f.chunks[f.idx]
	f.idx++
	return d, nil
}

// TestGRPCStdioClient_RunRecoversWriterPanic verifies that a panicking
// SyncStdout writer cannot crash the host: Run must recover and return.
// Prior to the fix, io.Copy into a panicking writer propagated the panic
// out of the forwarder goroutine.
func TestGRPCStdioClient_RunRecoversWriterPanic(t *testing.T) {
	t.Parallel()

	fake := &fakeStdioClient{chunks: []*plugin.StdioData{{
		Channel: plugin.StdioData_STDOUT,
		Data:    []byte("hello"),
	}}}
	c := &grpcStdioClient{
		log:         hclog.NewNullLogger(),
		stdioClient: fake,
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		c.Run(panicWriter{}, io.Discard)
	}()

	wg.Wait()
	select {
	case <-done:
	default:
		t.Fatal("Run did not return after writer panic")
	}
}
