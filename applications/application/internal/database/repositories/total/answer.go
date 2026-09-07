package total

import (
	"context"
	"database/sql"
	"log"

	"github.com/egot3/fathom/internal/contracts"
	"github.com/egot3/fathom/internal/models"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
)

type bunTotalRepository struct {
	db *bun.DB
}

func NewTotalRepository(i do.Injector) (TotalRepository, error) {
	db := do.MustInvoke[*bun.DB](i)
	return &bunTotalRepository{db: db}, nil
} //created all of it just to live a good life in tests

func (r *bunTotalRepository) SetAnswer(ctx context.Context, testUUID, groupUUID, userUUID, quizUUID uuid.UUID, answerValue string, score float32) error {
	e, err := r.db.NewSelect().Model(&models.User{
		UUID: userUUID,
	}).WherePK().Exists(ctx)
	if err != nil {
		return err
	}
	if !e {
		log.Printf("user not found")
		return sql.ErrNoRows
	}

	e, err = r.db.NewSelect().Model(&models.Group{
		UUID: groupUUID,
	}).WherePK().Exists(ctx)
	if err != nil {
		return err
	}
	if !e {
		log.Printf("group not found")
		return sql.ErrNoRows
	}

	_, err = r.db.NewInsert().On("CONFLICT DO UPDATE").Model(&models.Answer{
		TestUUID:    testUUID,
		UserUUID:    userUUID,
		QuizUUID:    quizUUID,
		GroupUUID:   groupUUID,
		AnswerValue: answerValue,
		Score:       score,
	}).Exec(ctx)
	return err
}

func (r *bunTotalRepository) Answer(ctx context.Context, userUUID, testUUID, groupUUID, quizUUID uuid.UUID) (string, error) {
	var answer string
	err := r.db.NewSelect().
		Model((*models.Answer)(nil)).
		Where("test_uuid = ?", testUUID).
		Where("group_uuid = ?", groupUUID).
		Where("user_uuid = ?", userUUID).
		Where("quiz_uuid = ?", quizUUID).
		OrderBy("answered_at", bun.OrderDesc).
		Limit(1).
		Column("answer_value").Scan(ctx, &answer)
	if err != nil {
		return "", err
	}

	return answer, nil
}

func (r *bunTotalRepository) Totalize(ctx context.Context, userUUID, testUUID, groupUUID uuid.UUID) error {
	var userTotal float64
	var quizUUIDs uuid.UUIDs
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		e, err := tx.NewSelect().Model(&models.User{UUID: userUUID}).
			WherePK().Exists(ctx)
		if err != nil {
			return err
		}
		if !e {
			return sql.ErrNoRows
		}

		e, err = tx.NewSelect().Model(&models.Group{UUID: groupUUID}).
			WherePK().Exists(ctx)
		if err != nil {
			return err
		}
		if !e {
			return sql.ErrNoRows
		}

		err = tx.NewSelect().TableExpr("tests_quizzes AS tq").
			Column("tq.quiz_uuid").
			Where("tq.test_uuid = ?", testUUID).
			Scan(ctx, &quizUUIDs)
		if err != nil {
			return err
		}

		if len(quizUUIDs) == 0 {
			userTotal = 0
		} else {
			latestPerQuiz := tx.NewSelect().
				TableExpr("users_groups_tests_quiz_answers AS inner_a").
				ColumnExpr("MAX(inner_a.answered_at)").
				Where("inner_a.test_uuid = a.test_uuid").
				Where("inner_a.group_uuid = a.group_uuid").
				Where("inner_a.user_uuid = a.user_uuid").
				Where("inner_a.quiz_uuid = a.quiz_uuid")

			err = tx.NewSelect().
				TableExpr("users_groups_tests_quiz_answers AS a").
				ColumnExpr("COALESCE(SUM(a.score * 1.0), 0.0)").
				Where("a.test_uuid = ?", testUUID).
				Where("a.group_uuid = ?", groupUUID).
				Where("a.user_uuid = ?", userUUID).
				Where("a.quiz_uuid IN (?)", bun.List(quizUUIDs)).
				Where("a.answered_at = (?)", latestPerQuiz).
				Scan(ctx, &userTotal)
			if err != nil {
				return err
			}
		}

		_, err = tx.NewInsert().On("CONFLICT (test_uuid, group_uuid, user_uuid) DO UPDATE").
			Set("score = EXCLUDED.score").
			Model(&models.UserGroupsTests{UserUUID: userUUID, GroupUUID: groupUUID, TestUUID: testUUID, Score: userTotal}).
			Exec(ctx)

		return err
	})
}

func (r *bunTotalRepository) AnswerScore(ctx context.Context, userUUID, testUUID, groupUUID, quizUUID uuid.UUID) (float32, error) {
	var score float32
	err := r.db.NewSelect().
		Model((*models.Answer)(nil)).
		Where("test_uuid = ?", testUUID).
		Where("group_uuid = ?", groupUUID).
		Where("user_uuid = ?", userUUID).
		Where("quiz_uuid = ?", quizUUID).
		OrderBy("answered_at", bun.OrderDesc).
		Limit(1).
		Column("score").Scan(ctx, &score)
	if err != nil {
		return 0, err
	}

	return score, nil
}

func (r *bunTotalRepository) Total(ctx context.Context, userUUID, testUUID, groupUUID uuid.UUID) (contracts.Total, error) {
	exists, err := r.db.NewSelect().
		TableExpr("users_groups_tests AS ugt").
		Where("ugt.test_uuid = ?", testUUID).
		Where("ugt.user_uuid = ?", userUUID).
		Where("ugt.group_uuid = ?", groupUUID).
		Exists(ctx)
	if err != nil {
		return contracts.Total{}, err
	}
	if !exists {
		return contracts.Total{}, sql.ErrNoRows
	}

	total := contracts.Total{}
	err = r.db.NewSelect().TableExpr("users_groups_tests AS ugt").
		Where("ugt.test_uuid = ?", testUUID).
		Where("ugt.user_uuid = ?", userUUID).
		Where("ugt.group_uuid = ?", groupUUID).
		Join("LEFT JOIN users_groups_tests_quiz_answers AS ugtqa").
		JoinOn("ugt.test_uuid = ugtqa.test_uuid").JoinOn("ugt.group_uuid = ugtqa.group_uuid").JoinOn("ugt.user_uuid = ugtqa.user_uuid").
		Join("LEFT JOIN quizzes AS q").JoinOn("q.uuid = ugtqa.quiz_uuid").
		Join("JOIN groups AS g").JoinOn("g.uuid = ugt.group_uuid").ColumnExpr("g.name AS group_name").ColumnExpr("g.uuid AS group_uuid").
		Join("JOIN tests AS t").JoinOn("t.uuid = ugt.test_uuid").ColumnExpr("t.name AS test_name").ColumnExpr("t.uuid AS test_uuid").
		Join("JOIN users AS u").JoinOn("u.uuid = ugt.user_uuid").ColumnExpr("u.nickname AS user_name").ColumnExpr("u.uuid AS user_uuid").
		ColumnExpr("ugt.score AS score").
		ColumnExpr("ugt.finalized_at AS finalized_at").
		ColumnExpr("COALESCE(SUM(q.score), 0) AS max_score").
		Scan(ctx, &total)
	if err != nil {
		return contracts.Total{}, err
	}
	return total, nil
}

func (r *bunTotalRepository) UserTotals(ctx context.Context, userUUID uuid.UUID, page, size int) ([]contracts.Total, int, error) {
	var totals []contracts.Total
	total, err := r.db.NewSelect().TableExpr("users_groups_tests AS ugt").
		Where("ugt.user_uuid = ?", userUUID).
		ColumnExpr("ugt.score AS score, ugt.test_uuid AS test_uuid, ugt.group_uuid AS group_uuid").
		Join("JOIN groups AS g").JoinOn("g.uuid = ugt.group_uuid").ColumnExpr("g.name AS group_name").
		Join("JOIN tests AS t").JoinOn("t.uuid = ugt.test_uuid").ColumnExpr("t.name AS test_name").
		OrderBy("ugt.finalized_at", bun.OrderDesc).
		Offset(page*size).Limit(size).
		ScanAndCount(ctx, &totals)
	if err != nil {
		return nil, 0, err
	}

	if len(totals) == 0 {
		return nil, 0, sql.ErrNoRows
	}

	return totals, total, nil
}

func (r *bunTotalRepository) TestTotals(ctx context.Context, testUUID uuid.UUID) ([]contracts.Total, error) {
	var totals []contracts.Total
	err := r.db.NewSelect().Model((*models.UserGroupsTests)(nil)).
		Where("test_uuid = ?", testUUID).Scan(ctx, &totals)
	if err != nil {
		return nil, err
	}

	if len(totals) == 0 {
		return nil, sql.ErrNoRows
	}

	return totals, nil
}

func (r *bunTotalRepository) GroupTestTotals(ctx context.Context, testUUID, groupUUID uuid.UUID) ([]contracts.Total, error) {
	var totals []contracts.Total
	err := r.db.NewSelect().Model((*models.UserGroupsTests)(nil)).
		Where("test_uuid = ?", testUUID).
		Where("group_uuid = ?", groupUUID).Scan(ctx, &totals)
	if err != nil {
		return nil, err
	}
	if len(totals) == 0 {
		return nil, sql.ErrNoRows
	}

	return totals, nil
}

func (r *bunTotalRepository) ListTotals(ctx context.Context, page int, size int) ([]contracts.Total, int, error) {
	var totals []contracts.Total
	total, err := r.db.NewSelect().TableExpr("users_groups_tests AS ugt").
		ColumnExpr("ugt.score AS score, ugt.test_uuid AS test_uuid, ugt.group_uuid AS group_uuid, ugt.user_uuid AS user_uuid, ugt.finalized_at AS finalized_at").
		Join("JOIN groups AS g").JoinOn("g.uuid = ugt.group_uuid").ColumnExpr("g.name AS group_name").
		Join("JOIN tests AS t").JoinOn("t.uuid = ugt.test_uuid").ColumnExpr("t.name AS test_name").
		Join("JOIN users AS u").JoinOn("u.uuid = ugt.user_uuid").ColumnExpr("u.nickname AS user_name").
		OrderBy("ugt.finalized_at", bun.OrderDesc).
		Offset(page*size).Limit(size).
		ScanAndCount(ctx, &totals)
	if err != nil {
		return nil, 0, err
	}

	if len(totals) == 0 {
		return nil, 0, sql.ErrNoRows
	}

	return totals, total, nil
}

func (r *bunTotalRepository) AnswersInTest(ctx context.Context, userUUID, testUUID, groupUUID uuid.UUID, page, size int) ([]contracts.Answer, int, error) {
	var answers []contracts.Answer

	total, err := r.db.NewSelect().TableExpr("users_groups_tests_quiz_answers AS ugtqa").
		Where("ugtqa.user_uuid = ?", userUUID).
		ColumnExpr("ugtqa.score AS score, "+
			"ugtqa.test_uuid AS test_uuid, "+
			"ugtqa.group_uuid AS group_uuid, "+
			"ugtqa.quiz_uuid AS quiz_uuid, "+
			"ugtqa.user_uuid AS user_uuid, "+
			"ugtqa.answered_at AS answered_at, "+
			"ugtqa.answer_value AS answer_value"). //wow
		Join("JOIN groups AS g").JoinOn("g.uuid = ugtqa.group_uuid").ColumnExpr("g.name AS group_name").
		Join("JOIN tests AS t").JoinOn("t.uuid = ugtqa.test_uuid").ColumnExpr("t.name AS test_name").
		Join("JOIN quizzes AS q").JoinOn("q.uuid = ugtqa.quiz_uuid").ColumnExpr("q.correct_answer AS correct").ColumnExpr("q.path AS quiz_name").ColumnExpr("q.score AS max_score").
		OrderBy("ugtqa.answered_at", bun.OrderDesc).
		Offset(page*size).Limit(size).
		ScanAndCount(ctx, &answers)
	if err != nil {
		return nil, 0, err
	}

	if len(answers) == 0 {
		return nil, 0, sql.ErrNoRows
	}

	return answers, total, nil
}
