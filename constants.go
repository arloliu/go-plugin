// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import "time"

const (
	// EnvUnixSocketDir specifies the directory that _plugins_ should create unix
	// sockets in. Does not affect client behavior.
	EnvUnixSocketDir = "PLUGIN_UNIX_SOCKET_DIR"

	// EnvUnixSocketGroup specifies the owning, writable group to set for Unix
	// sockets created by _plugins_. Does not affect client behavior.
	EnvUnixSocketGroup = "PLUGIN_UNIX_SOCKET_GROUP"

	envMultiplexGRPC = "PLUGIN_MULTIPLEX_GRPC"
)

// BrokerTimeout bounds the time the broker will wait on a pending
// sub-connection. It covers:
//   - MuxBroker.Accept waiting for the peer to Dial a reserved ID.
//   - MuxBroker.timeoutWait GC of pending entries.
//   - GRPCBroker.Dial waiting for connection info from the peer.
//   - GRPCBroker.knock / knock-ack handshake during multiplexing.
//   - GRPCBroker.timeoutWait GC of pending entries.
//
// The previous hard-coded 5s was adequate for light loads. At fleet scale
// with heavy GC or scheduler pressure, 5s may be tight; integrators can
// bump this at init time. Declared as a var; not a field on ClientConfig
// because the broker is shared across all clients in a process.
var BrokerTimeout = 5 * time.Second
