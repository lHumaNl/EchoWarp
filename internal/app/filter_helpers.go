package app

import (
	"context"
	"sync/atomic"

	"github.com/lHumaNl/echowarp/pkg/echowarp/audio"
)

// safeSendOpus forwards data to dst, recovering from a "send on closed
// channel" panic. The dst channel is typically a WebRTC peer track send
// channel which is closed externally (by the peer) when the remote side
// disconnects. Between our select choosing the `dst <- data` case and
// the actual send, the peer goroutine may close dst — leading to panic.
// Recover lets the filter goroutine exit cleanly instead of crashing
// the whole process.
//
// Returns true if ctx was canceled or dst was closed (caller should exit).
func safeSendOpus(ctx context.Context, dst chan<- []byte, data []byte) (done bool) {
	defer func() {
		if r := recover(); r != nil {
			// dst closed by peer during disconnect — return data to pool
			// and signal caller to exit.
			audio.PutOpusOutput(data)
			done = true
		}
	}()
	select {
	case dst <- data:
		return false
	case <-ctx.Done():
		audio.PutOpusOutput(data)
		return true
	}
}

// runOpusFilter is the common forwarding loop shared by mute/pause filter
// goroutines. It reads from src, drops frames when the predicate returns true,
// and forwards them otherwise. Handles close-on-disconnect races via safeSendOpus.
func runOpusFilter(ctx context.Context, src <-chan []byte, dst chan<- []byte, drop func() bool) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-src:
			if !ok {
				return
			}
			if drop() {
				audio.PutOpusOutput(data)
				continue
			}
			if safeSendOpus(ctx, dst, data) {
				return
			}
		}
	}
}

// newFlagFilterCh is a generic mute/pause filter factory backed by one atomic.Bool.
func newFlagFilterCh(ctx context.Context, dst chan<- []byte, flag *atomic.Bool) chan<- []byte {
	src := make(chan []byte, cap(dst))
	go runOpusFilter(ctx, src, dst, func() bool { return flag.Load() })
	return src
}

// newTwoFlagFilterCh is a filter dropping when either flag is true.
func newTwoFlagFilterCh(ctx context.Context, dst chan<- []byte, flag1, flag2 *atomic.Bool) chan<- []byte {
	src := make(chan []byte, cap(dst))
	go runOpusFilter(ctx, src, dst, func() bool { return flag1.Load() || flag2.Load() })
	return src
}
