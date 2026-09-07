package test

import (
	"context"
	"database/sql"
	"errors"

	"github.com/egot3/fathom/internal/carefulness"
	exportutlis "github.com/egot3/fathom/internal/exportUtlis"
	"github.com/egot3/fathom/internal/models"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/uptrace/bun"
)

type bunTestRepository struct {
	db *bun.DB
}

func NewTestRepository(i do.Injector) (TestRepository, error) {
	db := do.MustInvoke[*bun.DB](i)
	return &bunTestRepository{db: db}, nil
}

func (r *bunTestRepository) TestPathes(ctx context.Context, UUIDs uuid.UUIDs) ([]string, error) {
	pathes := make([]string, len(UUIDs))
	err := r.db.NewSelect().Model((*models.Quiz)(nil)).
		Column("path").
		Where("uuid IN (?)", bun.List(UUIDs)).
		Scan(ctx, pathes)
	if err != nil {
		return nil, err
	}

	if len(UUIDs) != len(pathes) {
		return nil, sql.ErrNoRows
	}

	return pathes, nil
}

func (r *bunTestRepository) CreateTest(ctx context.Context, name string) (models.Test, error) {
	var test = models.Test{Name: name}
	err := r.db.NewInsert().Ignore().Model(&test).Returning("name").Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Test{}, &carefulness.Conflict{Conflictor: "name"}
		}
		return models.Test{}, err
	}

	return test, nil
}

func (r *bunTestRepository) BundleQuizzesToTest(ctx context.Context, testUUID uuid.UUID, quizUUIDs uuid.UUIDs) error {
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		var testQuizzes []models.TestsQuizzes = lo.Map(quizUUIDs, func(u uuid.UUID, pos int) models.TestsQuizzes {
			return models.TestsQuizzes{TestUUID: testUUID, Position: pos, QuizUUID: u}
		})
		_, err := tx.NewInsert().
			Model(&testQuizzes).Ignore().
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}

// new(alpha)
func (r *bunTestRepository) PruneQuizzesFromTest(ctx context.Context, testUUID uuid.UUID, quizUUIDs uuid.UUIDs) error {
	notFound := 0

	res, err := r.db.NewDelete().Model((*models.TestsQuizzes)(nil)).
		Where("test_uuid = ?", testUUID).
		Where("quiz_uuid IN (?)", bun.List(quizUUIDs)).
		Exec(ctx)
	if err != nil {
		return err
	}
	c, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if c == 0 {
		return sql.ErrNoRows
	}

	notFound = len(quizUUIDs) - int(c)

	if notFound > 0 {
		return &carefulness.NotInTestError{Count: notFound}
	}

	return nil
}

func (r *bunTestRepository) Test(ctx context.Context, UUID uuid.UUID) (models.Test, error) {
	var test = models.Test{UUID: UUID}
	err := r.db.NewSelect().Model(&test).WherePK().Relation("Quizzes").Scan(ctx)
	if err != nil {
		return models.Test{}, err
	}

	return test, nil
}

func (r *bunTestRepository) Tests(ctx context.Context, UUIDs uuid.UUIDs) ([]models.Test, error) {
	tests := make([]models.Test, len(UUIDs))
	err := r.db.NewSelect().Model(&tests).Where("uuid IN (?)", bun.List(UUIDs)).Relation("Quizzes").Scan(ctx)
	if err != nil {
		return nil, err
	}

	return tests, nil
}

func (r *bunTestRepository) DeleteTest(ctx context.Context, UUID uuid.UUID) error {
	res, err := r.db.NewDelete().Model(&models.Test{UUID: UUID}).WherePK().Exec(ctx)

	c, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if c == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *bunTestRepository) UpdateTest(ctx context.Context, UUID uuid.UUID, name string) error {
	e, err := r.db.NewSelect().Model((*models.Test)(nil)).Where("name = ?", name).Exists(ctx)
	if err != nil {
		return err
	}
	if e {
		return carefulness.Conflict{Conflictor: "name"}
	}

	res, err := r.db.NewUpdate().Model(&models.Test{UUID: UUID, Name: name}).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	c, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if c == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *bunTestRepository) ListTests(ctx context.Context, page, size int) ([]models.Test, int, error) {
	tests := make([]models.Test, size)
	total, err := r.db.NewSelect().Model(&tests).OrderBy("uuid", bun.OrderAsc).
		Limit(size).Offset(page * size).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return tests, total, nil
}

func (r *bunTestRepository) ListTestsAdvanced(ctx context.Context, page, size int) ([]models.Test, int, error) {
	tests := make([]models.Test, size)
	total, err := r.db.NewSelect().Model(&tests).
		Relation("Quizzes").
		OrderBy("uuid", bun.OrderAsc).
		Limit(size).Offset(page * size).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	return tests, total, nil
}

func (r *bunTestRepository) ExistsByUUID(ctx context.Context, testUUID uuid.UUID) (bool, error) {
	return r.db.NewSelect().Model((*models.Test)(nil)).
		Where("uuid = ?", testUUID).Exists(ctx)
}

func (r *bunTestRepository) ImportTest(ctx context.Context, test exportutlis.YamlTest) error {
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewInsert().Model(&models.Test{
			UUID: test.UUID,
			Name: test.Name,
		}).Exec(ctx)
		if err != nil {
			return err
		}

		for _, q := range test.Quizzes {
			_, err := tx.NewInsert().Model(&models.TestsQuizzes{
				QuizUUID: q.UUID,
				TestUUID: test.UUID,
			}).Exec(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
