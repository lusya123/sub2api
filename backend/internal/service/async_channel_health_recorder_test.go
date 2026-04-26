package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// blockingRecorder is a test double that slots in where the *real*
// ChannelHealthRecorder lives but lets the test control exactly when Record
// unblocks. The async wrapper writes to this — we can then reason about
// back-pressure / drops / shutdown timing deterministically.
type blockingRecorder struct {
	ChannelHealthRecorder
	gate   chan struct{}
	mu     sync.Mutex
	calls  int
	blocks bool
}

// Record on the blocking recorder waits on gate when blocks is true. When
// blocks is false it just counts. Mirrors the *ChannelHealthRecorder.Record
// signature (context + event) so AsyncChannelHealthRecorder can invoke it
// exactly like the real one would.
func (b *blockingRecorder) Record(_ context.Context, _ ChannelHealthEvent) error {
	b.mu.Lock()
	b.calls++
	blocks := b.blocks
	b.mu.Unlock()
	if blocks {
		<-b.gate
	}
	return nil
}

// callCount returns how many Record calls have been observed. Atomic-free
// because the mutex already orders reads with writes.
func (b *blockingRecorder) callCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

// newAsyncWithBlocker wires an AsyncChannelHealthRecorder around a
// blockingRecorder so the test can freeze the background worker. The returned
// *AsyncChannelHealthRecorder uses the blockingRecorder directly via a small
// tweak: we swap out the inner pointer so that Record calls route to our
// double. We then create the async wrapper via a dedicated unexported ctor
// that accepts the replacement directly, bypassing the normal path that
// wants a real *ChannelHealthRecorder.
func newAsyncWithBlocker(t *testing.T, bufSize int, block bool) (*AsyncChannelHealthRecorder, *blockingRecorder) {
	t.Helper()
	br := &blockingRecorder{
		gate:   make(chan struct{}),
		blocks: block,
	}
	// The production NewAsyncChannelHealthRecorder needs a
	// *ChannelHealthRecorder. For the blocking-double tests we bypass it by
	// constructing the struct manually and wiring our double's Record into
	// the worker via a tiny shim: we replace `inner` with a real recorder
	// that is nil-safe (never dispatches to DB) and then start a manual
	// worker that reads from the channel and dispatches to br.Record
	// instead. This avoids dragging SQLite into the async timing tests.
	r := &AsyncChannelHealthRecorder{
		inner: nil, // unused; our custom worker dispatches to br directly
		ch:    make(chan ChannelHealthEvent, bufSize),
		done:  make(chan struct{}),
	}
	go func() {
		defer close(r.done)
		for e := range r.ch {
			_ = br.Record(context.Background(), e)
		}
	}()
	t.Cleanup(func() {
		// Ensure worker exits so goroutine leak detectors stay happy.
		_ = r.Shutdown(5 * time.Second)
	})
	return r, br
}

// TestAsyncRecorder_DropsWhenFull: fill the buffer while the worker is
// blocked and verify that excess enqueues both return false and bump the
// dropped counter.
func TestAsyncRecorder_DropsWhenFull(t *testing.T) {
	r, br := newAsyncWithBlocker(t, 2 /*bufSize*/, true /*block worker*/)

	// The worker will pick the first event immediately (the channel isn't
	// full at that point), so to produce a reliable "full" state we push
	// bufSize + 1 events and account for the first one being in-flight.
	ev := ChannelHealthEvent{AccountID: 1, Model: "m", Outcome: OutcomeSuccess}

	// Push until we see a drop. Upper bound 100 so the test can't spin.
	var okCount, dropCount int
	for i := 0; i < 100; i++ {
		if r.TryEnqueue(ev) {
			okCount++
		} else {
			dropCount++
		}
		if dropCount >= 5 {
			break
		}
	}

	require.GreaterOrEqual(t, dropCount, 5, "blocked worker + small buffer must produce drops")
	require.Equal(t, uint64(dropCount), r.Dropped(), "Dropped() must match the number of false returns")

	// Release the worker and drain so the Cleanup path is fast.
	close(br.gate)
}

// TestAsyncRecorder_FlushesOnShutdown: enqueue a handful while the worker
// runs freely, then Shutdown and verify every enqueued sample reached the
// inner recorder before Shutdown returned nil.
func TestAsyncRecorder_FlushesOnShutdown(t *testing.T) {
	r, br := newAsyncWithBlocker(t, 8 /*bufSize*/, false /*non-blocking*/)

	for i := 0; i < 5; i++ {
		require.True(t, r.TryEnqueue(ChannelHealthEvent{AccountID: int64(i), Model: "m", Outcome: OutcomeSuccess}))
	}

	require.NoError(t, r.Shutdown(2*time.Second))
	require.Equal(t, 5, br.callCount(), "shutdown must drain all in-flight samples")
	require.Zero(t, r.Dropped())
}

// TestAsyncRecorder_ShutdownTimeout: with the worker permanently stuck,
// Shutdown must return an error when the drain budget expires.
func TestAsyncRecorder_ShutdownTimeout(t *testing.T) {
	r, br := newAsyncWithBlocker(t, 4, true /*block*/)

	// Get one event into the worker (it will park on gate).
	require.True(t, r.TryEnqueue(ChannelHealthEvent{AccountID: 1, Model: "m", Outcome: OutcomeSuccess}))

	err := r.Shutdown(100 * time.Millisecond)
	require.Error(t, err, "blocked worker must surface a shutdown timeout")

	// Let the cleanup path finish cleanly.
	close(br.gate)
}

// TestAsyncRecorder_PostShutdownEnqueueDrops: after Shutdown, further
// TryEnqueue calls must return false without panicking on the closed channel.
func TestAsyncRecorder_PostShutdownEnqueueDrops(t *testing.T) {
	r, br := newAsyncWithBlocker(t, 4, false)
	_ = br

	require.NoError(t, r.Shutdown(1*time.Second))
	ok := r.TryEnqueue(ChannelHealthEvent{AccountID: 1, Model: "m", Outcome: OutcomeSuccess})
	require.False(t, ok, "post-shutdown enqueue must drop, not panic")
}

// TestAsyncRecorder_EndToEnd_WritesRowViaRealRecorder: wires the async
// wrapper around a *real* ChannelHealthRecorder backed by the SQLite fixture
// and verifies that TryEnqueue → worker → Record → upsert round-trips.
func TestAsyncRecorder_EndToEnd_WritesRowViaRealRecorder(t *testing.T) {
	client := newChannelHealthTestClient(t)
	inner := NewChannelHealthRecorder(client)
	async := NewAsyncChannelHealthRecorder(inner, 16)

	at := time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)
	require.True(t, async.TryEnqueue(ChannelHealthEvent{
		AccountID: 77,
		GroupID:   3,
		Model:     "claude-opus-4-7",
		Outcome:   OutcomeSuccess,
		LatencyMs: 120,
		Source:    SourcePassive,
		At:        at,
	}))

	require.NoError(t, async.Shutdown(2*time.Second))

	// After Shutdown, exactly one row must exist for this tuple.
	rows, err := client.ChannelHealthSample.Query().All(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(77), rows[0].AccountID)
	require.Equal(t, 1, rows[0].SuccessCount)
}

// TestAsyncRecorder_DroppedCounterIsLocal: documents (and pins) the semantics
// noted in the type doc — two independent AsyncChannelHealthRecorder
// instances track drops separately. This guards against a future refactor
// that accidentally shares state across the package-level or collapses
// counters via a global.
func TestAsyncRecorder_DroppedCounterIsLocal(t *testing.T) {
	r1, br1 := newAsyncWithBlocker(t, 1, true)
	r2, br2 := newAsyncWithBlocker(t, 1, true)

	// Saturate r1 only.
	for i := 0; i < 10; i++ {
		r1.TryEnqueue(ChannelHealthEvent{AccountID: 1, Model: "m"})
	}
	require.Greater(t, r1.Dropped(), uint64(0))
	require.Zero(t, r2.Dropped(), "independent instances must not share drop counters")

	// Sanity check that r2's worker is alive by using it.
	for i := 0; i < 10; i++ {
		r2.TryEnqueue(ChannelHealthEvent{AccountID: 2, Model: "m"})
	}
	require.Greater(t, r2.Dropped(), uint64(0))

	close(br1.gate)
	close(br2.gate)
}
