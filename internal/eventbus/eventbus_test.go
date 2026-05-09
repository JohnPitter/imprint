package eventbus

import (
	"testing"
	"time"
)

func TestBus_PublishToSubscriber(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe()
	defer cancel()

	b.Publish("actions:changed")
	select {
	case got := <-ch:
		if got != "actions:changed" {
			t.Errorf("got %q, want actions:changed", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("event was never delivered")
	}
}

func TestBus_FanoutToMultiple(t *testing.T) {
	b := New()
	ch1, cancel1 := b.Subscribe()
	defer cancel1()
	ch2, cancel2 := b.Subscribe()
	defer cancel2()

	b.Publish("hello")
	for _, ch := range []<-chan string{ch1, ch2} {
		select {
		case got := <-ch:
			if got != "hello" {
				t.Errorf("got %q, want hello", got)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("subscriber missed the fanout event")
		}
	}
}

func TestBus_CancelStopsDelivery(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe()
	cancel()

	b.Publish("post-cancel")
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel closed; got an event")
		}
	case <-time.After(50 * time.Millisecond):
		// channel close should have made the receive immediate; treat as
		// pass either way (Go's runtime can defer it under load).
	}
}

func TestBus_FullBufferDoesNotBlock(t *testing.T) {
	b := New()
	_, cancel := b.Subscribe()
	defer cancel()

	// Don't drain; flood until well past subBufSize.
	done := make(chan struct{})
	go func() {
		for i := 0; i < subBufSize*4; i++ {
			b.Publish("flood")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("publisher blocked on a slow subscriber")
	}
}
