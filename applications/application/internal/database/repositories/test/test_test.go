package test_test //finnaly something remotely funny

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"log"
	mrand "math/rand/v2"
	"testing"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/database/repositories/test"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func NewInjectorWithTestRepo(t testing.TB) do.Injector {
	t.Helper()

	i := testutils.NewTestInjector(t)

	do.Provide(i, test.NewTestRepository)

	return i
}

func RegisterModels(db *bun.DB) {
	db.RegisterModel((*models.TestsQuizzes)(nil))
	db.RegisterModel((*models.GroupsUsers)(nil))
}

func TestTest_Creation(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	t.Run("New test", func(t *testing.T) {

		name := rand.Text()
		_, err := r.CreateTest(t.Context(), name)
		require.NoError(t, err)

		var test models.Test
		err = db.NewSelect().Model(&test).Where("name = ?", name).Scan(t.Context())
		require.NoError(t, err)
		require.Equal(t, name, test.Name)
	})

	t.Run("Existing test", func(t *testing.T) {

		name := rand.Text()
		_, err := r.CreateTest(t.Context(), name)
		require.NoError(t, err)

		var test models.Test
		err = db.NewSelect().Model(&test).Where("name = ?", name).Scan(t.Context())
		require.NoError(t, err)
		require.Equal(t, name, test.Name)

		_, err = r.CreateTest(t.Context(), name)
		require.Error(t, err)

		require.ErrorAs(t, err, &carefulness.ErrConflict)
	})
}

func TestTest_Deletion(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	test := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	t.Run("Existing test", func(t *testing.T) {

		err := r.DeleteTest(t.Context(), test.UUID)
		require.NoError(t, err)

		err = db.NewSelect().Model(&test).WherePK().Scan(t.Context())
		require.Error(t, err, test)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("Non-existing test", func(t *testing.T) {

		err := r.DeleteTest(t.Context(), uuid.Nil)
		require.Error(t, err, test)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestTest_Update(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	test := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	t.Run("Found test", func(t *testing.T) {

		err := r.UpdateTest(t.Context(), test.UUID, rand.Text())
		require.NoError(t, err)

		testR := models.Test{UUID: test.UUID}
		err = db.NewSelect().Model(&testR).WherePK().Scan(t.Context())
		require.NoError(t, err)

		require.NotEqual(t, test.Name, testR.Name)
	})
	t.Run("Not found test", func(t *testing.T) {

		err := r.UpdateTest(t.Context(), uuid.Nil, rand.Text())
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}

func TestTest_Read(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	test := models.Test{Name: rand.Text()}
	err := db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
	require.NoError(t, err)

	t.Run("Found test", func(t *testing.T) {

		testR, err := r.Test(t.Context(), test.UUID)
		require.NoError(t, err)

		require.Equal(t, test.Name, testR.Name)
		require.Equal(t, test.UUID, testR.UUID)
		require.Equal(t, test.CreatedAt, testR.CreatedAt)
		require.Equal(t, test.UpdatedAt, testR.UpdatedAt)
		require.Empty(t, testR.Quizzes)
	})

	t.Run("Not found test", func(t *testing.T) {

		testR, err := r.Test(t.Context(), uuid.Nil)
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
		require.Empty(t, testR)
	})
}

func TestTest_Bundle(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	t.Run("Valid bundle", func(t *testing.T) {
		test := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		var quizzes []models.Quiz
		for range mrand.IntN(6) + 3 {
			quizzes = append(quizzes, models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}})
		}
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		uuids := lo.Map(quizzes, func(quiz models.Quiz, _ int) uuid.UUID { return quiz.UUID })
		err = r.BundleQuizzesToTest(t.Context(), test.UUID, uuids)
		require.NoError(t, err)

		type quizShort struct {
			UUID     uuid.UUID `bun:"quiz_uuid"`
			Position int       `bun:"position"`
		}
		var quizzesR []quizShort
		err = db.NewSelect().Model((*models.TestsQuizzes)(nil)).
			Where("test_uuid = ?", test.UUID).
			Column("quiz_uuid", "position").
			Scan(t.Context(), &quizzesR)

		var pathesR uuid.UUIDs
		var posR []int
		lo.ForEach(quizzesR, func(quizS quizShort, _ int) {
			pathesR = append(pathesR, quizS.UUID)
			posR = append(posR, quizS.Position)

		})
		require.Condition(t, func() (success bool) {
			i := -1 //crutch
			return lo.EveryBy(uuids, func(uuid uuid.UUID) bool {
				i++
				return i == posR[lo.IndexOf(pathesR, uuid)]
			})
		}, pathesR, posR, uuids)
	})

	t.Run("Patially valid bundle", func(t *testing.T) {
		testM := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		var quizzes []models.Quiz
		for range mrand.IntN(6) + 3 {
			quizzes = append(quizzes, models.Quiz{Path: fmt.Sprintf("/path/to/%v.md", rand.Text()), Checksum: [8]byte{}})
		}
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		uuids := lo.Map(quizzes, func(quiz models.Quiz, _ int) uuid.UUID { return quiz.UUID })
		err = r.BundleQuizzesToTest(t.Context(), testM.UUID, append(uuids, uuid.Nil))
		require.Error(t, err)
	})

	t.Run("Invalid bundle", func(t *testing.T) {
		testM := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		err = r.BundleQuizzesToTest(t.Context(), testM.UUID, append(uuid.UUIDs{}, uuid.Nil))
		require.Error(t, err)
	})
}

// --- Now making benchmarks for those, which seem slow
func BenchmarkGroup_Bundle_quizzes(b *testing.B) {
	log.SetOutput(io.Discard)
	b.Run("Benchmark 5 appendants", func(b *testing.B) {
		i := NewInjectorWithTestRepo(b)

		r := do.MustInvoke[test.TestRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		testUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Test{Name: name}).Returning("uuid").Scan(b.Context(), &testUUID)
		require.NoError(b, err)

		var quizzes []models.Quiz
		for range 5 {
			quizPath := fmt.Sprintf("/path/to/%v.md", rand.Text())

			require.NoError(b, err)
			quizzes = append(quizzes, models.Quiz{Path: quizPath, Checksum: [8]byte{}})
		}

		err = db.NewInsert().Model(&quizzes).
			Scan(b.Context())
		require.NoError(b, err)
		uuids := lo.Map(quizzes, func(quiz models.Quiz, _ int) uuid.UUID {
			return quiz.UUID
		})

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("tests_quizzes").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.BundleQuizzesToTest(b.Context(), testUUID, uuids)
			require.NoError(b, err)
		}
		db.Close()
	})

	b.Run("Benchmark 50 appendants", func(b *testing.B) {
		i := NewInjectorWithTestRepo(b)

		r := do.MustInvoke[test.TestRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		testUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Test{Name: name}).Returning("uuid").Scan(b.Context(), &testUUID)
		require.NoError(b, err)

		var quizzes []models.Quiz
		for range 50 {
			quizPath := fmt.Sprintf("/path/to/%v.md", rand.Text())

			require.NoError(b, err)
			quizzes = append(quizzes, models.Quiz{Path: quizPath, Checksum: [8]byte{}})
		}

		err = db.NewInsert().Model(&quizzes).
			Scan(b.Context())
		require.NoError(b, err)
		uuids := lo.Map(quizzes, func(quiz models.Quiz, _ int) uuid.UUID {
			return quiz.UUID
		})

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("tests_quizzes").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.BundleQuizzesToTest(b.Context(), testUUID, uuids)
			require.NoError(b, err)
		}
	})

	b.Run("Benchmark 500 appendants", func(b *testing.B) {
		i := NewInjectorWithTestRepo(b)

		r := do.MustInvoke[test.TestRepository](i)
		db := do.MustInvoke[*bun.DB](i)
		RegisterModels(db)

		name := rand.Text()
		testUUID := uuid.UUID{}
		err := db.NewInsert().Model(&models.Test{Name: name}).Returning("uuid").Scan(b.Context(), &testUUID)
		require.NoError(b, err)

		var quizzes []models.Quiz
		for range 500 {
			quizPath := fmt.Sprintf("/path/to/%v.md", rand.Text())

			require.NoError(b, err)
			quizzes = append(quizzes, models.Quiz{Path: quizPath, Checksum: [8]byte{}})
		}

		err = db.NewInsert().Model(&quizzes).
			Scan(b.Context())
		require.NoError(b, err)
		uuids := lo.Map(quizzes, func(quiz models.Quiz, _ int) uuid.UUID {
			return quiz.UUID
		})

		b.ResetTimer()

		for b.Loop() {
			b.StopTimer()
			_, err := db.NewTruncateTable().Table("tests_quizzes").Exec(b.Context())
			require.NoError(b, err)
			b.StartTimer()

			err = r.BundleQuizzesToTest(b.Context(), testUUID, uuids)
			require.NoError(b, err)
		}
	})
}

func TestTest_Prune(t *testing.T) {

	i := NewInjectorWithTestRepo(t)
	r := do.MustInvoke[test.TestRepository](i)
	db := do.MustInvoke[*bun.DB](i)

	RegisterModels(db)

	t.Run("Valid prune", func(t *testing.T) {
		test := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		count := mrand.IntN(6) + 3
		var quizzes []models.Quiz = make([]models.Quiz, count)
		for i := range count {
			path := fmt.Sprintf("/path/to/%v.md", rand.Text())
			quizzes[i] = models.Quiz{Path: path, Checksum: [8]byte{}}
		}
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		uuids := make(uuid.UUIDs, len(quizzes))
		inserts := lo.Map(quizzes, func(quiz models.Quiz, i int) models.TestsQuizzes {
			uuids[i] = quiz.UUID
			return models.TestsQuizzes{TestUUID: test.UUID, QuizUUID: quiz.UUID, Position: i}
		})
		_, err = db.NewInsert().Model(&inserts).Exec(t.Context())
		require.NoError(t, err)

		err = r.PruneQuizzesFromTest(t.Context(), test.UUID, uuids)
		require.NoError(t, err)

		type quizShort struct {
			Path     string `bun:"quiz_path"`
			Position int    `bun:"position"`
		}
		var quizzesR []quizShort
		err = db.NewSelect().Model((*models.TestsQuizzes)(nil)).
			Where("test_uuid = ?", test.UUID).
			Column("quiz_path", "position").
			Scan(t.Context(), &quizzesR)

		require.Empty(t, quizzesR)
	})

	t.Run("Patially valid prune", func(t *testing.T) {
		testM := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		count := mrand.IntN(6) + 3
		var quizzes []models.Quiz = make([]models.Quiz, count)
		pathes := make([]string, count)
		for i := range count {
			path := fmt.Sprintf("/path/to/%v.md", rand.Text())
			pathes[i] = path
			quizzes[i] = models.Quiz{Path: path, Checksum: [8]byte{}}
		}
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)
		uuids := make(uuid.UUIDs, len(quizzes))

		inserts := lo.Map(quizzes, func(quiz models.Quiz, i int) models.TestsQuizzes {
			uuids[i] = quiz.UUID
			return models.TestsQuizzes{TestUUID: testM.UUID, QuizUUID: quiz.UUID, Position: i}
		})
		_, err = db.NewInsert().Model(&inserts).Exec(t.Context())
		require.NoError(t, err)

		err = r.PruneQuizzesFromTest(t.Context(), testM.UUID, append(uuids, uuid.Nil))
		require.Error(t, err)
		require.Equal(t, "1 quizzes are not in test", err.Error())

		type quizShort struct {
			Path     string `bun:"quiz_path"`
			Position int    `bun:"position"`
		}
		var quizzesR []quizShort
		err = db.NewSelect().Model((*models.TestsQuizzes)(nil)).
			Where("test_uuid = ?", testM.UUID).
			Column("quiz_path", "position").
			Scan(t.Context(), &quizzesR)

		require.Empty(t, quizzesR)
	})

	t.Run("Invalid prune", func(t *testing.T) {
		testM := models.Test{Name: rand.Text()}
		err := db.NewInsert().Model(&testM).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		err = r.PruneQuizzesFromTest(t.Context(), testM.UUID, append(uuid.UUIDs{}, uuid.Nil))
		require.Error(t, err)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})
}
