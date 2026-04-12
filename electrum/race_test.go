package electrum_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
)

// goroutinesLeakCheck records the current goroutine count and returns a
// checker that asserts the count has returned to at most that baseline.
// Call the returned func after Shutdown to verify no goroutines leaked.
//
// Uses manual polling rather than require.Eventually: testify's Eventually
// evaluates the condition inside a spawned goroutine, which inflates
// runtime.NumGoroutine() by 1 on every tick and causes the check to
// permanently fail when the target count exactly equals the baseline.
func goroutinesLeakCheck(t *testing.T) func() {
	t.Helper()
	baseline := runtime.NumGoroutine()
	return func() {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if runtime.NumGoroutine() <= baseline {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Errorf("goroutine leak: count did not return to baseline %d (now %d)",
			baseline, runtime.NumGoroutine())
	}
}

// startAutoResponder spawns a goroutine that reads every message sent on
// transport and replies with a JSON response carrying the same request ID.
// The returned stop func shuts the responder down.
func startAutoResponder(transport *MockTransport, result any) func() {
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case msg := <-transport.sent():
				var req struct {
					ID uint64 `json:"id"`
				}
				if err := json.Unmarshal(msg, &req); err != nil {
					return
				}
				resp, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"id":      req.ID,
					"result":  result,
				})
				transport.responses() <- resp
			}
		}
	}()
	return func() { close(stop) }
}

// ── network.go fixes ─────────────────────────────────────────────────────────

// TestShutdown_Concurrent verifies that calling Shutdown from many goroutines
// simultaneously does not panic. Before the fix Shutdown called close(s.quit)
// without a sync.Once guard, causing a "close of closed channel" panic on
// concurrent or repeated calls.
func TestShutdown_Concurrent(t *testing.T) {
	ctx := context.Background()
	client, _ := newTestClient(ctx, t)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Shutdown()
		}()
	}
	wg.Wait()
}

// TestConcurrentRequests_HandlersPreserved verifies that completing one request
// does not destroy the handlers of all other in-flight requests. Before the fix
// the deferred cleanup returned nil instead of val, wiping the entire map.
func TestConcurrentRequests_HandlersPreserved(t *testing.T) {
	ctx := context.Background()
	client, transport := newTestClient(ctx, t)
	defer client.Shutdown()

	const N = 5

	// Respond to each request using the ID extracted from the wire message so
	// responses are correctly routed regardless of completion order.
	go func() {
		for i := 0; i < N; i++ {
			msg := <-transport.sent()
			var req struct {
				ID uint64 `json:"id"`
			}
			if err := json.Unmarshal(msg, &req); err != nil {
				return
			}
			resp, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  nil,
			})
			transport.responses() <- resp
		}
	}()

	var wg sync.WaitGroup
	errs := make([]error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = client.Ping(ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "request %d should succeed", i)
	}
}

// ── subscribe.go goroutine-leak fixes ────────────────────────────────────────

// TestSubscribeHeaders_NoGoroutineLeak verifies that the goroutine spawned by
// SubscribeHeaders exits when the client is shut down. Before the fix the
// goroutine looped on `range s.listenPush(...)`, which blocks forever because
// the push channel is never closed. The fix adds a select on s.quit.
func TestSubscribeHeaders_NoGoroutineLeak(t *testing.T) {
	checkLeaks := goroutinesLeakCheck(t)

	ctx := context.Background()
	client, transport := newTestClient(ctx, t)

	err := transport.exec(1, &electrum.SubscribeHeadersResult{Height: 1, Hex: "aabbcc"}, func() error {
		_, err := client.SubscribeHeaders(ctx)
		return err
	})
	require.NoError(t, err)

	client.Shutdown()
	checkLeaks()
}

// TestSubscribeMasternode_NoGoroutineLeak is the SubscribeMasternode analogue
// of TestSubscribeHeaders_NoGoroutineLeak.
func TestSubscribeMasternode_NoGoroutineLeak(t *testing.T) {
	checkLeaks := goroutinesLeakCheck(t)

	ctx := context.Background()
	client, transport := newTestClient(ctx, t)

	err := transport.exec(1, "ENABLED", func() error {
		_, err := client.SubscribeMasternode(ctx, "collateral1")
		return err
	})
	require.NoError(t, err)

	client.Shutdown()
	checkLeaks()
}

// TestSubscribeScripthash_NoGoroutineLeak is the SubscribeScripthash analogue
// of TestSubscribeHeaders_NoGoroutineLeak. SubscribeScripthash makes no
// initial server request so no mock response is needed.
func TestSubscribeScripthash_NoGoroutineLeak(t *testing.T) {
	checkLeaks := goroutinesLeakCheck(t)

	ctx := context.Background()
	client, _ := newTestClient(ctx, t)

	client.SubscribeScripthash()

	client.Shutdown()
	checkLeaks()
}

// ── subscribe.go locking fixes ───────────────────────────────────────────────

// TestScripthashSubscription_ConcurrentAccess runs Add, Remove, RemoveAddress,
// GetAddress, GetScripthash, SH, and Resubscribe from many goroutines
// simultaneously. With -race this covers:
//   - TOCTOU in Remove / RemoveAddress / Resubscribe (loop over subscribedSH
//     before holding the lock)
//   - Unprotected map reads in GetAddress and GetScripthash
//   - SH returning a reference to the live slice without a lock
func TestScripthashSubscription_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	client, transport := newTestClient(ctx, t)
	defer func() {
		client.Shutdown()
	}()

	stopResponder := startAutoResponder(transport, "status")
	defer stopResponder()

	sub, notifChan := client.SubscribeScripthash()

	// Drain notifications so notifChan never fills up during the test.
	go func() {
		for range notifChan {
		}
	}()

	// Seed initial entries sequentially so there is data for concurrent readers.
	const initial = 5
	for i := 0; i < initial; i++ {
		require.NoError(t, sub.Add(ctx, fmt.Sprintf("sh%d", i), fmt.Sprintf("addr%d", i)))
	}

	var wg sync.WaitGroup

	// Concurrent reads.
	for i := 0; i < 10; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = sub.SH()
		}()
		go func(i int) {
			defer wg.Done()
			_, _ = sub.GetAddress(fmt.Sprintf("sh%d", i%initial))
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = sub.GetScripthash(fmt.Sprintf("addr%d", i%initial))
		}(i)
	}

	// Concurrent writes: add new entries.
	for i := initial; i < initial+5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = sub.Add(ctx, fmt.Sprintf("sh%d", i), fmt.Sprintf("addr%d", i))
		}(i)
	}

	// Concurrent writes: remove existing entries (errors are acceptable when
	// another goroutine races to remove the same entry first).
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = sub.Remove(ctx, fmt.Sprintf("sh%d", i))
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = sub.RemoveAddress(fmt.Sprintf("addr%d", i))
		}(i)
	}

	// Concurrent resubscribe: takes a snapshot under RLock then re-adds each.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sub.Resubscribe(ctx)
	}()

	wg.Wait()
}

// TestSubscribeScripthash_NotificationAndRemoveNoDeadlock reproduces the
// deadlock that occurred when a notification arrived while notifChan was at
// capacity and Remove was called concurrently.
//
// Old behaviour: the listener goroutine held sub.lock (write lock) while
// blocking on the full channel; Remove could not acquire the lock → deadlock.
//
// Fixed by releasing sub.lock (downgraded to RLock) before the channel send.
func TestSubscribeScripthash_NotificationAndRemoveNoDeadlock(t *testing.T) {
	ctx := context.Background()
	client, transport := newTestClient(ctx, t)
	defer func() {
		client.Shutdown()
	}()

	stopResponder := startAutoResponder(transport, "status")
	defer stopResponder()

	sub, notifChan := client.SubscribeScripthash()

	require.NoError(t, sub.Add(ctx, "sh1"))
	<-notifChan // drain the Add notification

	// Fill notifChan to its full capacity (buffer == 10) without draining it.
	for i := 0; i < 10; i++ {
		require.NoError(t, transport.notify("blockchain.scripthash.subscribe",
			[2]string{"sh1", fmt.Sprintf("fill%d", i)}))
	}
	require.Eventually(t, func() bool { return len(notifChan) == 10 },
		2*time.Second, 5*time.Millisecond, "notifChan should be full")

	// Trigger one more notification. The listener goroutine will try to forward
	// it to the already-full notifChan.
	// Old code: listener held sub.lock while blocking → Remove deadlocked.
	// New code: listener releases lock before the send → Remove proceeds.
	require.NoError(t, transport.notify("blockchain.scripthash.subscribe",
		[2]string{"sh1", "trigger"}))

	// Call Remove concurrently; it must complete within the timeout.
	removeDone := make(chan error, 1)
	go func() {
		removeDone <- sub.Remove(ctx, "sh1")
	}()

	// Drain notifChan so the listener goroutine eventually unblocks.
	go func() {
		for range notifChan {
		}
	}()

	select {
	case err := <-removeDone:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: Remove did not complete within timeout")
	}
}
