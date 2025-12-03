package notifications

import (
	"context"
	"sync"
)

const (
	// DefaultQueueSize is the maximum number of events that can be queued.
	DefaultQueueSize = 100
)

// Manager coordinates notification delivery across multiple providers.
// It uses an async bounded queue to prevent blocking rotation operations.
type Manager struct {
	providers []NotificationProvider
	queue     chan RotationEvent
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
	done      chan struct{}

	// Metrics tracking
	droppedCount int64
	droppedMu    sync.Mutex
}

// NewManager creates a new notification manager with the specified queue size.
// If queueSize is 0, DefaultQueueSize is used.
func NewManager(queueSize int) *Manager {
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}
	return &Manager{
		providers: make([]NotificationProvider, 0),
		queue:     make(chan RotationEvent, queueSize),
		done:      make(chan struct{}),
	}
}

// RegisterProvider adds a notification provider to the manager.
func (m *Manager) RegisterProvider(provider NotificationProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, provider)
}

// Providers returns a copy of the registered providers.
func (m *Manager) Providers() []NotificationProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	providers := make([]NotificationProvider, len(m.providers))
	copy(providers, m.providers)
	return providers
}

// Start begins the background notification worker goroutine.
// This must be called before sending events.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	m.wg.Add(1)
	go m.worker(ctx)
}

// Stop gracefully shuts down the notification manager.
// It waits for pending notifications to be processed.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	close(m.done)
	m.wg.Wait()
}

// Send queues a rotation event for notification delivery.
// If the queue is full, the oldest event is dropped and the counter is incremented.
// This method never blocks - notifications are best-effort.
func (m *Manager) Send(event RotationEvent) {
	m.mu.RLock()
	if !m.running {
		m.mu.RUnlock()
		return
	}
	m.mu.RUnlock()

	select {
	case m.queue <- event:
		// Event queued successfully
	default:
		// Queue is full - drop the event and increment counter
		m.droppedMu.Lock()
		m.droppedCount++
		m.droppedMu.Unlock()

		// Increment Prometheus counter
		incrementDroppedCounter()
	}
}

// DroppedCount returns the number of events that were dropped due to queue overflow.
func (m *Manager) DroppedCount() int64 {
	m.droppedMu.Lock()
	defer m.droppedMu.Unlock()
	return m.droppedCount
}

// worker processes events from the queue and dispatches to providers.
func (m *Manager) worker(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - drain remaining events
			m.drainQueue(ctx)
			return
		case <-m.done:
			// Manager stopped - drain remaining events
			m.drainQueue(ctx)
			return
		case event, ok := <-m.queue:
			if !ok {
				return
			}
			m.dispatchEvent(ctx, event)
		}
	}
}

// drainQueue processes any remaining events in the queue.
func (m *Manager) drainQueue(ctx context.Context) {
	for {
		select {
		case event, ok := <-m.queue:
			if !ok {
				return
			}
			// Use a short timeout for draining
			drainCtx, cancel := context.WithTimeout(context.Background(), 5*1e9) // 5 seconds
			m.dispatchEvent(drainCtx, event)
			cancel()
		default:
			return
		}
	}
}

// dispatchEvent sends an event to all providers that support it.
func (m *Manager) dispatchEvent(ctx context.Context, event RotationEvent) {
	m.mu.RLock()
	providers := m.providers
	m.mu.RUnlock()

	for _, provider := range providers {
		if !provider.SupportsEvent(event.Type) {
			continue
		}

		// Send notification - errors are logged but don't fail the rotation
		if err := provider.Send(ctx, event); err != nil {
			// TODO: Add proper logging once integrated with logging package
			_ = err
		}
	}
}
