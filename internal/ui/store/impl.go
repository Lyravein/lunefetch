package store

import (
	"sort"
	"strings"
	"sync"

	"github.com/lyravein/lunefetch/internal/storage"
)

// Ensure DownloadStore implements Store at compile time.
var _ Store = (*DownloadStore)(nil)

// Mutator is the interface that DownloadStore delegates mutations to.
// Implemented by pages.DownloadsPage — kept as an interface so Store
// does not import the pages package (would create a cycle).
type Mutator interface {
	StartDownload(id int64)
	PauseDownload(id int64)
	ResumeDownload(id int64)
	CancelDownload(id int64)
	DeleteDownload(id int64)
}

// Adder handles the Add mutation separately because it needs more
// context (HEAD request, DB write, chunk creation) than a simple id-based op.
// Implemented by layout.App.
type Adder interface {
	HandleAdd(req AddRequest)
}

// DownloadStore is the concrete implementation of Store.
type DownloadStore struct {
	sm      *storage.StateManager
	mutator Mutator
	adder   Adder

	mu         sync.RWMutex
	all        []storage.DownloadRecord  // raw from DB
	view       []*storage.DownloadRecord // filtered + sorted + searched
	selectedID int64

	filter   DownloadStatus
	category string
	search   string
	sortCol  TableColumn
	sortAsc  bool
}

// NewDownloadStore creates a new Store.
// mutator and adder are injected after construction via SetMutator/SetAdder
// to avoid circular initialization.
func NewDownloadStore(sm *storage.StateManager) *DownloadStore {
	return &DownloadStore{
		sm:      sm,
		sortCol: ColAdded,
		sortAsc: true,
	}
}

// SetMutator injects the mutation handler (pages.DownloadsPage).
func (s *DownloadStore) SetMutator(m Mutator) { s.mutator = m }

// SetAdder injects the add handler (layout.App).
func (s *DownloadStore) SetAdder(a Adder) { s.adder = a }

// --- Lifecycle ---

func (s *DownloadStore) Load() error {
	records, err := s.sm.ListDownloads()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.all = records
	s.mu.Unlock()
	s.rebuild()
	return nil
}

func (s *DownloadStore) Reload() error { return s.Load() }

// --- Read ---

func (s *DownloadStore) Downloads() []*storage.DownloadRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*storage.DownloadRecord, len(s.view))
	copy(out, s.view)
	return out
}

func (s *DownloadStore) Selected() *storage.DownloadRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.view {
		if s.view[i].ID == s.selectedID {
			return s.view[i]
		}
	}
	return nil
}

// --- View state ---

func (s *DownloadStore) SetFilter(status DownloadStatus) {
	s.mu.Lock()
	s.filter = status
	s.category = ""
	s.mu.Unlock()
	s.rebuild()
}

// SetCategory filters downloads by their assigned file category.
func (s *DownloadStore) SetCategory(category string) {
	s.mu.Lock()
	s.category = category
	s.filter = StatusAll
	s.mu.Unlock()
	s.rebuild()
}

func (s *DownloadStore) SetSearch(query string) {
	s.mu.Lock()
	s.search = strings.ToLower(query)
	s.mu.Unlock()
	s.rebuild()
}

func (s *DownloadStore) SetSort(col TableColumn, asc bool) {
	s.mu.Lock()
	s.sortCol = col
	s.sortAsc = asc
	s.mu.Unlock()
	s.rebuild()
}

func (s *DownloadStore) Select(id int64) {
	s.mu.Lock()
	s.selectedID = id
	s.mu.Unlock()
}

// --- Mutations ---

func (s *DownloadStore) Add(req AddRequest) {
	if s.adder != nil {
		s.adder.HandleAdd(req)
	}
	s.Load() //nolint:errcheck
}

func (s *DownloadStore) Pause(id int64) {
	if s.mutator != nil {
		s.mutator.PauseDownload(id)
	}
	s.Load() //nolint:errcheck
}

func (s *DownloadStore) Resume(id int64) {
	if s.mutator != nil {
		s.mutator.ResumeDownload(id)
	}
	s.Load() //nolint:errcheck
}

func (s *DownloadStore) Cancel(id int64) {
	if s.mutator != nil {
		s.mutator.CancelDownload(id)
	}
	s.Load() //nolint:errcheck
}

func (s *DownloadStore) Delete(id int64) {
	if s.mutator != nil {
		s.mutator.DeleteDownload(id)
	}
	s.Load() //nolint:errcheck
}

func (s *DownloadStore) Retry(id int64) {
	// Set status back to pending then re-enqueue.
	s.sm.UpdateDownloadStatus(id, "pending") //nolint:errcheck
	if s.mutator != nil {
		s.mutator.ResumeDownload(id)
	}
	s.Load() //nolint:errcheck
}

// --- Internal ---

// rebuild recomputes the view slice applying current filter, search, and sort.
// Must NOT be called while holding s.mu.
func (s *DownloadStore) rebuild() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []*storage.DownloadRecord
	for i := range s.all {
		r := &s.all[i]

		// Filter by status.
		if s.filter != StatusAll && DownloadStatus(r.Status) != s.filter {
			continue
		}
		if s.category != "" && r.Category != s.category {
			continue
		}

		// Filter by search query.
		if s.search != "" && !strings.Contains(strings.ToLower(r.Filename), s.search) {
			continue
		}

		filtered = append(filtered, r)
	}

	// Sort.
	col := s.sortCol
	asc := s.sortAsc
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		comparison := 0
		switch col {
		case ColName:
			comparison = strings.Compare(strings.ToLower(a.Filename), strings.ToLower(b.Filename))
		case ColSize:
			comparison = compareInt64(a.TotalSize, b.TotalSize)
		case ColProgress:
			pa := progressOf(a)
			pb := progressOf(b)
			if pa < pb {
				comparison = -1
			} else if pa > pb {
				comparison = 1
			}
		case ColStatus:
			comparison = strings.Compare(a.Status, b.Status)
		case ColAdded:
			comparison = a.CreatedAt.Compare(b.CreatedAt)
		default:
			comparison = a.CreatedAt.Compare(b.CreatedAt)
		}
		if comparison == 0 {
			comparison = compareInt64(a.ID, b.ID)
		}
		if asc {
			return comparison < 0
		}
		return comparison > 0
	})

	s.view = filtered
}

func compareInt64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func progressOf(r *storage.DownloadRecord) float64 {
	if r.TotalSize == 0 {
		return 0
	}
	return float64(r.DownloadedSize) / float64(r.TotalSize)
}
