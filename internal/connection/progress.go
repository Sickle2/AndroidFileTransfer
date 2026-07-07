package connection

import (
	"sync"

	"AndroidFileTransfer/internal/model"
)

// Broadcaster is a fan-out pub/sub hub for TransferProgress events.
// It is safe for concurrent use.
type Broadcaster struct {
	mu          sync.Mutex
	subscribers []chan model.TransferProgress
	closed      bool
}

// Subscribe creates a new buffered channel (capacity 16) and registers it as a
// subscriber. The caller must drain or discard the channel; full channels are
// silently skipped by Publish.
func (b *Broadcaster) Subscribe() <-chan model.TransferProgress {
	ch := make(chan model.TransferProgress, 16)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		close(ch)
		return ch
	}
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Publish sends p to every subscriber in a non-blocking fashion.
// Subscribers whose channels are full are silently skipped.
func (b *Broadcaster) Publish(p model.TransferProgress) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for _, ch := range b.subscribers {
		select {
		case ch <- p:
		default:
			// channel full — drop rather than block
		}
	}
}

// Close closes all subscriber channels and marks the broadcaster as closed.
// Any subsequent Subscribe call will receive an already-closed channel.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, ch := range b.subscribers {
		close(ch)
	}
	b.subscribers = nil
}
