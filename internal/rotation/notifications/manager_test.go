package notifications

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a test double for NotificationProvider
type fakeProvider struct {
	name          string
	supportedEvts []EventType
	sendFunc      func(ctx context.Context, event RotationEvent) error
	mu            sync.Mutex
	sentEvents    []RotationEvent
	sendDelay     time.Duration
}

func newFakeProvider(name string) *fakeProvider {
	return &fakeProvider{
		name:          name,
		supportedEvts: AllEventTypes(),
		sentEvents:    make([]RotationEvent, 0),
	}
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) SupportsEvent(eventType EventType) bool {
	for _, e := range p.supportedEvts {
		if e == eventType {
			return true
		}
	}
	return false
}

func (p *fakeProvider) Validate(ctx context.Context) error { return nil }

func (p *fakeProvider) Send(ctx context.Context, event RotationEvent) error {
	if p.sendDelay > 0 {
		select {
		case <-time.After(p.sendDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if p.sendFunc != nil {
		return p.sendFunc(ctx, event)
	}

	p.mu.Lock()
	p.sentEvents = append(p.sentEvents, event)
	p.mu.Unlock()
	return nil
}

func (p *fakeProvider) getSentEvents() []RotationEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	events := make([]RotationEvent, len(p.sentEvents))
	copy(events, p.sentEvents)
	return events
}

func TestManager_NewManager(t *testing.T) {
	t.Parallel()

	t.Run("default queue size", func(t *testing.T) {
		t.Parallel()
		m := NewManager(0)
		assert.NotNil(t, m)
	})

	t.Run("custom queue size", func(t *testing.T) {
		t.Parallel()
		m := NewManager(50)
		assert.NotNil(t, m)
	})
}

func TestManager_RegisterProvider(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	p1 := newFakeProvider("test1")
	p2 := newFakeProvider("test2")

	m.RegisterProvider(p1)
	m.RegisterProvider(p2)

	providers := m.Providers()
	assert.Len(t, providers, 2)
}

func TestManager_StartStop(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.Start(ctx)
	// Start again should be a no-op
	m.Start(ctx)

	m.Stop()
	// Stop again should be a no-op
	m.Stop()
}

func TestManager_Send_QueuesEvents(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	provider := newFakeProvider("test")
	m.RegisterProvider(provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "test-service",
		Timestamp: time.Now(),
	}

	m.Send(event)

	// Give the worker time to process
	time.Sleep(100 * time.Millisecond)

	m.Stop()

	events := provider.getSentEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "test-service", events[0].Service)
}

func TestManager_Send_MultipleProviders(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	p1 := newFakeProvider("provider1")
	p2 := newFakeProvider("provider2")
	m.RegisterProvider(p1)
	m.RegisterProvider(p2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "multi-test",
		Timestamp: time.Now(),
	}

	m.Send(event)

	time.Sleep(100 * time.Millisecond)
	m.Stop()

	assert.Len(t, p1.getSentEvents(), 1)
	assert.Len(t, p2.getSentEvents(), 1)
}

func TestManager_Send_FiltersByEventType(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	provider := newFakeProvider("test")
	// Only support completed events
	provider.supportedEvts = []EventType{EventTypeCompleted}
	m.RegisterProvider(provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Send a failed event (not supported)
	failedEvent := RotationEvent{
		Type:      EventTypeFailed,
		Service:   "failed-service",
		Timestamp: time.Now(),
	}
	m.Send(failedEvent)

	// Send a completed event (supported)
	completedEvent := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "completed-service",
		Timestamp: time.Now(),
	}
	m.Send(completedEvent)

	time.Sleep(100 * time.Millisecond)
	m.Stop()

	events := provider.getSentEvents()
	require.Len(t, events, 1)
	assert.Equal(t, "completed-service", events[0].Service)
}

func TestManager_Send_DropsWhenQueueFull(t *testing.T) {
	t.Parallel()

	// Create a manager with a tiny queue
	m := NewManager(2)

	// Provider with a delay to cause queue backup
	provider := newFakeProvider("slow")
	provider.sendDelay = 100 * time.Millisecond
	m.RegisterProvider(provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Send many events to fill the queue
	for i := 0; i < 10; i++ {
		m.Send(RotationEvent{
			Type:      EventTypeCompleted,
			Service:   "burst-service",
			Timestamp: time.Now(),
		})
	}

	// Some events should have been dropped
	time.Sleep(500 * time.Millisecond)
	m.Stop()

	dropped := m.DroppedCount()
	assert.Greater(t, dropped, int64(0), "Some events should have been dropped")
}

func TestManager_Send_NotRunning(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	provider := newFakeProvider("test")
	m.RegisterProvider(provider)

	// Don't start the manager
	event := RotationEvent{
		Type:      EventTypeCompleted,
		Service:   "no-start",
		Timestamp: time.Now(),
	}
	m.Send(event)

	// No events should be sent
	assert.Empty(t, provider.getSentEvents())
}

func TestManager_ContextCancellation(t *testing.T) {
	t.Parallel()

	m := NewManager(10)
	provider := newFakeProvider("test")
	m.RegisterProvider(provider)

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)

	// Cancel context
	cancel()

	// Give the worker time to exit
	time.Sleep(100 * time.Millisecond)

	// Manager should handle cancellation gracefully
	// (no assertion needed - just ensuring no panic/hang)
}
