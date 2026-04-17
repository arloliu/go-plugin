// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"sync/atomic"

	hclog "github.com/hashicorp/go-hclog"
)

// internalLogger holds the hclog.Logger used by library-internal code
// paths that do not have access to a per-Client logger — the brokers,
// RPCServer, and the stream copy helper. Previously these sites used
// stdlib log.Printf, which bypassed the host's structured log pipeline
// and showed up as unstructured lines on stderr.
//
// Stored behind atomic.Value so SetInternalLogger is safe to call from
// any goroutine, and cheap to Load from hot paths.
var internalLogger atomic.Value

func init() {
	internalLogger.Store(hclogWrapper{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "go-plugin",
			Output: hclog.DefaultOutput,
			Level:  hclog.Info,
		}),
	})
}

// hclogWrapper exists solely so atomic.Value always stores the same
// concrete type (atomic.Value panics on mixed types).
type hclogWrapper struct{ logger hclog.Logger }

// SetInternalLogger installs the hclog.Logger used for library-internal
// diagnostics emitted by the broker, RPCServer, and stream copy paths.
// Call at process init to route these into your host's structured log
// pipeline instead of the stdlib log.Printf default. A nil argument is
// ignored; the previous logger is retained.
func SetInternalLogger(l hclog.Logger) {
	if l == nil {
		return
	}
	internalLogger.Store(hclogWrapper{logger: l})
}

// libLog returns the current internal logger. Kept terse because it's
// on the hot path of every error log emitted by the brokers.
func libLog() hclog.Logger {
	return internalLogger.Load().(hclogWrapper).logger
}
