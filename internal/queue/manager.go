package queue

import (
	"sync"

	"github.com/lyravein/lunefetch/internal/storage"
)

// StartFunc is called by the Manager to actually start a download.
// It is provided by the TUI model.
type StartFunc func(id int64)

// Manager controls how many downloads run concurrently and keeps the
// queue moving: when a slot opens up it pulls the next item from the DB
// queue and calls StartFunc.
type Manager struct {
	mu            sync.Mutex
	state         *storage.StateManager
	maxConcurrent int
	active        map[int64]struct{}
	startFn       StartFunc
}

func NewManager(state *storage.StateManager, maxConcurrent int, startFn StartFunc) *Manager {
	return &Manager{
		state:         state,
		maxConcurrent: maxConcurrent,
		active:        make(map[int64]struct{}),
		startFn:       startFn,
	}
}

// SetMaxConcurrent updates the concurrency limit at runtime.
func (m *Manager) SetMaxConcurrent(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n < 1 {
		n = 1
	}
	m.maxConcurrent = n
	m.drainQueue()
}

// TryStart attempts to start the download immediately.
// If the concurrency limit is reached, it enqueues the download instead.
// Returns true if the download was started, false if it was queued.
func (m *Manager) TryStart(id int64) (started bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.active[id]; exists {
		return true, nil
	}
	rec, err := m.state.GetDownload(id)
	if err != nil {
		return false, err
	}
	if rec == nil || rec.DeletedAt.Valid {
		return false, nil
	}

	if len(m.active) < m.maxConcurrent {
		m.active[id] = struct{}{}
		m.state.UpdateDownloadStatus(id, "downloading") //nolint:errcheck
		m.state.SetQueuePosition(id, nil)               //nolint:errcheck
		go m.startFn(id)
		return true, nil
	}

	// No slot available — put it at the end of the queue.
	if err := m.enqueue(id); err != nil {
		return false, err
	}
	return false, nil
}

// OnDone must be called when a download finishes (success, failure, or cancel).
// It decrements the active counter and starts the next queued item if any.
func (m *Manager) OnDone(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.active[id]; !exists {
		return
	}
	delete(m.active, id)
	m.state.SetQueuePosition(id, nil) //nolint:errcheck

	m.drainQueue()
}

// EnqueueScheduled moves a scheduled download into queue or starts it directly.
func (m *Manager) EnqueueScheduled(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.SetScheduledAt(id, nil) //nolint:errcheck

	if _, exists := m.active[id]; exists {
		return nil
	}
	if len(m.active) < m.maxConcurrent {
		m.active[id] = struct{}{}
		m.state.UpdateDownloadStatus(id, "downloading") //nolint:errcheck
		go m.startFn(id)
		return nil
	}

	return m.enqueue(id)
}

// Active returns the number of currently running downloads.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.active)
}

// Remove removes an inactive item from the persistent queue. Active workers
// keep owning their slot until their completion path calls OnDone.
func (m *Manager) Remove(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, active := m.active[id]; active {
		return
	}
	m.state.SetQueuePosition(id, nil) //nolint:errcheck
}

// Drain starts persisted queued downloads while capacity is available.
func (m *Manager) Drain() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.drainQueue()
}

// enqueue assigns the next available queue_position to id and sets status to "queued".
// Caller must hold m.mu.
func (m *Manager) enqueue(id int64) error {
	rec, err := m.state.GetDownload(id)
	if err != nil {
		return err
	}
	if rec == nil || rec.DeletedAt.Valid {
		return nil
	}
	if rec.Status == "queued" && rec.QueuePosition.Valid {
		return nil
	}
	row := m.state.DB().QueryRow(
		`SELECT COALESCE(MAX(queue_position), 0) FROM downloads WHERE status = 'queued'`,
	)
	var maxPos int64
	if err := row.Scan(&maxPos); err != nil {
		return err
	}
	pos := maxPos + 1
	if err := m.state.SetQueuePosition(id, &pos); err != nil {
		return err
	}
	return m.state.UpdateDownloadStatus(id, "queued")
}

// drainQueue starts the next queued download if a slot is available.
// Caller must hold m.mu.
func (m *Manager) drainQueue() {
	for len(m.active) < m.maxConcurrent {
		next, err := m.state.NextInQueue()
		if err != nil || next == nil {
			return
		}
		if _, exists := m.active[next.ID]; exists {
			m.state.SetQueuePosition(next.ID, nil) //nolint:errcheck
			continue
		}
		m.active[next.ID] = struct{}{}
		m.state.UpdateDownloadStatus(next.ID, "downloading") //nolint:errcheck
		m.state.SetQueuePosition(next.ID, nil)               //nolint:errcheck
		go m.startFn(next.ID)
	}
}
