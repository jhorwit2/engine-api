package events

import (
	"sync"

	"golang.org/x/net/context"
)

// EventProcessor gets called when an event comes in
// for a registered action
type EventProcessor func(event Message)

// EventHandler is abstract interface for users to customize
// their own handle functions for each type of event action.
type EventHandler interface {
	// Handle registers the event processor for the given action.
	// Multiple calls to handle with same action will result in overriding
	// the pervious event processor for the same action.
	Handle(action string, ep EventProcessor)

	// Get the handler for the given action or nil if the action is not registered.
	Get(action string) EventProcessor

	// Watch the stream of messages applying any registered
	// handlers for the given message action.
	// To stop watching cancel the context.
	Watch(ctx context.Context, messages <-chan Message)
}

type eventHandler struct {
	handlers map[string]EventProcessor
	mu       sync.Mutex
}

func (eh *eventHandler) Handle(action string, ep EventProcessor) {
	eh.mu.Lock()
	eh.handlers[action] = ep
	eh.mu.Unlock()
}

func (eh *eventHandler) Get(action string) EventProcessor {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	return eh.handlers[action]
}

func (eh *eventHandler) Watch(ctx context.Context, messages <-chan Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-messages:
			if h := eh.Get(event.Action); h != nil {
				h(event)
			}
		}
	}
}

// NewEventHandler creates a new EventHandler
func NewEventHandler() EventHandler {
	return &eventHandler{
		handlers: make(map[string]EventProcessor),
	}
}
