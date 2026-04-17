// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"strings"
	"testing"
)

// copyStream must never panic on a nil reader or writer. It runs inside
// fire-and-forget goroutines; a panic there would crash the host process.
func TestCopyStream_NilSrcDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("copyStream panicked on nil src: %v", r)
		}
	}()
	var buf bytes.Buffer
	copyStream("stdout", &buf, nil)
}

func TestCopyStream_NilDstDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("copyStream panicked on nil dst: %v", r)
		}
	}()
	copyStream("stdout", nil, strings.NewReader("hello"))
}

func TestCopyStream_CopiesNormally(t *testing.T) {
	var buf bytes.Buffer
	copyStream("stdout", &buf, strings.NewReader("hello world"))
	if buf.String() != "hello world" {
		t.Fatalf("expected copy to succeed, got %q", buf.String())
	}
}
