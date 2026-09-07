package testrunner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/hashutils"
	"github.com/egot3/fathom/internal/logging"
	"github.com/egot3/fathom/internal/quiz"
	quizparser "github.com/egot3/fathom/internal/quizParser"
	"github.com/google/uuid"
	"github.com/samber/lo"
)

var (
	ErrQuizNotCached   = errors.New("quiz not cached in runner")
	ErrRunnerInactive  = errors.New("runner not started")
	ErrRunnerPaused    = errors.New("runner is paused")
	ErrRunnerNotPaused = errors.New("runner wasn't paused")
	ErrRunnerExpired   = errors.New("runner expired")
	ErrBadQuizzes      = errors.New("quizUUIDs and quizPathes lens differ")
	ErrAlreadyRunning  = errors.New("test for this group is already running")
)

type NotCachedError struct {
	Count int
}

func (e *NotCachedError) Error() string {
	return fmt.Sprintf("%d quizzes not cached in runner", e.Count)
}

func (e *NotCachedError) Is(target error) bool {
	return target == ErrQuizNotCached
}

type concreteTestRunner struct {
	mu         sync.RWMutex
	quizzes    []quiz.Quiz
	timer      *time.Timer
	isPaused   bool
	deadline   time.Time
	pausedAt   time.Time
	checksum   uint64
	giveup     chan struct{}
	cleanup    func()
	groupUUIDs uuid.UUIDs
	testUUID   uuid.UUID
}

func (tr *concreteTestRunner) start(ctx context.Context, duration time.Duration, quizPaths []string, quizUUIDs, groupUUIDs uuid.UUIDs, testUUID uuid.UUID, cleanup func()) error {
	if len(quizPaths) != len(quizUUIDs) {
		return ErrBadQuizzes
	}

	logger := logging.LoggerFromContext(ctx).With(slog.String("layer", "runner"),
		slog.String("duration", duration.String()),
		slog.Any("quizPathes", quizPaths),
		slog.Any("quizUUIDs", quizUUIDs.Strings()),
	)

	logger.Debug("starting runner...")
	ultimateChecksum := make([]uint64, len(quizPaths))
	quizzes := make([]quiz.Quiz, len(quizPaths))
	for i, path := range quizPaths {
		err := func() error {
			if !filepath.IsAbs(path) {
				return carefulness.ErrAbsoluteRequired
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			quiz, err := quizparser.ParseQuiz(f)
			if err != nil {
				return fmt.Errorf("parsing quiz at %q: %w", path, err)
			}
			quiz.UUID = quizUUIDs[i]

			quiz.Checksum, err = hashutils.HashFile(f)
			if err != nil {
				return err
			}
			ultimateChecksum[i] = quiz.Checksum
			quizzes[i] = *quiz
			return nil
		}()
		if err != nil {
			return err
		}
	}

	tr.mu.Lock()

	if tr.timer != nil {
		tr.timer.Stop()
	}

	tr.testUUID = testUUID

	tr.checksum = hashutils.HashHashes(ultimateChecksum)
	tr.quizzes = quizzes
	tr.isPaused = false
	tr.deadline = time.Now().Add(duration)

	tr.timer = time.NewTimer(time.Until(tr.deadline))

	tr.giveup = make(chan struct{})
	stop := tr.giveup

	go func(t *time.Timer, stop <-chan struct{}, cleanup func()) {
		select {
		case <-t.C:
			cleanup()
		case <-stop:
			cleanup()
		}
		logger.Info("test stopped!")
	}(tr.timer, stop, cleanup)
	tr.mu.Unlock()

	return nil
}

func (tr *concreteTestRunner) Test() uuid.UUID {
	return tr.testUUID
}

func (tr *concreteTestRunner) Get(quizUUID uuid.UUID) (*quiz.Quiz, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if tr.giveup == nil {
		return nil, ErrRunnerInactive
	}

	q, ok := lo.Find(tr.quizzes, func(quiz quiz.Quiz) bool {
		return quiz.UUID == quizUUID
	})
	if !ok {
		return nil, ErrQuizNotCached
	}
	return &q, nil
}

func (tr *concreteTestRunner) Stop() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.giveup != nil {
		close(tr.giveup)
		tr.giveup = nil
	}
	if tr.timer != nil {
		tr.timer.Stop()
	}
	tr.quizzes = nil
}

// acquire path quiz pairs from db via uuid
// called upsert as maps.Copy overwrites dest on collision
func (tr *concreteTestRunner) UpsertQuiz(quizPaths []string, quizUUIDs uuid.UUIDs) error {
	if len(quizPaths) != len(quizUUIDs) {
		return ErrBadQuizzes
	}

	tr.mu.RLock()
	if tr.giveup == nil { // reading a bunch of files might fry the potato
		tr.mu.RUnlock()
		return ErrRunnerInactive
	}
	tr.mu.RUnlock()

	quizzes := make([]quiz.Quiz, len(quizPaths))
	for i, path := range quizPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("unsupported path scheme %q: only abs paths are currently supported", path) //registry is not implemented
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		quiz, err := quizparser.ParseQuiz(f)
		if err != nil {
			return fmt.Errorf("parsing quiz at %q: %w", path, err)
		}
		quiz.UUID = quizUUIDs[i]
		quizzes[i] = *quiz

	}

	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.giveup == nil { //TOCTOU
		return ErrRunnerInactive
	}
	tr.quizzes = append(tr.quizzes, quizzes...)

	ultimateChecksum := make([]uint64, len(tr.quizzes))
	lo.ForEach(tr.quizzes, func(quiz quiz.Quiz, _ int) {
		ultimateChecksum = append(ultimateChecksum, quiz.Checksum)
	})

	tr.checksum = hashutils.HashHashes(ultimateChecksum)

	return nil
}

func (tr *concreteTestRunner) RemoveQuiz(uuids uuid.UUIDs) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.giveup == nil {
		return ErrRunnerInactive
	}

	oldL := len(tr.quizzes)
	tr.quizzes = lo.Filter(tr.quizzes, func(quiz quiz.Quiz, _ int) bool {
		return !slices.Contains(uuids, quiz.UUID)
	})

	ultimateChecksum := make([]uint64, len(tr.quizzes))
	lo.ForEach(tr.quizzes, func(quiz quiz.Quiz, _ int) {
		ultimateChecksum = append(ultimateChecksum, quiz.Checksum)
	})

	tr.checksum = hashutils.HashHashes(ultimateChecksum)

	nf := len(uuids) - (oldL - len(tr.quizzes))

	if nf != 0 {
		return &NotCachedError{Count: nf}
	}

	return nil
}

func (tr *concreteTestRunner) ExtendTime(duration time.Duration) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.giveup == nil {
		return ErrRunnerInactive
	}

	tr.deadline = tr.deadline.Add(duration)
	if !tr.isPaused {
		if !tr.timer.Stop() {
			return ErrRunnerNotPaused
		}
		tr.timer.Reset(time.Until(tr.deadline))
	}

	return nil
}

// doesn't lock the runner for the whole pause letting dialer know what's up
func (tr *concreteTestRunner) Pause() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.giveup == nil {
		return ErrRunnerInactive
	}
	if tr.isPaused {
		return ErrRunnerPaused
	}
	if !tr.timer.Stop() {
		return ErrRunnerExpired //expiration
	}

	tr.isPaused = true
	tr.pausedAt = time.Now()

	return nil
}

func (tr *concreteTestRunner) Resume() error {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.giveup == nil {
		return ErrRunnerInactive
	}
	if !tr.isPaused {
		return ErrRunnerNotPaused
	}

	sincePause := time.Since(tr.pausedAt)
	tr.deadline = tr.deadline.Add(sincePause)

	if sincePause <= 0 {
		tr.giveup <- struct{}{}
		tr.giveup = nil
		tr.quizzes = nil
		tr.isPaused = false
		return ErrRunnerInactive
	}

	if time.Until(tr.deadline) <= 0 {
		tr.giveup <- struct{}{}
		tr.giveup = nil
		tr.quizzes = nil
		tr.isPaused = false
		return ErrRunnerExpired
	}

	tr.timer.Reset(sincePause)

	tr.isPaused = false
	return nil
}

func (tr *concreteTestRunner) IsPaused() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.isPaused
}

func (tr *concreteTestRunner) Deadline() time.Time {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.deadline
}

func (tr *concreteTestRunner) GetAll() uuid.UUIDs {
	tr.mu.RLock()
	quizzesLocal := tr.quizzes
	tr.mu.RUnlock()

	quizzes := lo.Map(quizzesLocal, func(quiz quiz.Quiz, _ int) uuid.UUID {
		return quiz.UUID
	})

	return quizzes
}

func (tr *concreteTestRunner) Checksum() uint64 {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.checksum
}

func (tr *concreteTestRunner) Quizzes() uuid.UUIDs {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return lo.Map(tr.quizzes, func(item quiz.Quiz, _ int) uuid.UUID {
		return item.UUID
	})
}

func (tr *concreteTestRunner) Groups() uuid.UUIDs {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.groupUUIDs
}
