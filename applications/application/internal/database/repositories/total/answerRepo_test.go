package total_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"testing"

	"github.com/egot3/fathom/internal/database/repositories/total"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func NewInjectorWithTestRepo(t testing.TB) do.Injector {
	t.Helper()

	i := testutils.NewTestInjector(t)

	do.Provide(i, total.NewTotalRepository)

	return i
}

func RegisterModels(db *bun.DB) {
	db.RegisterModel((*models.TestsQuizzes)(nil))
	db.RegisterModel((*models.GroupsUsers)(nil))
	db.RegisterModel((*models.UserGroupsTests)(nil))
	db.RegisterModel((*models.Answer)(nil))
}

func TestAnswer_Set(t *testing.T) {
	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[total.TotalRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	testM := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	var quiz models.Quiz = models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}, Score: 1}
	err = db.NewInsert().Model(&quiz).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID}).Exec(t.Context())
	require.NoError(t, err)

	name := rand.Text()
	var groupUUID uuid.UUID
	err = db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var userUUID uuid.UUID
	err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
		Returning("uuid").
		Scan(t.Context(), &userUUID)
	require.NoError(t, err)
	require.NotEmpty(t, userUUID)

	_, err = db.NewInsert().Model(&models.GroupsUsers{GroupUUID: groupUUID, UserUUID: userUUID}).Exec(t.Context())

	testCases := []struct {
		desc      string
		testUUID  uuid.UUID
		userUUID  uuid.UUID
		groupUUID uuid.UUID
		quizUUID  uuid.UUID
		expectErr bool
	}{
		{
			desc:      "Valid set",
			testUUID:  testM.UUID,
			quizUUID:  quiz.UUID,
			userUUID:  userUUID,
			groupUUID: groupUUID,
			expectErr: false,
		},
		{
			desc:      "Invalid user",
			testUUID:  testM.UUID,
			quizUUID:  quiz.UUID,
			userUUID:  uuid.Nil,
			groupUUID: groupUUID,
			expectErr: true,
		},
		{
			desc:      "Invalid group",
			testUUID:  testM.UUID,
			quizUUID:  quiz.UUID,
			userUUID:  userUUID,
			groupUUID: uuid.Nil,
			expectErr: true,
		},
		{
			desc:      "Invalid group and user", // testing those pairs as they are only binded between eachother
			testUUID:  testM.UUID,
			quizUUID:  quiz.UUID,
			userUUID:  uuid.Nil,
			groupUUID: uuid.Nil,
			expectErr: true,
		},
		{
			desc:      "Invalid all",
			testUUID:  uuid.Nil,
			quizUUID:  uuid.Nil,
			userUUID:  uuid.Nil,
			groupUUID: uuid.Nil,
			expectErr: true,
		},
	} // no test and quiz as it's managed at upper layer
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			t.Cleanup(func() {
				_, err := db.NewTruncateTable().Model((*models.Answer)(nil)).Exec(context.Background())
				require.NoError(t, err)
			})

			err := r.SetAnswer(t.Context(), tC.testUUID, tC.groupUUID, tC.userUUID, tC.quizUUID, "", 1)
			if tC.expectErr {
				require.Error(t, err, err.Error())
				require.ErrorIs(t, err, sql.ErrNoRows, err.Error())
				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("Upsertion", func(t *testing.T) {
		t.Cleanup(func() {
			_, err := db.NewTruncateTable().Model((*models.Answer)(nil)).Exec(context.Background())
			require.NoError(t, err)
		})

		err := r.SetAnswer(t.Context(), testM.UUID, groupUUID, userUUID, quiz.UUID, "", 1)

		require.NoError(t, err)
		var insertion models.Answer = models.Answer{}
		err = db.NewSelect().Model(&insertion).
			Where("test_uuid = ?", testM.UUID).
			Where("group_uuid = ?", groupUUID).
			Where("user_uuid = ?", userUUID).
			Where("quiz_uuid = ?", quiz.UUID).
			Scan(t.Context())
		require.NoError(t, err)

		err = r.SetAnswer(t.Context(), testM.UUID, groupUUID, userUUID, quiz.UUID, "", 1)

		require.NoError(t, err)

		var updation models.Answer = models.Answer{}
		err = db.NewSelect().Model(&insertion).
			Where("test_uuid = ?", testM.UUID).
			Where("group_uuid = ?", groupUUID).
			Where("user_uuid = ?", userUUID).
			Where("quiz_uuid = ?", quiz.UUID).
			Scan(t.Context())
		require.NoError(t, err)

		require.NotEqual(t, insertion.AnsweredAt, updation.AnsweredAt)
	})
}

func TestAnswer_Get(t *testing.T) {
	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[total.TotalRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	testM := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	var quiz models.Quiz = models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}, Score: 1}
	err = db.NewInsert().Model(&quiz).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID}).Exec(t.Context())
	require.NoError(t, err)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err = db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var userUUID uuid.UUID

	err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
		Returning("uuid").
		Scan(t.Context(), &userUUID)
	require.NoError(t, err)
	require.NotEmpty(t, userUUID)

	_, err = db.NewInsert().Model(&models.GroupsUsers{GroupUUID: groupUUID, UserUUID: userUUID}).Exec(t.Context())

	score := mrand.Float32() * 256
	totalValue := rand.Text()
	_, err = db.NewInsert().Model(&models.Answer{
		GroupUUID:   groupUUID,
		TestUUID:    testM.UUID,
		UserUUID:    userUUID,
		QuizUUID:    quiz.UUID,
		AnswerValue: totalValue,
		Score:       score,
	}).Exec(t.Context())
	require.NoError(t, err)

	t.Run("Score suite", func(t *testing.T) {
		t.Run("Of known", func(t *testing.T) {
			scoreR, err := r.AnswerScore(t.Context(), userUUID, testM.UUID, groupUUID, quiz.UUID)
			require.NoError(t, err)
			require.EqualValues(t, score, scoreR)
		})
		t.Run("Of unknown", func(t *testing.T) {
			scoreR, err := r.AnswerScore(t.Context(), uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil)
			require.Error(t, err)
			require.EqualValues(t, 0, scoreR)
		})
	})
	t.Run("Value suite", func(t *testing.T) {
		t.Run("Of known", func(t *testing.T) {
			totalR, err := r.Answer(t.Context(), userUUID, testM.UUID, groupUUID, quiz.UUID)
			require.NoError(t, err)
			require.Equal(t, totalValue, totalR)
		})
		t.Run("Of unknown", func(t *testing.T) {
			totalR, err := r.Answer(t.Context(), uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil)
			require.Error(t, err)
			require.Equal(t, "", totalR)
		})
	})
}

func TestAnswer_Totalization(t *testing.T) {
	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[total.TotalRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	testM := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	var quiz models.Quiz = models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}, Score: 1}
	err = db.NewInsert().Model(&quiz).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID}).Exec(t.Context())
	require.NoError(t, err)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err = db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var userUUID uuid.UUID

	err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
		Returning("uuid").
		Scan(t.Context(), &userUUID)
	require.NoError(t, err)
	require.NotEmpty(t, userUUID)

	_, err = db.NewInsert().Model(&models.GroupsUsers{GroupUUID: groupUUID, UserUUID: userUUID}).Exec(t.Context())

	score := mrand.Float32() * 256
	_, err = db.NewInsert().Model(&models.Answer{
		GroupUUID:   groupUUID,
		TestUUID:    testM.UUID,
		UserUUID:    userUUID,
		QuizUUID:    quiz.UUID,
		AnswerValue: rand.Text(),
		Score:       score,
	}).Exec(t.Context())
	require.NoError(t, err)

	t.Run("Totalize known", func(t *testing.T) {
		t.Cleanup(func() {
			_, err := db.NewTruncateTable().Model((*models.UserGroupsTests)(nil)).Exec(context.Background())
			require.NoError(t, err)
		})

		err := r.Totalize(t.Context(), userUUID, testM.UUID, groupUUID)
		require.NoError(t, err)

		var scoreR float64
		err = db.NewSelect().Model(&models.UserGroupsTests{
			UserUUID:  userUUID,
			GroupUUID: groupUUID,
			TestUUID:  testM.UUID,
		}).Column("score").Scan(t.Context(), &scoreR)

		require.NoError(t, err)
		require.Condition(t, func() (success bool) {
			return math.Abs(scoreR-float64(score)) < 0.1
		})
	})

	t.Run("Totalize unknown", func(t *testing.T) {
		t.Cleanup(func() {
			_, err := db.NewTruncateTable().Model((*models.UserGroupsTests)(nil)).Exec(context.Background())
			require.NoError(t, err)
		})

		err := r.Totalize(t.Context(), uuid.Nil, testM.UUID, groupUUID)
		require.Error(t, err)

		c, err := db.NewSelect().Model((*models.UserGroupsTests)(nil)).
			Column("score").Count(t.Context())

		require.NoError(t, err)
		require.Equal(t, 0, c)
	})

	t.Run("Re-totalize known", func(t *testing.T) {
		t.Cleanup(func() {
			_, err := db.NewTruncateTable().Model((*models.UserGroupsTests)(nil)).Exec(context.Background())
			require.NoError(t, err)
		})

		err := r.Totalize(t.Context(), userUUID, testM.UUID, groupUUID)
		require.NoError(t, err)

		var scoreR float64
		err = db.NewSelect().Model(&models.UserGroupsTests{
			UserUUID:  userUUID,
			GroupUUID: groupUUID,
			TestUUID:  testM.UUID,
		}).Column("score").Scan(t.Context(), &scoreR)

		require.NoError(t, err)
		require.Condition(t, func() (success bool) {
			return math.Abs(scoreR-float64(score)) < 0.1
		})

		scoreN := mrand.Float32() * 256
		var quiz models.Quiz = models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}, Score: 1}
		err = db.NewInsert().Model(&quiz).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		_, err = db.NewInsert().Model(&models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID}).Exec(t.Context())
		require.NoError(t, err)

		_, err = db.NewInsert().Model(&models.Answer{
			GroupUUID:   groupUUID,
			TestUUID:    testM.UUID,
			UserUUID:    userUUID,
			QuizUUID:    quiz.UUID,
			AnswerValue: rand.Text(),
			Score:       scoreN,
		}).Exec(t.Context())
		require.NoError(t, err)

		err = r.Totalize(t.Context(), userUUID, testM.UUID, groupUUID)
		require.NoError(t, err)

		var scoreR2 float32
		err = db.NewSelect().Model(&models.UserGroupsTests{
			UserUUID:  userUUID,
			GroupUUID: groupUUID,
			TestUUID:  testM.UUID,
		}).Column("score").Scan(t.Context(), &scoreR2)

		require.NoError(t, err)
		require.InDelta(t, score+scoreN, scoreR2, 1e-4)
	})
}

func TestAnswer_Totals(t *testing.T) {
	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[total.TotalRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	testM := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	var quiz models.Quiz = models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}, Score: 1}
	err = db.NewInsert().Model(&quiz).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID}).Exec(t.Context())
	require.NoError(t, err)

	name := rand.Text()
	groupUUID := uuid.UUID{}
	err = db.NewInsert().Model(&models.Group{Name: name}).Returning("uuid").Scan(t.Context(), &groupUUID)
	require.NoError(t, err)

	var userUUID uuid.UUID

	err = db.NewInsert().Model(&models.User{Nickname: rand.Text(), PasswordHash: []byte{}}).
		Returning("uuid").
		Scan(t.Context(), &userUUID)
	require.NoError(t, err)
	require.NotEmpty(t, userUUID)

	_, err = db.NewInsert().Model(&models.GroupsUsers{GroupUUID: groupUUID, UserUUID: userUUID}).Exec(t.Context())

	score := mrand.Float64() * 256
	_, err = db.NewInsert().Model(&models.UserGroupsTests{
		TestUUID:  testM.UUID,
		GroupUUID: groupUUID,
		UserUUID:  userUUID,
		Score:     score,
	}).Exec(t.Context())
	require.NoError(t, err)

	t.Run("User total", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {

			total, err := r.Total(t.Context(), userUUID, testM.UUID, groupUUID)
			require.NoError(t, err)
			require.Equal(t, score, total.Score, total)
		})

		errTestCases := []struct {
			desc      string
			userUUID  uuid.UUID
			groupUUID uuid.UUID
			testUUID  uuid.UUID
		}{
			{
				desc:      "No user UUID",
				userUUID:  uuid.Nil,
				groupUUID: groupUUID,
				testUUID:  testM.UUID,
			},
			{
				desc:      "No test UUID",
				userUUID:  userUUID,
				groupUUID: groupUUID,
				testUUID:  uuid.Nil,
			},
			{
				desc:      "No group UUID",
				userUUID:  userUUID,
				groupUUID: uuid.Nil,
				testUUID:  testM.UUID,
			},
			{
				desc:      "No user&group UUID",
				userUUID:  uuid.Nil,
				groupUUID: uuid.Nil,
				testUUID:  testM.UUID,
			},
			{
				desc:      "No user&test UUID",
				userUUID:  uuid.Nil,
				groupUUID: groupUUID,
				testUUID:  uuid.Nil,
			},
			{
				desc:      "No group&test UUID",
				userUUID:  userUUID,
				groupUUID: uuid.Nil,
				testUUID:  uuid.Nil,
			},
			{
				desc:      "No user&test&group UUID",
				userUUID:  uuid.Nil,
				groupUUID: uuid.Nil,
				testUUID:  uuid.Nil,
			},
		}
		for _, etc := range errTestCases {
			t.Run(etc.desc, func(t *testing.T) {

				retrieved, err := r.Total(t.Context(), etc.userUUID, etc.testUUID, etc.groupUUID)
				require.Error(t, err, retrieved)
				require.ErrorIs(t, err, sql.ErrNoRows)
			})
		}
	})

	t.Run("Group totals", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {

			totals, err := r.GroupTestTotals(t.Context(), testM.UUID, groupUUID)
			require.NoError(t, err)
			require.Len(t, totals, 1)
			require.Equal(t, totals[0].Score, score)
		})

		errTestCases := []struct {
			desc      string
			groupUUID uuid.UUID
			testUUID  uuid.UUID
		}{
			{
				desc:      "No test UUID",
				groupUUID: groupUUID,
				testUUID:  uuid.Nil,
			},
			{
				desc:      "No group UUID",
				groupUUID: uuid.Nil,
				testUUID:  testM.UUID,
			},
			{
				desc:      "No group&test UUID",
				groupUUID: uuid.Nil,
				testUUID:  uuid.Nil,
			},
		}
		for _, etc := range errTestCases {
			t.Run(etc.desc, func(t *testing.T) {

				retrieved, err := r.GroupTestTotals(t.Context(), etc.testUUID, etc.groupUUID)
				require.Error(t, err)
				require.ErrorIs(t, err, sql.ErrNoRows)
				require.Nil(t, retrieved)
			})
		}
	})

	t.Run("Test totals", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {

			totals, err := r.TestTotals(t.Context(), testM.UUID)
			require.NoError(t, err)
			require.Len(t, totals, 1)
			require.Equal(t, totals[0].Score, score)
		})
		t.Run("Invalid", func(t *testing.T) {

			totals, err := r.TestTotals(t.Context(), uuid.Nil)
			require.Error(t, err)
			require.Nil(t, totals)
		})
	})

	t.Run("User totals", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {

			totals, total, err := r.UserTotals(t.Context(), userUUID, 0, 1)
			require.NoError(t, err)
			require.Equal(t, 1, total)
			require.Len(t, totals, 1)
			require.Equal(t, totals[0].Score, score)
		})
		t.Run("Invalid", func(t *testing.T) {

			totals, total, err := r.UserTotals(t.Context(), uuid.Nil, 0, 1)
			require.Len(t, totals, 0)

			require.Equal(t, 0, total)
			require.Error(t, err)
			require.Nil(t, totals)
		})
	})

	t.Run("All totals", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {

			totals, total, err := r.ListTotals(t.Context(), 0, 1)
			require.NoError(t, err)
			require.Equal(t, 1, total)
			require.Len(t, totals, 1)
			require.Equal(t, totals[0].Score, score)
			t.Logf("%+v", totals)
		})
	})
}

func BenchmarkTotals_Errors(b *testing.B) {
	i := NewInjectorWithTestRepo(b)
	r := do.MustInvoke[total.TotalRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)
	b.Run("Not found", func(b *testing.B) {
		for b.Loop() {
			r.UserTotals(b.Context(), uuid.Nil, 0, 1)
		}
	})
}
