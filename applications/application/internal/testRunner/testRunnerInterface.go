package testrunner

import (
	"context"
	"time"

	"github.com/egot3/fathom/internal/quiz"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
)

type TestRunner interface {
	start(ctx context.Context, duration time.Duration, quizPaths []string, quizUUIDs, groupUUIDs uuid.UUIDs, testUUID uuid.UUID, cleanup func()) error
	Get(quizUUID uuid.UUID) (*quiz.Quiz, error)
	Stop()
	ExtendTime(duration time.Duration) error
	Resume() error
	Pause() error
	IsPaused() bool
	Deadline() time.Time

	Groups() uuid.UUIDs
	Quizzes() uuid.UUIDs
	GetAll() uuid.UUIDs
	Checksum() uint64
	Test() uuid.UUID
}

var TestRunnerPackage = do.Package(
	do.Lazy(NewManager),
)
