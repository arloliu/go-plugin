// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package cmdrunner

import (
	"errors"
	"os/exec"
)

// errProcessGroupNotSupported signals to Kill that the platform has no
// process-group primitive we can use; the caller should fall back to
// killing the plugin PID only. A Windows Job Object-based implementation
// would be the correct fix; tracked as future work.
var errProcessGroupNotSupported = errors.New("process group kill not supported on windows")

func configureProcessGroup(_ *exec.Cmd) {
	// No-op on Windows for now.
}

func killProcessGroup(_ int) error {
	return errProcessGroupNotSupported
}
