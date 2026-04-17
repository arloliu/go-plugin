// Copyright IBM Corp. 2016, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestManagedClients_RemovedOnKill verifies that Kill drops the client
// from the global managedClients slice. Before the fix, entries lived
// until process exit, so a long-running host creating and killing many
// managed plugins accumulated stale references forever.
func TestManagedClients_RemovedOnKill(t *testing.T) {
	// Snapshot initial state; don't assume test ordering.
	managedClientsLock.Lock()
	baseline := len(managedClients)
	managedClientsLock.Unlock()

	// Construct a managed Client with a nil runner so Kill returns early
	// after the cleanup deferred runs. This exercises the removal logic
	// without needing a real subprocess.
	c := &Client{
		config: &ClientConfig{Managed: true},
		logger: hclog.NewNullLogger(),
	}
	managedClientsLock.Lock()
	managedClients = append(managedClients, c)
	managedClientsLock.Unlock()

	// Kill with runner==nil returns before the deferred cleanup runs.
	// We need the cleanup path, so we invoke removeManagedClient
	// directly — the contract under test is that it is called, which
	// we also verify by call-through from the deferred block via a
	// second test below.
	removeManagedClient(c)

	managedClientsLock.Lock()
	after := len(managedClients)
	managedClientsLock.Unlock()
	if after != baseline {
		t.Fatalf("expected managedClients to return to baseline %d, got %d", baseline, after)
	}
}

// TestRemoveManagedClient_NotPresent ensures the removal helper is a
// safe no-op for a client that isn't tracked, so double-calls from
// defer paths can't corrupt the slice.
func TestRemoveManagedClient_NotPresent(t *testing.T) {
	managedClientsLock.Lock()
	before := len(managedClients)
	managedClientsLock.Unlock()

	removeManagedClient(&Client{})

	managedClientsLock.Lock()
	after := len(managedClients)
	managedClientsLock.Unlock()
	if after != before {
		t.Fatalf("managedClients changed on no-op remove: before=%d after=%d", before, after)
	}
}

// TestManagedClients_DoubleKill_Idempotent verifies that calling Kill a
// second time on a managed client does not corrupt or mutate the
// managedClients slice. The first Kill removes the entry; the second
// must be a no-op in terms of slice state. Without this guarantee a
// supervisor that optimistically calls Kill from multiple cleanup paths
// would risk slice corruption.
func TestManagedClients_DoubleKill_Idempotent(t *testing.T) {
	managedClientsLock.Lock()
	baseline := len(managedClients)
	managedClientsLock.Unlock()

	process := helperProcess("mock")
	c := NewClient(&ClientConfig{
		Cmd:             process,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
		Managed:         true,
	})
	if _, err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	c.Kill()
	managedClientsLock.Lock()
	afterFirst := len(managedClients)
	managedClientsLock.Unlock()

	c.Kill()
	managedClientsLock.Lock()
	afterSecond := len(managedClients)
	managedClientsLock.Unlock()

	if afterFirst != baseline {
		t.Fatalf("first Kill did not return slice to baseline: baseline=%d after=%d", baseline, afterFirst)
	}
	if afterSecond != afterFirst {
		t.Fatalf("second Kill changed slice: afterFirst=%d afterSecond=%d", afterFirst, afterSecond)
	}
}

// TestManagedClients_RemovedOnKill_Integration exercises the full Kill
// deferred path against a real plugin subprocess. This guards against a
// future refactor that moves the removeManagedClient call out of the
// defer block and silently reintroduces the leak. Pairs with the unit
// test above.
func TestManagedClients_RemovedOnKill_Integration(t *testing.T) {
	managedClientsLock.Lock()
	baseline := len(managedClients)
	managedClientsLock.Unlock()

	process := helperProcess("mock")
	c := NewClient(&ClientConfig{
		Cmd:             process,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
		Managed:         true,
	})

	managedClientsLock.Lock()
	afterCreate := len(managedClients)
	managedClientsLock.Unlock()
	if afterCreate != baseline+1 {
		t.Fatalf("managed client not registered: baseline=%d afterCreate=%d", baseline, afterCreate)
	}

	// Start the plugin (the "mock" helper emits a valid handshake) so
	// the Kill deferred block has a runner to wait on and reaches the
	// removeManagedClient call.
	if _, err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	c.Kill()

	managedClientsLock.Lock()
	afterKill := len(managedClients)
	managedClientsLock.Unlock()
	if afterKill != baseline {
		t.Fatalf("Kill did not remove managed client: baseline=%d afterKill=%d", baseline, afterKill)
	}
}
