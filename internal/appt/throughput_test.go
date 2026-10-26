package appt

import (
	"math"
	"testing"
	"time"

	"github.com/lianxin255/jumpboard/internal/engine"
)

// feed records one reading of a single rule whose counters total total.
func feed(r *rateRing, id string, now time.Time, total int64) bool {
	return r.record([]engine.RuleStatus{{MappingID: id, BytesIn: total}}, now)
}

// TestRateFirstReadingSetsBaseline pins that the first reading only sets the
// baseline: there is no rate until a second one says what changed.
func TestRateFirstReadingSetsBaseline(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	if feed(&r, "m1", now, 5000) {
		t.Fatal("the first reading has no interval to rate")
	}
	if got := r.throughput("m1"); len(got) != 0 {
		t.Errorf("no sample after the first reading, got %v", got)
	}
	if got := r.current("m1"); got != 0 {
		t.Errorf("current rate with no history = %v, want 0", got)
	}
}

// TestRateOneSecondDelta checks the rate itself: 2048 more bytes one second
// later is 2048 B/s.
func TestRateOneSecondDelta(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	feed(&r, "m1", now, 0)
	if !feed(&r, "m1", now.Add(time.Second), 2048) {
		t.Fatal("the second reading should record a sample")
	}
	got := r.throughput("m1")
	if len(got) != 1 || math.Abs(got[0]-2048) > 0.01 {
		t.Fatalf("2048 bytes over 1s = 2048 B/s, got %v", got)
	}
	if cur := r.current("m1"); math.Abs(cur-2048) > 0.01 {
		t.Errorf("current = %v, want 2048", cur)
	}
}

// TestRateCounterResetSkipped: a re-apply restarts the forwarder and its
// counters from zero. The tick that sees them go backwards records nothing
// rather than a negative rate, and the reset becomes the new baseline.
func TestRateCounterResetSkipped(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	feed(&r, "m1", now, 4096)
	if feed(&r, "m1", now.Add(time.Second), 100) {
		t.Fatal("a counter that went backwards is not a rate")
	}
	if got := r.throughput("m1"); len(got) != 0 {
		t.Fatalf("no sample across a reset, got %v", got)
	}
	if !feed(&r, "m1", now.Add(2*time.Second), 1100) {
		t.Fatal("the reading after the reset should record a sample")
	}
	if got := r.current("m1"); math.Abs(got-1000) > 0.01 {
		t.Errorf("rate from the new baseline = %v, want 1000", got)
	}
}

// TestRateTooSoonReadingSkipped: a reading under half a second after the
// last says more about the tick than the traffic, so it only moves the
// baseline.
func TestRateTooSoonReadingSkipped(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	feed(&r, "m1", now, 0)
	if feed(&r, "m1", now.Add(200*time.Millisecond), 4096) {
		t.Fatal("a reading too soon should not record")
	}
	if got := r.throughput("m1"); len(got) != 0 {
		t.Fatalf("no sample from a too-soon reading, got %v", got)
	}
}

// TestRateRingCapsAtWindow pins the ring's size: once the minute is full the
// oldest sample is dropped. Reading i carries i*i total bytes, so the sample
// after it rates 2i-1 B/s — every sample a different value.
func TestRateRingCapsAtWindow(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	for i := 0; i <= RateWindow+5; i++ {
		feed(&r, "m1", now.Add(time.Duration(i)*time.Second), int64(i)*int64(i))
	}
	got := r.throughput("m1")
	if len(got) != RateWindow {
		t.Fatalf("history caps at %d samples, got %d", RateWindow, len(got))
	}
	// 66 readings leave 65 samples (1, 3, …, 129 B/s); keeping the newest 60
	// drops the first five, so the window opens on 11 and closes on 129.
	if math.Abs(got[0]-11) > 0.01 || math.Abs(got[RateWindow-1]-129) > 0.01 {
		t.Errorf("the oldest samples should be dropped: first %v last %v, want 11 and 129",
			got[0], got[RateWindow-1])
	}
}

// TestThroughputReturnsACopy: the frame walks the slice after the lock is
// released, so mutating it must not reach the next call.
func TestThroughputReturnsACopy(t *testing.T) {
	var r rateRing
	now := time.Unix(1700000000, 0)
	feed(&r, "m1", now, 0)
	feed(&r, "m1", now.Add(time.Second), 1024)
	first := r.throughput("m1")
	first[0] = -1
	if again := r.throughput("m1"); again[0] != 1024 {
		t.Errorf("throughput must return a copy, saw the mutated %v", again[0])
	}
}

// TestSamplerLifecycle pins the loop's lifecycle: Stop ends it, waits for
// the goroutine, and is safe to call again.
func TestSamplerLifecycle(t *testing.T) {
	s := newRateSampler(engine.NewLocal(), nil)
	s.Start()
	s.Stop()
	s.Stop()
}
