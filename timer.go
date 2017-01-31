// Package timer is a Go timer implementation with a fixed Reset behavior.
package timer

import (
	"sync"
	"time"
)

// The Timer type represents a single event. When the Timer expires,
// the current time will be sent on C, unless the Timer was created by AfterFunc.
// A Timer must be created with NewTimer. NewStoppedTimer or AfterFunc.
type Timer struct {
	C <-chan time.Time

	i    int   // heap index.
	when int64 // Timer wakes up at when.

	// f is called in a locked context on timeout. This function must not block
	// and must behave well-defined.
	f func(t *time.Time)

	// reset is called in a locked context. This function must not block
	// and must behave well-defined.
	reset func()
}

// AfterFunc waits for the duration to elapse and then calls f in its own goroutine.
// It returns a Timer that can be used to cancel the call using its Stop method.
func AfterFunc(d time.Duration, f func()) *Timer {
	var m sync.RWMutex // Use a RWMutex to allow concurrent callback calls.
	var ctx uint32
	t := &Timer{
		f: func(*time.Time) {
			fctx := ctx
			go func() {
				m.RLock()
				defer m.RUnlock() // Use defer for panics.
				if fctx == ctx {
					f()
				}
			}()
		},
		reset: func() {
			m.Lock()
			ctx++
			if ctx == 1<<32-1 { // math.MaxUint32
				ctx = 0
			}
			m.Unlock()
		},
	}
	addTimer(t, d)
	return t
}

// NewTimer creates a new Timer that will send the current time on its
// channel after at least duration d.
func NewTimer(d time.Duration) *Timer {
	t := NewStoppedTimer()
	addTimer(t, d)
	return t
}

// NewStoppedTimer creates a new stopped Timer.
func NewStoppedTimer() *Timer {
	c := make(chan time.Time, 1)
	t := &Timer{
		C: c,
		f: func(t *time.Time) {
			// Don't block.
			select {
			case c <- *t:
			default:
			}
		},
		reset: func() {
			// Empty the channel if filled.
			select {
			case <-c:
			default:
			}
		},
	}
	return t
}

// Stop prevents the Timer from firing.
// It returns true if the call stops the timer,
// false if the timer has already expired or been stopped.
// Stop does not close the channel, to prevent a read from
// the channel succeeding incorrectly.
func (t *Timer) Stop() (wasActive bool) {
	if t.f == nil {
		panic("timer: Stop called on uninitialized Timer")
	}
	return delTimer(t)
}

// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active,
// false if the timer had expired or been stopped.
// The channel t.C is cleared and calling t.Reset() behaves as creating a
// new Timer.
func (t *Timer) Reset(d time.Duration) bool {
	if t.f == nil {
		panic("timer: Reset called on uninitialized Timer")
	}
	return resetTimer(t, d)
}
