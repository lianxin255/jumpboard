package appt

import (
	"sync"
	"time"

	"github.com/lianxin255/jumpboard/internal/engine"
)

// RateWindow is how many seconds of throughput a rule keeps — the span the
// detail popover's chart draws.
const RateWindow = 60

// sampleEvery is how often the counters are read.
const sampleEvery = time.Second

// rateSample is one tick of a rule's throughput, in bytes per second.
type rateSample struct {
	at  time.Time
	bps float64
}

// prevReading is a rule's counters at the previous tick, the baseline the
// next tick's rate is differenced against.
type prevReading struct {
	bytes int64
	at    time.Time
}

// rateRing keeps the last RateWindow samples of every rule. Its zero value
// is ready to record, so the tests drive it without the app around it.
type rateRing struct {
	mu      sync.Mutex
	samples map[string][]rateSample
	prev    map[string]prevReading
}

// record folds one reading of every rule into the history, and reports
// whether any sample landed.
func (r *rateRing) record(rules []engine.RuleStatus, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.prev == nil {
		r.prev = map[string]prevReading{}
	}
	if r.samples == nil {
		r.samples = map[string][]rateSample{}
	}
	any := false
	for _, rule := range rules {
		total := rule.BytesIn + rule.BytesOut
		prev, seen := r.prev[rule.MappingID]
		r.prev[rule.MappingID] = prevReading{bytes: total, at: now}
		elapsed := now.Sub(prev.at).Seconds()
		// The first reading only sets the baseline; a reading too soon says
		// more about the tick than the traffic; a counter that went
		// backwards is a re-apply resetting it. None of them is a rate.
		if !seen || elapsed < 0.5 || total < prev.bytes {
			continue
		}
		hist := append(r.samples[rule.MappingID],
			rateSample{at: now, bps: float64(total-prev.bytes) / elapsed})
		if len(hist) > RateWindow {
			hist = hist[len(hist)-RateWindow:]
		}
		r.samples[rule.MappingID] = hist
		any = true
	}
	return any
}

// throughput returns a rule's rates oldest-first, copied so the frame can
// walk them without the lock.
func (r *rateRing) throughput(mappingID string) []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	hist := r.samples[mappingID]
	if len(hist) == 0 {
		return nil
	}
	out := make([]float64, len(hist))
	for i, s := range hist {
		out[i] = s.bps
	}
	return out
}

// current is a rule's newest rate, 0 with no history yet.
func (r *rateRing) current(mappingID string) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	hist := r.samples[mappingID]
	if len(hist) == 0 {
		return 0
	}
	return hist[len(hist)-1].bps
}

// rateSampler reads the engine's counters once a second and keeps a rate
// ring per rule, for the detail popover's throughput curve. Its lifecycle
// mirrors the health checker's: started once the app is built, stopped by
// Close.
type rateSampler struct {
	rateRing

	eng *engine.Local
	// onChange asks the window for a frame after a tick that recorded
	// something, so an idle engine never wakes it.
	onChange func()

	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

// newRateSampler reads counters from eng and reports new samples through
// onChange.
func newRateSampler(eng *engine.Local, onChange func()) *rateSampler {
	return &rateSampler{eng: eng, onChange: onChange}
}

// Start begins sampling. Calling it twice restarts the loop.
func (s *rateSampler) Start() {
	s.Stop() // a loop already running must end first
	stop, done := make(chan struct{}), make(chan struct{})
	s.mu.Lock()
	s.stop, s.done = stop, done
	s.mu.Unlock()
	go func() {
		defer close(done)
		t := time.NewTicker(sampleEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-t.C:
				// Rules locks, so the read is safe off the main thread.
				if s.rateRing.record(s.eng.Rules(), now) && s.onChange != nil {
					s.onChange()
				}
			}
		}
	}()
}

// Stop ends the loop and waits for its goroutine to leave. Idempotent.
func (s *rateSampler) Stop() {
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop, s.done = nil, nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
}
