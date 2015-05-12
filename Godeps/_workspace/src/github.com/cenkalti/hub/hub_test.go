package hub

import "testing"

const testKind = 1
const testValue = "foo"

type testEvent string

func (e testEvent) Kind() int {
	return testKind
}

func TestPubSub(t *testing.T) {
	var h Hub
	var s string

	h.Subscribe(testKind, func(e Event) { s = string(e.(testEvent)) })
	h.Publish(testEvent(testValue))

	if s != testValue {
		t.Errorf("invalid value: %s", s)
	}
}

func TestCancel(t *testing.T) {
	var h Hub
	var called bool

	cancel := h.Subscribe(testKind, func(e Event) { called = true })
	cancel()
	h.Publish(testEvent(testValue))

	if called {
		t.Error("unexpected call")
	}
}
