package handler_test

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	charmlog "github.com/charmbracelet/log"
	"github.com/egot3/fathom/internal/contracts"
	"github.com/egot3/fathom/internal/database/repositories"
	"github.com/egot3/fathom/internal/handler"
	"github.com/egot3/fathom/internal/models"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/egot3/fathom/server"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func RegisterM2M(db *bun.DB) {
	db.RegisterModel((*models.TestsQuizzes)(nil))
	db.RegisterModel((*models.GroupsUsers)(nil))
}

func TestTestHandler_AddQuizzes(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizzes := make([]models.Quiz, 10)
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			quizzes[i] = models.Quiz{
				Path:          "/somepath/" + rand.Text() + ".md",
				Checksum:      [8]byte{},
				Score:         1,
				CorrectAnswer: rand.Text(),
			}
		})
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		contract := contracts.AddQuizzesToTestRequest{
			QuizUUIDs: make(uuid.UUIDs, 10),
		}
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			contract.QuizUUIDs[i] = quiz.UUID
		})

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		t.Logf(
			`quizzes len: %d
retrieved quiz sample: %v
quiz sample(different): %v`,
			len(testRetrieved.Quizzes),
			lo.Sample(testRetrieved.Quizzes),
			lo.Sample(quizzes))

		require.Condition(t, func() (success bool) {
			return lo.ElementsMatchBy(testRetrieved.Quizzes, quizzes,
				func(quiz models.Quiz) uuid.UUID {
					return quiz.UUID
				})
		})
	})

	t.Run("Empty", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		contract := contracts.AddQuizzesToTestRequest{}

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		require.Empty(t, testRetrieved.Quizzes)
	})

	t.Run("Conflict!", func(t *testing.T) { // on conflict just append

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizzes := make([]models.Quiz, 10)
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			quizzes[i] = models.Quiz{
				Path:          "/somepath/" + rand.Text() + ".md",
				Checksum:      [8]byte{},
				Score:         1,
				CorrectAnswer: rand.Text(),
			}
		})
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		contract := contracts.AddQuizzesToTestRequest{
			QuizUUIDs: make(uuid.UUIDs, 10),
		}
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			contract.QuizUUIDs[i] = quiz.UUID
		})

		_, err = db.NewInsert().Model(&models.TestsQuizzes{
			TestUUID: test.UUID,
			QuizUUID: lo.Sample(quizzes).UUID,
		}).Exec(t.Context())
		require.NoError(t, err)

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		t.Logf(
			`quizzes len: %d
retrieved quiz sample: %v
quiz sample(different): %v`,
			len(testRetrieved.Quizzes),
			lo.Sample(testRetrieved.Quizzes),
			lo.Sample(quizzes))

		lo.ForEach(testRetrieved.Quizzes, func(quiz models.Quiz, _ int) {
			t.Log(quiz.UUID)
		})

		require.Condition(t, func() (success bool) {
			return lo.ElementsMatchBy(testRetrieved.Quizzes, quizzes,
				func(quiz models.Quiz) uuid.UUID {
					t.Log(quiz.UUID)
					return quiz.UUID
				})
		})
	})
}

func TestTestHandler_Delete(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		exists, err := db.NewSelect().Model((*models.Test)(nil)).
			Where("uuid = ?", test.UUID).Exists(t.Context())
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("Not found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/test/%v", uuid.Nil),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		exists, err := db.NewSelect().Model((*models.Test)(nil)).
			Where("uuid = ?", test.UUID).Exists(t.Context())
		require.NoError(t, err)
		require.True(t, exists)
	})
}

func TestTestHandler_Get(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var bodyContract contracts.GetTestResponse
		err = json.NewDecoder(rec.Body).Decode(&bodyContract)
		require.NoError(t, err)

		require.Equal(t, test.Name, bodyContract.Test.Name)
		require.Equal(t, test.UUID, bodyContract.Test.UUID)
	})

	t.Run("Not found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/test/%v", uuid.Nil),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestTestHandler_Patch(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		newName := rand.Text()
		reqJSON, err := json.Marshal(contracts.PatchTestRequest{
			Name: &newName,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		var testRetrieved models.Test
		err = db.NewSelect().Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Scan(t.Context())
		require.NoError(t, err)

		require.Equal(t, newName, testRetrieved.Name)
	})

	t.Run("Not found", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		newName := rand.Text()
		reqJSON, err := json.Marshal(contracts.PatchTestRequest{
			Name: &newName,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/test/%v", uuid.Nil),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)

		var testRetrieved models.Test
		err = db.NewSelect().Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Scan(t.Context())
		require.NoError(t, err)

		require.NotEqual(t, newName, testRetrieved.Name)
		require.Equal(t, test.Name, testRetrieved.Name)
	})

	t.Run("No changes", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		reqJSON, err := json.Marshal(contracts.PatchTestRequest{})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		var testRetrieved models.Test
		err = db.NewSelect().Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Scan(t.Context())
		require.NoError(t, err)

		require.Equal(t, test.Name, testRetrieved.Name)
	})

	t.Run("Naming issues", func(t *testing.T) {
		t.Run("Too big", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			newName := "extremely____long_____strings___which___is____bigger____than_____255_____chars____in____lenght____far____bigger____also___with____randomness:____" + rand.Text() + rand.Text() + rand.Text() + rand.Text() + rand.Text()
			reqJSON, err := json.Marshal(contracts.PatchTestRequest{
				Name: &newName,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)

			var testRetrieved models.Test
			err = db.NewSelect().Model(&testRetrieved).
				Where("uuid = ?", test.UUID).Scan(t.Context())
			require.NoError(t, err)

			require.NotEqual(t, newName, testRetrieved.Name)
		})

		t.Run("Smol", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			newName := "1"
			reqJSON, err := json.Marshal(contracts.PatchTestRequest{
				Name: &newName,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/test/%v", test.UUID.String()),
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)

			var testRetrieved models.Test
			err = db.NewSelect().Model(&testRetrieved).
				Where("uuid = ?", test.UUID).Scan(t.Context())
			require.NoError(t, err)

			require.NotEqual(t, newName, testRetrieved.Name)
		})
	})
}

func TestTestHandler_Post(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {
		t.Run("No children", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			reqJSON, err := json.Marshal(contracts.PostTestRequest{
				Name: rand.Text(),
			})

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/test/",
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNoContent, rec.Code)

			count, err := db.NewSelect().Model((*models.Test)(nil)).
				Count(t.Context())
			require.NoError(t, err)
			require.Equal(t, 1, count)
		})

		t.Run("yes children", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			quizzes := make([]models.Quiz, 10)
			lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
				quizzes[i] = models.Quiz{
					Path:          "/somepath/" + rand.Text() + ".md",
					Checksum:      [8]byte{},
					Score:         1,
					CorrectAnswer: rand.Text(),
				}
			})
			err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			contract := contracts.PostTestRequest{
				Quizzes: make(uuid.UUIDs, 10),
				Name:    rand.Text(),
			}
			lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
				contract.Quizzes[i] = quiz.UUID
			})

			reqJSON, err := json.Marshal(contract)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/test/",
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNoContent, rec.Code)

			count, err := db.NewSelect().Model((*models.Test)(nil)).
				Count(t.Context())
			require.NoError(t, err)
			require.Equal(t, 1, count)

			var test models.Test
			c, err := db.NewSelect().Model(&test).
				Where("name = ?", contract.Name).Relation("Quizzes").
				ScanAndCount(t.Context())
			require.NoError(t, err)

			require.Equal(t, 1, c)
			require.Condition(t, func() (success bool) {
				return lo.ElementsMatchBy(test.Quizzes, quizzes,
					func(quiz models.Quiz) uuid.UUID {
						return quiz.UUID
					})
			})
		})
	})

	t.Run("Conflict!", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		reqJSON, err := json.Marshal(contracts.PostTestRequest{
			Name: test.Name,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/test",
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("Bad names", func(t *testing.T) {
		t.Run("Too big", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			newName := "extremely____long_____strings___which___is____bigger____than_____255_____chars____in____lenght____far____bigger____also___with____randomness:____" + rand.Text() + rand.Text() + rand.Text() + rand.Text() + rand.Text()
			reqJSON, err := json.Marshal(contracts.PostTestRequest{
				Name: newName,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/test",
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)

			var testRetrieved models.Test
			err = db.NewSelect().Model(&testRetrieved).
				Where("name = 1").Scan(t.Context())
			require.Error(t, err)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})

		t.Run("Smol", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			reqJSON, err := json.Marshal(contracts.PostTestRequest{
				Name: "1",
			})
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/test",
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)

			var testRetrieved models.Test
			err = db.NewSelect().Model(&testRetrieved).
				Where("name = 1").Scan(t.Context())
			require.Error(t, err)
			require.ErrorIs(t, err, sql.ErrNoRows)
		})
	})
}

func TestTestHandler_RemoveQuizzes(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizzes := make([]models.Quiz, 10)

		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			quizzes[i] = models.Quiz{
				Path:          "/somepath/" + rand.Text() + ".md",
				Checksum:      [8]byte{},
				Score:         1,
				CorrectAnswer: rand.Text(),
			}
		})
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizTests := make([]models.TestsQuizzes, 10)
		contract := contracts.RemoveQuizzesRequest{
			QuizUUIDs: make(uuid.UUIDs, 10),
		}
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			contract.QuizUUIDs[i] = quiz.UUID
			quizTests[i] = models.TestsQuizzes{
				TestUUID: test.UUID,
				QuizUUID: quiz.UUID,
				Position: i,
			}
		})

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		_, err = db.NewInsert().Model(&quizTests).Exec(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		t.Logf(
			`quizzes len: %d
retrieved quiz sample: %v
quiz sample(different): %v`,
			len(testRetrieved.Quizzes),
			lo.Sample(testRetrieved.Quizzes),
			lo.Sample(quizzes))

		require.Empty(t, testRetrieved.Quizzes)
	})

	t.Run("Empty", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		contract := contracts.AddQuizzesToTestRequest{}

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		require.Empty(t, testRetrieved.Quizzes)
	})

	t.Run("Partial deletion", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizzes := make([]models.Quiz, 10)

		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			quizzes[i] = models.Quiz{
				Path:          "/somepath/" + rand.Text() + ".md",
				Checksum:      [8]byte{},
				Score:         1,
				CorrectAnswer: rand.Text(),
			}
		})
		err = db.NewInsert().Model(&quizzes).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		quizTests := make([]models.TestsQuizzes, 10)
		contract := contracts.RemoveQuizzesRequest{
			QuizUUIDs: make(uuid.UUIDs, 10),
		}
		lo.ForEach(quizzes, func(quiz models.Quiz, i int) {
			contract.QuizUUIDs[i] = quiz.UUID
			quizTests[i] = models.TestsQuizzes{
				TestUUID: test.UUID,
				QuizUUID: quiz.UUID,
				Position: i,
			}
		})

		_, err = db.NewInsert().Model(&models.TestsQuizzes{
			TestUUID: test.UUID,
			QuizUUID: lo.Sample(quizzes).UUID,
		}).Exec(t.Context())
		require.NoError(t, err)

		reqJSON, err := json.Marshal(contract)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/test/%v/quizzes", test.UUID.String()),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)

		testRetrieved := models.Test{}
		err = db.NewSelect().
			Model(&testRetrieved).
			Where("uuid = ?", test.UUID).Relation("Quizzes").
			Scan(t.Context())
		require.NoError(t, err)

		require.Len(t, testRetrieved.Quizzes, 0)
	})
}

func TestTestHandler_List(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		test := models.Test{
			Name: rand.Text(),
		}
		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/test?page=0&size=1",
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)

		var bodyContract contracts.ListTestsResponse
		err = json.NewDecoder(rec.Body).Decode(&bodyContract)
		require.NoError(t, err)

		require.Len(t, bodyContract.Tests, 1)
		require.EqualValues(t, 1, bodyContract.Total)
		require.Equal(t, test.Name, bodyContract.Tests[0].Name)
		require.Equal(t, test.UUID, bodyContract.Tests[0].UUID)
	})

	t.Run("Bad args", func(t *testing.T) {
		t.Run("Page -", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/test?page=-1&size=1",
				nil,
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, 422, rec.Code)
		})

		t.Run("Size -", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/test?page=1&size=-1",
				nil,
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, 422, rec.Code)
		})

		t.Run("Size 0", func(t *testing.T) {

			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.New(charmlog.New(os.Stderr)))
			do.Provide(i, testrunner.NewManager)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			test := models.Test{
				Name: rand.Text(),
			}
			db := do.MustInvoke[*bun.DB](i)
			RegisterM2M(db)

			err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodGet,
				"/api/v1/test?page=1&size=0",
				nil,
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, 422, rec.Code)
		})
	})
}

func TestTestHandler_Start(t *testing.T) {

	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.New(charmlog.NewWithOptions(os.Stderr, charmlog.Options{
			Level: charmlog.DebugLevel,
		})))

		do.Provide(i, testrunner.NewManager)
		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		m := do.MustInvoke[*testrunner.Manager](i)

		db := do.MustInvoke[*bun.DB](i)
		RegisterM2M(db)

		test := models.Test{
			Name: rand.Text(),
		}
		err = db.NewInsert().Model(&test).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		group := models.Group{
			Name: rand.Text(),
		}
		err = db.NewInsert().Model(&group).Returning("*").Scan(t.Context())
		require.NoError(t, err)

		d := mrand.N(5 * time.Hour)

		durStr := d.String()

		t.Log(durStr)

		reqJSON, err := json.Marshal(contracts.StartRequest{
			TestUUID:    test.UUID,
			Duration:    durStr,
			GroupsUUIDs: uuid.UUIDs{group.UUID},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/test/running/start",
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		all := m.GetAll()
		require.Len(t, all, 1)
		tr, ok := m.Get(all[0])
		require.True(t, ok)

		require.NoError(t, err)
		require.Equal(t, tr.Test(), test.UUID)
		require.WithinDuration(t, tr.Deadline(), time.Now().Add(d), time.Minute)
	})
}
