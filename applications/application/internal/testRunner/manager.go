package testrunner

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egot3/fathom/internal/hashutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/zeebo/xxh3"
)

type Manager struct {
	mu      sync.RWMutex
	runners map[uint64]TestRunner
}

func (m *Manager) Start(ctx context.Context, duration time.Duration,
	quizPaths []string, quizUUIDs, groupUUIDs uuid.UUIDs, testUUID uuid.UUID,
) (TestRunner, error) {
	key := deriveKey(groupUUIDs, testUUID)

	m.mu.Lock()
	if _, exists := m.runners[key]; exists {
		m.mu.Unlock()
		return nil, ErrAlreadyRunning
	}
	tr := &concreteTestRunner{}
	m.runners[key] = tr
	m.mu.Unlock()

	if err := tr.start(ctx, duration, quizPaths, quizUUIDs, groupUUIDs, testUUID,
		func() { m.remove(key, tr) }); err != nil {
		m.remove(key, tr)
		return nil, err
	}
	return tr, nil
}

func (m *Manager) remove(key uint64, tr *concreteTestRunner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.runners[key]; ok && cur == tr {
		delete(m.runners, key)
	}
}

func (m *Manager) Get(key uint64) (TestRunner, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tr, ok := m.runners[key]
	return tr, ok
}

// deterministic key derivement
func deriveKey(groupUUIDs uuid.UUIDs, testUUID uuid.UUID) uint64 {
	grStrings := groupUUIDs.Strings()
	slices.SortFunc(grStrings, strings.Compare)

	checksums := make([]uint64, len(groupUUIDs)+1)
	for i, str := range grStrings {
		checksums[i] = xxh3.HashString(str)
	}
	checksums[len(checksums)-1] = xxh3.Hash(testUUID[:])
	return hashutils.HashHashes(checksums)
}

func (m *Manager) GetAll() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return slices.Collect(maps.Keys(m.runners))
}

func (m *Manager) IsQuizRunning(searchedUUID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, runner := range m.runners {
		for _, quiz := range runner.Quizzes() {
			if quiz == searchedUUID {
				return true
			}
		}
	}
	return false
}

func (m *Manager) AllTests() uuid.UUIDs {
	m.mu.RLock()
	defer m.mu.RUnlock()

	testUUIDs := make(uuid.UUIDs, 0, len(m.runners))
	for _, r := range m.runners {
		testUUIDs = append(testUUIDs, r.Test())
	}

	return testUUIDs
}

func NewManager(i do.Injector) (*Manager, error) {
	return &Manager{
		mu:      sync.RWMutex{},
		runners: map[uint64]TestRunner{},
	}, nil
}
