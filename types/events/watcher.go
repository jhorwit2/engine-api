package events

import (
	"sync"

	goevents "github.com/docker/go-events"
	"golang.org/x/net/context"
)

// MatcherFunc return true if the event matches or false if not.
type MatcherFunc func(event Message) bool

// Watcher watches an event stream from the daemon providing
// an easy way to filter events to specific channels
type Watcher interface {
	// Watch all the events from the daemon.
	//
	// If matcher functions are specified then the channel will only
	// receive events that match any of the matchers.
	// For example, say you pass in two matchers: one for
	// label: foo and the other for label: bar. The channel returned
	// will receive events that have either label.
	//
	// Cancel the context to quit watching the events on this channel
	Watch(ctx context.Context, matcher ...MatcherFunc) <-chan Message

	// Stop will close every channel returned by watch. Stop will
	// not return until all channels have been closed succesfully.
	Stop()
}

type watcher struct {
	broadcast *goevents.Broadcaster
	events    <-chan Message
	buffer    int
	shutdown  chan struct{}
	wg        sync.WaitGroup
}

func (w *watcher) Watch(ctx context.Context, matchers ...MatcherFunc) <-chan Message {
	return w.createSinkWrapper(ctx, func(event goevents.Event) bool {
		if len(matchers) == 0 {
			return true
		}

		msg, ok := event.(Message)
		if !ok {
			return false
		}

		for _, matches := range matchers {
			if matches(msg) {
				return true
			}
		}
		return false
	})
}

func (w *watcher) createSinkWrapper(ctx context.Context, matcher goevents.MatcherFunc) <-chan Message {
	eventq := make(chan Message, w.buffer)
	ch := goevents.NewChannel(w.buffer)
	sink := goevents.Sink(goevents.NewQueue(ch))

	if matcher != nil {
		sink = goevents.NewFilter(sink, matcher)
	}

	w.wg.Add(1)
	cleanup := func() {
		w.wg.Done()
		close(eventq)
		w.broadcast.Remove(sink)
		ch.Close()
		sink.Close()
	}

	w.broadcast.Add(sink)

	go func() {
		defer cleanup()

		for {
			select {
			case <-w.shutdown:
				return
			case <-ctx.Done():
				return
			case e := <-ch.C:

				select {
				case <-ctx.Done():
					return
				case <-w.shutdown:
					return
				case eventq <- e.(Message):
				}
			}
		}
	}()

	return eventq
}

func (w *watcher) startWatching() {
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case <-w.shutdown:
			return
		case e, ok := <-w.events:
			if !ok {
				return
			}

			select {
			case <-w.shutdown:
				return
			default:
				w.broadcast.Write(e)
			}
		}
	}
}

func (w *watcher) Stop() {
	if w.shutdown != nil {
		close(w.shutdown)
		w.wg.Wait()
	}
}

// NewWatcher returns a new event watcher for the given channel.
// buffer should be specified to allow every channel returned by watch
// to buffer events.
//
// It's up to the caller to close the events channel
// and call Stop() in the event of an error or the stream is done.
func NewWatcher(events <-chan Message, buffer int) Watcher {
	w := &watcher{
		buffer:    buffer,
		broadcast: goevents.NewBroadcaster(),
		events:    events,
		shutdown:  make(chan struct{}),
	}
	go w.startWatching()
	return w
}
