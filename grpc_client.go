// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/arloliu/go-plugin/internal/plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// defaultPingTimeout bounds how long Ping() blocks before it reports an
// unresponsive plugin. Without a bound, a wedged plugin would hang the
// host supervisor indefinitely. Declared as a var rather than a const so
// the value can be shortened in tests.
var defaultPingTimeout = 10 * time.Second

func dialGRPCConn(tls *tls.Config, dialer func(context.Context, string) (net.Conn, error), dialOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// Build dialing options.
	opts := make([]grpc.DialOption, 0)

	// We use a custom dialer so that we can connect over unix domain sockets.
	opts = append(opts, grpc.WithContextDialer(dialer))

	// Fail right away
	opts = append(opts, grpc.FailOnNonTempDialError(true))

	// If we have no TLS configuration set, we need to explicitly tell grpc
	// that we're connecting with an insecure connection.
	if tls == nil {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(
			credentials.NewTLS(tls)))
	}

	opts = append(opts,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(math.MaxInt32)))

	// Add our custom options if we have any
	opts = append(opts, dialOpts...)

	// Connect. Note the first parameter is unused because we use a custom
	// dialer that has the state to see the address.
	conn, err := grpc.Dial("unused", opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// newGRPCClient creates a new GRPCClient. The Client argument is expected
// to be successfully started already with a lock held.
func newGRPCClient(doneCtx context.Context, c *Client) (*GRPCClient, error) {
	conn, err := dialGRPCConn(c.config.TLSConfig, c.dialer, c.config.GRPCDialOptions...)
	if err != nil {
		return nil, err
	}

	muxer, err := c.getGRPCMuxer(c.address)
	if err != nil {
		return nil, err
	}

	// Start the broker.
	brokerGRPCClient := newGRPCBrokerClient(conn)
	broker := newGRPCBroker(brokerGRPCClient, c.config.TLSConfig, c.unixSocketCfg, c.runner, muxer)
	go broker.Run()
	go func() { _ = brokerGRPCClient.StartStream() }()

	// Start the stdio client
	stdioClient, err := newGRPCStdioClient(doneCtx, c.logger.Named("stdio"), conn)
	if err != nil {
		return nil, err
	}
	go stdioClient.Run(c.config.SyncStdout, c.config.SyncStderr)

	cl := &GRPCClient{
		Conn:        conn,
		Plugins:     c.config.Plugins,
		doneCtx:     doneCtx,
		broker:      broker,
		controller:  plugin.NewGRPCControllerClient(conn),
		pingTimeout: c.config.PingTimeout,
	}

	return cl, nil
}

// GRPCClient connects to a GRPCServer over gRPC to dispense plugin types.
type GRPCClient struct {
	Conn    *grpc.ClientConn
	Plugins map[string]Plugin

	doneCtx context.Context
	broker  *GRPCBroker

	controller plugin.GRPCControllerClient

	// pingTimeout bounds Ping() and the Close-time Shutdown RPC. Zero means
	// fall back to defaultPingTimeout.
	pingTimeout time.Duration
}

// Close implements ClientProtocol.
func (c *GRPCClient) Close() error {
	_ = c.broker.Close()
	// Shutdown is best-effort: the RPC commonly returns "Unavailable" as
	// the plugin's gRPC server tears itself down in response, which is the
	// expected behaviour and should not make Close() look like a failure.
	// We do, however, bound the call with a timeout so a wedged plugin
	// cannot hang the caller indefinitely — c.doneCtx alone was unbounded
	// until plugin exit.
	ctx, cancel := context.WithTimeout(context.Background(), c.effectivePingTimeout())
	defer cancel()
	_, _ = c.controller.Shutdown(ctx, &plugin.Empty{})
	return c.Conn.Close()
}

func (c *GRPCClient) effectivePingTimeout() time.Duration {
	if c.pingTimeout > 0 {
		return c.pingTimeout
	}
	return defaultPingTimeout
}

// Dispense implements ClientProtocol.
func (c *GRPCClient) Dispense(name string) (any, error) {
	raw, ok := c.Plugins[name]
	if !ok {
		return nil, fmt.Errorf("unknown plugin type: %s", name)
	}

	p, ok := raw.(GRPCPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q doesn't support gRPC", name)
	}

	return p.GRPCClient(c.doneCtx, c.broker, c.Conn)
}

// Ping implements ClientProtocol.
func (c *GRPCClient) Ping() error {
	client := grpc_health_v1.NewHealthClient(c.Conn)
	ctx, cancel := context.WithTimeout(context.Background(), c.effectivePingTimeout())
	defer cancel()
	_, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: GRPCServiceName,
	})

	return err
}
