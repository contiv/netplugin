// Package hub provides a simple event dispatcher for publish/subscribe pattern.
package hub

import "sync"

// Event is an interface for published events.
type Event interface {
	Kind() int
}

// Hub is an event dispatcher, publishes events to the subscribers
// which are subscribed for a specific event type.
// Optimized for publish calls.
// The handlers may be called in order different than they are registered.
type Hub struct {
	subscribers map[int][]handler
	m           sync.RWMutex
	seq         uint64
}

type handler struct {
	f  func(Event)
	id uint64
}

// Subscribe registers f for the event of a specific kind.
func (h *Hub) Subscribe(kind int, f func(Event)) (cancel func()) {
	var cancelled bool
	h.m.Lock()
	h.seq++
	id := h.seq
	if h.subscribers == nil {
		h.subscribers = make(map[int][]handler)
	}
	h.subscribers[kind] = append(h.subscribers[kind], handler{id: id, f: f})
	h.m.Unlock()
	return func() {
		h.m.Lock()
		if cancelled {
			panic("subscription is already cancelled")
		}
		cancelled = true
		a := h.subscribers[kind]
		for i, h := range a {
			if h.id == id {
				a[i], a = a[len(a)-1], a[:len(a)-1]
				break
			}
		}
		if len(a) == 0 {
			delete(h.subscribers, kind)
		}
		h.m.Unlock()
	}
}

// Publish an event to the subscribers.
func (h *Hub) Publish(e Event) {
	h.m.RLock()
	if handlers, ok := h.subscribers[e.Kind()]; ok {
		for _, h := range handlers {
			h.f(e)
		}
	}
	h.m.RUnlock()
}

// DefaultHub is the default Hub used by Publish and Subscribe.
var DefaultHub Hub

// Subscribe registers f for the event of a specific kind in the DefaultHub.
func Subscribe(kind int, f func(Event)) (cancel func()) {
	return DefaultHub.Subscribe(kind, f)
}

// Publish an event to the subscribers in DefaultHub.
func Publish(e Event) {
	DefaultHub.Publish(e)
}
