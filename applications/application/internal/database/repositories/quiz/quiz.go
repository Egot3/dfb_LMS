package quiz

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/quiz"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
)

type bunQuizRepository struct {
	db *bun.DB
}

func NewQuizRepository(i do.Injector) (QuizRepository, error) {
	db := do.MustInvoke[*bun.DB](i)
	return &bunQuizRepository{db: db}, nil
}

func (r *bunQuizRepository) QuizPath(ctx context.Context, quizUUID uuid.UUID) (string, error) {
	var path string
	err := r.db.NewSelect().Model(&models.Quiz{UUID: quizUUID}).
		WherePK().Column("path").Scan(ctx, &path)
	if err != nil {
		return "", err
	}

	return path, nil
}

func (r *bunQuizRepository) RegisterQuiz(ctx context.Context, path string, checksum [8]byte, score int, answer []byte) error {
	q := models.Quiz{
		Path:          path,
		Checksum:      checksum,
		Score:         score,
		CorrectAnswer: string(answer),
	}
	err := r.db.NewInsert().
		Model(&q).
		Returning("*").Ignore().
		Scan(ctx)
	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return carefulness.Conflict{Conflictor: "Path"}
		}
		return err
	}
	return nil
}

func (r *bunQuizRepository) DeallocateQuiz(ctx context.Context, quizUUID uuid.UUID) error {
	res, err := r.db.NewDelete().Model(&models.Quiz{UUID: quizUUID}).WherePK().Exec(ctx)
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

func (r *bunQuizRepository) ListQuizzes(ctx context.Context, page, size int) ([]models.Quiz, int, error) {
	var quizzes []models.Quiz
	total, err := r.db.NewSelect().Model(&quizzes).
		Offset(page*size).Limit(size).
		OrderBy("path", bun.OrderDesc).ScanAndCount(ctx)
	if err != nil {
		return nil, total, err
	}

	return quizzes, total, nil
}

func (r *bunQuizRepository) CheckRegistered(ctx context.Context, path string) (bool, error) {
	return r.db.NewSelect().Model((*models.Quiz)(nil)).Where("path = ?", path).Exists(ctx)
}

func (r *bunQuizRepository) CheckIntegrity(ctx context.Context, path string, checksum [8]byte) (bool, error) {
	return r.db.NewSelect().Model((*models.Quiz)(nil)).Where("path = ?", path).Where("checksum = ?", checksum).Exists(ctx)
}

func (r *bunQuizRepository) UpdateChecksum(ctx context.Context, quizUUID uuid.UUID, checksum [8]byte) error {
	_, err := r.db.NewUpdate().Model((*models.Quiz)(nil)).
		Where("uuid = ?", quizUUID).Set("checksum = ?", checksum).Exec(ctx)

	return err
}

func (r *bunQuizRepository) PatchQuiz(ctx context.Context, quizUUID uuid.UUID, path *string, score *int, answer *quiz.QuizAnswers, checksum *[8]byte) error {
	q := r.db.NewUpdate().Model((*models.Quiz)(nil)).
		Where("uuid = ?", quizUUID)

	if path != nil {
		q = q.Set("path = ?", path)
	}
	if score != nil {
		q = q.Set("score = ?", score)
	}
	if answer != nil {
		ans, err := json.Marshal(answer)
		if err != nil {
			return err
		}

		q = q.Set("correct_answer = ?", string(ans))
	}
	if checksum != nil {
		q = q.Set("checksum = ?", checksum)
	}

	_, err := q.Exec(ctx)
	return err
}

func (r *bunQuizRepository) CorrectAnswer(ctx context.Context, quizUUID uuid.UUID) (string, error) {
	var answer string
	err := r.db.NewSelect().Model((*models.Quiz)(nil)).
		Where("uuid = ?", quizUUID).Column("correct_answer").
		Scan(ctx, &answer)
	if err != nil {
		return "", err
	}

	return answer, nil
}

func (r *bunQuizRepository) ExistsByUUID(ctx context.Context, quizUUID uuid.UUID) (bool, error) {
	return r.db.NewSelect().Model((*models.Quiz)(nil)).
		Where("uuid = ?", quizUUID).Exists(ctx)
}

func (r *bunQuizRepository) QuizFresh(ctx context.Context, quizUUID uuid.UUID, checksum [8]byte) (bool, error) {
	return r.db.NewSelect().Model((*models.Quiz)(nil)).
		Where("uuid = ?", quizUUID).Where("checksum = ?", checksum).Exists(ctx)
}

func (r *bunQuizRepository) Quiz(ctx context.Context, quizUUID uuid.UUID) (models.Quiz, error) {
	quiz := models.Quiz{UUID: quizUUID}
	err := r.db.NewSelect().Model(&quiz).
		WherePK().Scan(ctx)
	if err != nil {
		return models.Quiz{}, err
	}

	return quiz, nil
}
