// Package eventbus is a tiny in-process publish/subscribe used to push
// "something changed" notifications from the write side of the system to
// long-lived SSE subscribers on the read side.
//
// Two design choices that matter:
//
//  1. Events are deliberately coarse strings ("actions:changed",
//     "memories:changed"). The HTTP handler uses them only as a hint to
//     re-fetch — the real freshness contract still belongs to the database.
//     If a publish is missed (slow consumer, broker drop, restart) the
//     poll fallback in the client picks up the change within a few seconds.
//
//  2. Subscribers receive events on a buffered channel; if a subscriber
//     does not drain fast enough we drop events for that subscriber rather
//     than blocking the publisher. The whole pubsub is best-effort by design.
package eventbus

import "sync"

const subBufSize = 8

// Bus is a many-publishers/many-subscribers fanout for short string events.
// Safe for concurrent use.
type Bus struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

// New returns a ready Bus.
func New() *Bus {
	return &Bus{subs: map[chan string]struct{}{}}
}

// Subscribe returns a receive channel and a cancel function. The caller
// MUST call cancel when done so the bus can drop the channel and stop
// trying to deliver events.
func (b *Bus) Subscribe() (<-chan string, func()) {
	ch := make(chan string, subBufSize)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Publish sends the event to every current subscriber. Subscribers whose
// buffers are full silently drop the event — see the package doc for why.
func (b *Bus) Publish(event string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- event:
		default:
			// Buffer full; drop. The poll fallback covers this case.
		}
	}
}
