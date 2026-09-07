package quiz_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	mrand "math/rand/v2"
	"testing"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/database/repositories/quiz"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func NewInjectorWithQuizRepo(t *testing.T) do.Injector {
	t.Helper()

	i := testutils.NewTestInjector(t)

	do.Provide(i, quiz.NewQuizRepository)

	return i
}

func TestQuiz_Register(t *testing.T) {

	i := NewInjectorWithQuizRepo(t)

	r := do.MustInvoke[quiz.QuizRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	t.Run("Valid quiz", func(t *testing.T) {

		path := "/usr/" + rand.Text() + "/path/to/quiz.md"
		err := r.RegisterQuiz(t.Context(), path, [8]byte{}, 1, []byte("121321"))
		require.NoError(t, err)

		quiz := models.Quiz{Path: path}
		err = db.NewSelect().Model(&quiz).Where("path = ?", path).Scan(t.Context())
		require.NoError(t, err)

		require.Equal(t, quiz.Path, path)
		require.Equal(t, quiz.CorrectAnswer, "121321")
	})
	t.Run("Not abs path", func(t *testing.T) {

		err := r.RegisterQuiz(t.Context(), "path.md", [8]byte{}, 1, []byte{})
		require.Error(t, err)
		require.ErrorIs(t, err, carefulness.ErrAbsoluteRequired)
	})
	t.Run("Not md", func(t *testing.T) {

		err := r.RegisterQuiz(t.Context(), "/usr/path/to/cooler_quiz.mdx", [8]byte{}, 1, []byte{})
		require.Error(t, err)
		require.ErrorIs(t, err, carefulness.PlainMarkdownRequired)
	})
}

func TestQuiz_Deallocate(t *testing.T) {

	i := NewInjectorWithQuizRepo(t)

	r := do.MustInvoke[quiz.QuizRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	path := "/usr/path/to/quiz.md"
	var uuidQ uuid.UUID
	err := db.NewInsert().Model(&models.Quiz{Path: path, Checksum: [8]byte{1, 2}, Score: 2}).Returning("uuid").Scan(t.Context(), &uuidQ)
	require.NoError(t, err)

	testCases := []struct {
		desc           string
		uuid           uuid.UUID
		expectNotFound bool
	}{
		{
			desc:           "Valid deallocation",
			uuid:           uuidQ,
			expectNotFound: false,
		},
		{
			desc:           "Invalid deallocation",
			uuid:           uuid.Nil,
			expectNotFound: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

			err = r.DeallocateQuiz(context.Background(), tC.uuid)
			if tC.expectNotFound {
				require.ErrorIs(t, err, sql.ErrNoRows)
				return
			}
			require.NoError(t, err)

			f, err := db.NewSelect().Model(&models.Quiz{Path: path}).WherePK().Exists(t.Context())
			require.NoError(t, err)
			require.False(t, f)
		})
	}
}

func TestQuiz_Check_registered(t *testing.T) {

	i := NewInjectorWithQuizRepo(t)

	r := do.MustInvoke[quiz.QuizRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	path := "/usr/path/to/quiz.md"
	_, err := db.NewInsert().Model(&models.Quiz{Path: path, Checksum: [8]byte{}, Score: 2}).Exec(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc               string
		path               string
		expectUnregistered bool
	}{
		{
			desc:               "Registered",
			path:               path,
			expectUnregistered: false,
		},
		{
			desc:               "Unregistered",
			path:               "",
			expectUnregistered: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			is, err := r.CheckRegistered(context.Background(), tC.path)
			require.NoError(t, err)
			if tC.expectUnregistered {
				require.False(t, is)
			} else {
				require.True(t, is)
			}
		})
	}
}

func TestQuiz_Check_integrity(t *testing.T) {

	i := NewInjectorWithQuizRepo(t)

	r := do.MustInvoke[quiz.QuizRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	path := "/usr/path/to/quiz.md"
	randomBytes := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

	_, err := db.NewInsert().Model(&models.Quiz{Path: path, Checksum: randomBytes, Score: 2}).Exec(t.Context())
	require.NoError(t, err)

	testCases := []struct {
		desc               string
		path               string
		checksum           [8]byte
		expectUnintegrated bool
	}{
		{
			desc:               "Integrated",
			path:               path,
			checksum:           randomBytes,
			expectUnintegrated: false,
		},
		{
			desc:               "Unintegrated, registered",
			path:               path,
			checksum:           [8]byte{},
			expectUnintegrated: true,
		},
		{
			desc:               "Unintegrated, unregistered",
			path:               "",
			checksum:           [8]byte{},
			expectUnintegrated: true,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			is, err := r.CheckIntegrity(context.Background(), tC.path, tC.checksum)
			require.NoError(t, err)
			if tC.expectUnintegrated {
				require.False(t, is)
			} else {
				require.True(t, is)
			}
		})
	}
}

func TestQuiz_List(t *testing.T) {

	i := NewInjectorWithQuizRepo(t)

	r := do.MustInvoke[quiz.QuizRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	var pathes []string

	for range 5 {
		path := fmt.Sprintf("/usr/path/to/%v.md", rand.Text())
		randomBytes := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}

		quiz := models.Quiz{Path: path, Checksum: randomBytes, Score: mrand.Int()}
		_, err := db.NewInsert().Model(&quiz).Exec(t.Context())
		require.NoError(t, err)

		pathes = append(pathes, quiz.Path)
	}

	quizzes, total, err := r.ListQuizzes(t.Context(), 0, len(pathes)-1)
	require.NoError(t, err)
	require.Equal(t, len(pathes), total)
	require.Len(t, quizzes, len(pathes)-1)
	require.Condition(t, func() (success bool) {
		return lo.Every(pathes, lo.Map(quizzes, func(item models.Quiz, _ int) string {
			return item.Path
		}))
	})
	require.Condition(t, func() (success bool) {
		return lo.EveryBy(lo.Window(quizzes, 2), func(item []models.Quiz) bool {
			return item[0].Path > item[1].Path
		})
	})
}
