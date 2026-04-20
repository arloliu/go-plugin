# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `plugin`: New exported `ErrBrokerTimeout` sentinel. Broker accept, dial,
  and knock timeouts now wrap it via `%w`, so callers can distinguish a
  broker timeout from other transport errors via `errors.Is`.

### Changed

- `client`: `Client.Kill` is now documented and guaranteed safe under
  concurrent callers; the shutdown body runs at most once even when many
  goroutines race.

### Fixed

- `client`: `Client.Kill` is now gated by `sync.Once` so concurrent callers
  no longer both enter the graceful-shutdown path and double-close the RPC
  client. Previously the lock was released before the shutdown body ran,
  producing spurious "connection is shut down" errors and redundant
  force-kills under racing callers.
- `client`: A non-test reattach now adopts `ReattachConfig.ProtocolVersion`
  as the negotiated version. Previously `NegotiatedVersion()` returned `0`
  after reattach, which could silently mismatch `VersionedPlugins` after a
  plugin rebuild.
- `plugin`: Panics in host-provided `SyncStdout` / `SyncStderr` writers are
  now recovered in the gRPC stdio forwarder goroutine instead of taking
  down the host process.

## [1.8.0] - 2026-04-19

### Added

- `client`: New `ClientConfig.ShutdownTimeout` to bound the grace window on
  `Kill` before the plugin process is force-terminated.
- `client`: New `ClientConfig.PingTimeout` to bound health-check `Ping()`
  calls so wedged plugins cannot hang the host.
- `client`: New `ClientConfig.DisableProcessGroupKill` opt-out for TTY
  hosts that need the legacy single-process kill behaviour.
- `plugin`: New exported `BrokerTimeout` variable for tuning broker accept
  and dial timeouts.

### Changed

- Module path renamed from `github.com/hashicorp/go-plugin` to
  `github.com/arloliu/go-plugin`.
- `client`: `GRPCClient.Close` now bounds the underlying `Shutdown` with a
  timeout, and the gRPC controller prefers `GracefulStop` over `Stop`.
- `cmdrunner`: On POSIX, the plugin's entire process group is killed on
  shutdown to avoid orphaned children.
- `plugin`: Library-internal `log.Printf` output is now routed through
  `hclog` instead of the default logger.

### Fixed

- `client`: Managed clients are now removed from the global slice on
  `Kill`, preventing a slow leak across plugin restarts.
- `plugin`: `getGRPCMuxer` init errors are persisted across calls rather
  than silently retried.
- `plugin`: Panics in the stderr log pump and stdout scanner goroutines
  are now recovered instead of taking down the host.
- `plugin`: Malformed plugin handshake output no longer panics the host
  process.

## [1.7.0] - 2025-08-11

### Changed

- When go-plugin encounters a stack trace on the server stderr stream, it
  now raises output to a log level of Error instead of Debug.
  [[GH-292](https://github.com/arloliu/go-plugin/pull/292)]
- Don't spend resources parsing log lines when logging is disabled.
  [[GH-352](https://github.com/arloliu/go-plugin/pull/352)]

## [1.6.2] - 2024-10-21

### Added

- Added support for gRPC dial options to the `Dial` API.
  [[GH-257](https://github.com/arloliu/go-plugin/pull/257)]

### Fixed

- Fixed a bug where reattaching to a plugin that exits could kill an
  unrelated process.
  [[GH-320](https://github.com/arloliu/go-plugin/pull/320)]

## [1.6.1] - 2024-05-10

### Changed

- `deps`: bump `google.golang.org/grpc` to v1.58.3.
  [[GH-296](https://github.com/arloliu/go-plugin/pull/296)]

### Fixed

- Suppress spurious `os.ErrClosed` on plugin shutdown.
  [[GH-299](https://github.com/arloliu/go-plugin/pull/299)]

## [1.6.0] - 2023-11-13

### Added

- Support muxing gRPC broker connections over a single listener.
  [[GH-288](https://github.com/arloliu/go-plugin/pull/288)]
- `client`: Configurable buffer size for reading plugin log lines.
  [[GH-265](https://github.com/arloliu/go-plugin/pull/265)]

### Changed

- `plugin`: Plugins written in other languages can optionally start to
  advertise whether they support gRPC broker multiplexing. If the
  environment variable `PLUGIN_MULTIPLEX_GRPC` is set, it is safe to
  include a seventh field containing a boolean value in the `|`-separated
  protocol negotiation line.
- Use `buf` for proto generation.
  [[GH-286](https://github.com/arloliu/go-plugin/pull/286)]
- `deps`: bump `golang.org/x/net` to v0.17.0.
  [[GH-285](https://github.com/arloliu/go-plugin/pull/285)]
- `deps`: bump `golang.org/x/sys` to v0.13.0.
  [[GH-285](https://github.com/arloliu/go-plugin/pull/285)]
- `deps`: bump `golang.org/x/text` to v0.13.0.
  [[GH-285](https://github.com/arloliu/go-plugin/pull/285)]

## [1.5.2] - 2023-09-22

### Added

- `client`: New `UnixSocketConfig.TempDir` option allows setting the
  directory to use when creating plugin-specific Unix socket directories.
  [[GH-282](https://github.com/arloliu/go-plugin/pull/282)]

## [1.5.1] - 2023-09-05

### Added

- `client`: New `UnixSocketConfig` option in `ClientConfig` to support
  making the client's Unix sockets group-writable.
  [[GH-277](https://github.com/arloliu/go-plugin/pull/277)]

### Fixed

- `server`: `PLUGIN_UNIX_SOCKET_DIR` is consistently used for gRPC broker
  sockets as well as the initial socket.
  [[GH-277](https://github.com/arloliu/go-plugin/pull/277)]

## [1.5.0] - 2023-08-29

### Added

- `client`: New `runner.Runner` interface to support clients providing
  custom plugin command runner implementations.
  [[GH-270](https://github.com/arloliu/go-plugin/pull/270)]
    - Accessible via new `ClientConfig` field `RunnerFunc`, which is
      mutually exclusive with `Cmd` and `Reattach`.
    - Reattaching support via `ReattachConfig` field `ReattachFunc`.
- `client`: New `ClientConfig` field `SkipHostEnv` allows omitting the
  client process' own environment variables from the plugin command's
  environment.
  [[GH-270](https://github.com/arloliu/go-plugin/pull/270)]
- `client`: Add `ID()` method to `Client` for retrieving the pid or other
  unique ID of a running plugin.
  [[GH-272](https://github.com/arloliu/go-plugin/pull/272)]
- `server`: Support setting the directory to create Unix sockets in with
  the env var `PLUGIN_UNIX_SOCKET_DIR`.
  [[GH-270](https://github.com/arloliu/go-plugin/pull/270)]
- `server`: Support setting group write permission and a custom group
  name or gid owner with the env var `PLUGIN_UNIX_SOCKET_GROUP`.
  [[GH-270](https://github.com/arloliu/go-plugin/pull/270)]

## [1.4.11-rc1] - 2023-08-11

### Changed

- `deps`: bump `protoreflect` to v1.15.1.
  [[GH-264](https://github.com/arloliu/go-plugin/pull/264)]

## [1.4.10] - 2023-06-02

### Changed

- `deps`: Remove direct dependency on `golang.org/x/net`.
  [[GH-240](https://github.com/arloliu/go-plugin/pull/240)]

### Fixed

- `additional notes`: ensure to close files.
  [[GH-241](https://github.com/arloliu/go-plugin/pull/241)]

## [1.4.9] - 2023-03-02

### Removed

- `client`: Remove log warning introduced in 1.4.5 when `SecureConfig` is
  nil. [[GH-238](https://github.com/arloliu/go-plugin/pull/238)]

## [1.4.8] - 2022-12-07

### Fixed

- Fix Windows build.
  [[GH-227](https://github.com/arloliu/go-plugin/pull/227)]

## [1.4.7] - 2022-12-06

### Changed

- More detailed error message on plugin start failure.
  [[GH-223](https://github.com/arloliu/go-plugin/pull/223)]

## [1.4.6] - 2022-11-08

### Fixed

- `server`: Prevent gRPC broker goroutine leak when using `GRPCServer`
  type `GracefulStop()` or `Stop()` methods.
  [[GH-220](https://github.com/arloliu/go-plugin/pull/220)]

## [1.4.5] - 2022-08-18

### Added

- `client`: log warning when `SecureConfig` is nil.
  [[GH-207](https://github.com/arloliu/go-plugin/pull/207)]

## [1.4.4] - 2022-05-03

### Changed

- `client`: increase level of plugin exit logs.
  [[GH-195](https://github.com/arloliu/go-plugin/pull/195)]

### Fixed

- Bidirectional communication: fix bidirectional communication when
  AutoMTLS is enabled.
  [[GH-193](https://github.com/arloliu/go-plugin/pull/193)]
- RPC: Trim a spurious log message for plugins using RPC.
  [[GH-186](https://github.com/arloliu/go-plugin/pull/186)]
