// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"io"
)

func copyStream(name string, dst io.Writer, src io.Reader) {
	// A misconfigured caller that passes a nil reader or writer should not
	// crash the host process — just log and return. This runs inside
	// fire-and-forget goroutines with no caller to observe a panic.
	if src == nil {
		libLog().Error("stream copy aborted: src is nil", "stream", name)
		return
	}
	if dst == nil {
		libLog().Error("stream copy aborted: dst is nil", "stream", name)
		return
	}
	if _, err := io.Copy(dst, src); err != nil && err != io.EOF {
		libLog().Error("stream copy error", "stream", name, "error", err)
	}
}
