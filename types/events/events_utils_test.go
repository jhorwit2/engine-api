package events

import (
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestEventHandler(t *testing.T) {
	// Setup the channel with an event
	c := make(chan Message, 1)
	c <- Message{
		ID:     "test_id",
		Action: "testing",
	}
	defer close(c)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// Register an event to process the messages.
	eh := NewEventHandler()

	actionCalled := false
	eh.Handle("testing", func(event Message) {
		actionCalled = true
		cancel()
	})

	eh.Watch(ctx, c)

	if !actionCalled {
		t.Fatal("Expected action handler to be called but it was not")
	}
}
