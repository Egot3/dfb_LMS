package test

import (
	"context"

	exportutlis "github.com/egot3/fathom/internal/exportUtlis"
	"github.com/egot3/fathom/internal/models"
	"github.com/google/uuid"
)

type TestRepository interface {
	CreateTest(ctx context.Context, name string) (models.Test, error)
	BundleQuizzesToTest(ctx context.Context, testUUID uuid.UUID, quizUUIDs uuid.UUIDs) error
	PruneQuizzesFromTest(ctx context.Context, testUUID uuid.UUID, quizUUIDs uuid.UUIDs) error
	Test(ctx context.Context, UUID uuid.UUID) (models.Test, error)
	Tests(ctx context.Context, UUIDs uuid.UUIDs) ([]models.Test, error)
	DeleteTest(ctx context.Context, UUID uuid.UUID) error
	UpdateTest(ctx context.Context, UUID uuid.UUID, name string) error
	TestPathes(ctx context.Context, UUIDs uuid.UUIDs) ([]string, error)
	ListTests(ctx context.Context, page, size int) ([]models.Test, int, error)
	ExistsByUUID(ctx context.Context, testUUID uuid.UUID) (bool, error)
	ImportTest(ctx context.Context, test exportutlis.YamlTest) error
	ListTestsAdvanced(ctx context.Context, page, size int) ([]models.Test, int, error)
}
