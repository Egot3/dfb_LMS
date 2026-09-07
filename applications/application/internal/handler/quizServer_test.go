package handler_test

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egot3/fathom/internal/config"
	"github.com/egot3/fathom/internal/contracts"
	"github.com/egot3/fathom/internal/database/repositories"
	"github.com/egot3/fathom/internal/handler"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/quiz"
	quizparser "github.com/egot3/fathom/internal/quizParser"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/egot3/fathom/internal/testutils"
	"github.com/egot3/fathom/server"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestQuizHandler_Post(t *testing.T) {

	dir := t.TempDir()

	err := os.Mkdir(dir+"/quizzes", 0750)
	require.NoError(t, err)

	os.Setenv("DATA_DIRECTORY", dir)

	i := testutils.NewTestInjector(t,
		repositories.RepositoryPackage,
	)
	cfg := do.MustInvoke[*config.Config](i)

	t.Run("Valid", func(t *testing.T) {

		i := i.Scope("valid")

		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		name := rand.Text()
		body :=
			"# Sky color\n\nWhat color is da sky?\n\n[depends]"
		reqJSON, _ := json.Marshal(contracts.PostQuizRequest{
			Name: name,
			Body: body,
			Meta: quiz.Frontmatter{
				Kind:  quiz.Input,
				Score: 1,
			},
		})
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/quiz/",
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)

		path, err := cfg.TurnToAbs(name)
		require.NoError(t, err)
		t.Logf("Filepath: %v", path)
		require.FileExists(t, path)
		b, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Contains(t, string(b), body, string(b))

		err = os.Remove(path)
		require.NoError(t, err)
	})

	t.Run("Orphans", func(t *testing.T) {
		meta := quiz.Frontmatter{
			Kind:  quiz.Input,
			Score: 1,
		}
		testCases := []struct {
			desc string
			name string
			body string
			meta quiz.Frontmatter
		}{
			{
				desc: "No name",
				name: "",
				body: rand.Text(),
				meta: meta,
			},
			{
				desc: "No body",
				name: rand.Text(),
				body: "",
				meta: meta,
			},
			{
				desc: "No meta",
				name: rand.Text(),
				body: rand.Text(),
				meta: quiz.Frontmatter{},
			},
			{
				desc: "Nothing",
				name: "",
				body: "",
				meta: quiz.Frontmatter{},
			},
		}
		for _, tC := range testCases {
			t.Run(tC.desc, func(t *testing.T) {

				i := testutils.NewTestInjector(t,
					repositories.RepositoryPackage,
				)
				do.ProvideValue(i, slog.Default())
				do.Provide(i, testrunner.NewManager)

				do.Provide(i, handler.NewTestService)
				router, err := server.ChiServer(i)
				require.NoError(t, err)

				reqJSON, _ := json.Marshal(contracts.PostQuizRequest{
					Name: tC.name,
					Body: tC.body,
					Meta: tC.meta,
				})
				req := httptest.NewRequest(
					http.MethodPost,
					"/api/v1/quiz/",
					bytes.NewReader(reqJSON),
				)
				req.Header.Set("Content-Type", "application/json")
				testutils.AddTeacherCookie(t, req)
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				bodyString := rec.Body.String()
				require.Equal(t, http.StatusBadRequest, rec.Code, bodyString)
			})
		}
	})

	t.Run("Conflict!", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		name := rand.Text()
		db := do.MustInvoke[*bun.DB](i)
		cfg := do.MustInvoke[*config.Config](i)

		path, err := cfg.TurnToAbs(name)
		require.NoError(t, err)
		_, err = db.NewInsert().Model(&models.Quiz{
			Path:          path,
			Checksum:      [8]byte{},
			Score:         1,
			CorrectAnswer: "omega",
		}).Exec(t.Context())

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		body := `# quiz!
			 there is a body!
			 [yeah!]`
		reqJSON, err := json.Marshal(contracts.PostQuizRequest{
			Name: name,
			Body: body,
			Meta: quiz.Frontmatter{
				Kind:  quiz.Input,
				Score: 1,
			},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/quiz/",
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusConflict, rec.Code, bodyString)
		require.NoFileExists(t, path, bodyString)
	})
}

func TestQuizHandler_Get(t *testing.T) {

	i := testutils.NewTestInjector(t,
		repositories.RepositoryPackage,
	)
	do.ProvideValue(i, slog.Default())
	do.Provide(i, testrunner.NewManager)

	f := testutils.TestQuiz(t)
	defer f.Close()

	var quizUUID uuid.UUID
	score := 1
	db := do.MustInvoke[*bun.DB](i)
	err := db.NewInsert().Model(&models.Quiz{
		Path:          f.Name(),
		Checksum:      [8]byte{},
		Score:         score,
		CorrectAnswer: "x",
	}).Returning("uuid").
		Scan(t.Context(), &quizUUID)
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {

		i = i.Scope("valid")

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusOK, rec.Code, bodyString)

		var quizS contracts.GetQuizResponse
		err = json.Unmarshal(rec.Body.Bytes(), &quizS)
		require.NoError(t, err)

		require.Equal(t, score, quizS.Meta.Score)
		require.Contains(t, quizS.Body, "[yeah!]")
	})

	t.Run("Invalid", func(t *testing.T) {

		i = i.Scope("invalid")

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/quiz/%v", uuid.Max),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)
	})
}

func TestQuizHandler_List(t *testing.T) {

	i := testutils.NewTestInjector(t,
		repositories.RepositoryPackage,
	)
	do.ProvideValue(i, slog.Default())
	do.Provide(i, testrunner.NewManager)

	f := testutils.TestQuiz(t)
	defer f.Close()

	var quizUUID uuid.UUID
	score := 1
	db := do.MustInvoke[*bun.DB](i)
	err := db.NewInsert().Model(&models.Quiz{
		Path:          f.Name(),
		Checksum:      [8]byte{},
		Score:         score,
		CorrectAnswer: "x",
	}).Returning("uuid").
		Scan(t.Context(), &quizUUID)
	require.NoError(t, err)

	do.Provide(i, handler.NewTestService)
	router, err := server.ChiServer(i)
	require.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/quiz?page=0&size=1",
		nil,
	)
	req.Header.Set("Content-Type", "application/json")
	testutils.AddTeacherCookie(t, req)

	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	bodyString := rec.Body.String()
	require.Equal(t, http.StatusOK, rec.Code, bodyString)

	var quizL contracts.ListQuizResponse
	err = json.Unmarshal(rec.Body.Bytes(), &quizL)
	require.NoError(t, err)

	require.Len(t, quizL.Quizzes, 1)
	require.Equal(t, quizUUID, quizL.Quizzes[0].UUID)
	require.Equal(t, score, quizL.Quizzes[0].Score)
	require.Equal(t, "x", quizL.Quizzes[0].CorrectAnswer)
}

func TestQuizHandler_Delete(t *testing.T) {

	i := testutils.NewTestInjector(t,
		repositories.RepositoryPackage,
	)
	do.ProvideValue(i, slog.Default())
	do.Provide(i, testrunner.NewManager)

	f := testutils.TestQuiz(t)
	defer f.Close()

	var quizUUID uuid.UUID
	score := 1
	db := do.MustInvoke[*bun.DB](i)
	err := db.NewInsert().Model(&models.Quiz{
		Path:          f.Name(),
		Checksum:      [8]byte{},
		Score:         score,
		CorrectAnswer: "x",
	}).Returning("uuid").
		Scan(t.Context(), &quizUUID)
	require.NoError(t, err)

	t.Run("Valid", func(t *testing.T) {

		i = i.Scope("valid")

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)

		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNoContent, rec.Code, bodyString)
	})

	t.Run("Invalid", func(t *testing.T) {

		i = i.Scope("invalid")

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("/api/v1/quiz/%v", uuid.Max),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)
	})
}

func TestQuizHandler_Patch(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {

		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)
		cfg := do.MustInvoke[*config.Config](i)

		file := testutils.TestQuiz(t)
		defer file.Close()

		var quizUUID uuid.UUID
		score := 1
		db := do.MustInvoke[*bun.DB](i)
		err := db.NewInsert().Model(&models.Quiz{
			Path:          file.Name(),
			Checksum:      [8]byte{},
			Score:         score,
			CorrectAnswer: "x",
		}).Returning("uuid").
			Scan(t.Context(), &quizUUID)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		tmp, err := os.CreateTemp("", "*.md")
		require.NoError(t, err)
		defer tmp.Close()

		newName := strings.TrimSuffix(tmp.Name(), ".md")
		newScore := mrand.IntN(25) + 1

		reqJSON, _ := json.Marshal(contracts.PatchQuizRequest{
			Name: &newName,
			Meta: &quiz.Frontmatter{
				Score: newScore,
				Kind:  quiz.Input,
			},
		})
		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		newPath, err := cfg.TurnToAbs(newName)
		require.NoError(t, err)
		require.FileExists(t, newPath)
		defer os.Remove(newPath)

		var scoreCheck int
		err = db.NewSelect().Model((*models.Quiz)(nil)).
			Where("uuid = ?", quizUUID).Column("score").
			Scan(t.Context(), &scoreCheck)

		require.Equal(t, newScore, scoreCheck)

		q, err := quizparser.ParseQuiz(file)
		require.NoError(t, err)

		_, err = file.Seek(0, io.SeekStart)
		require.NoError(t, err)

		buf := []byte{}
		_, err = file.Read(buf)
		require.NoError(t, err)

		require.Equal(t, newScore, q.Meta.Score, string(buf))

	})

	t.Run("Zeros", func(t *testing.T) {
		t.Run("Name", func(t *testing.T) {
			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.Default())
			do.Provide(i, testrunner.NewManager)

			file := testutils.TestQuiz(t)
			defer file.Close()

			path, err := filepath.Abs(file.Name())
			require.NoError(t, err)

			var quizUUID uuid.UUID
			score := 1
			db := do.MustInvoke[*bun.DB](i)
			err = db.NewInsert().Model(&models.Quiz{
				Path:          path,
				Checksum:      [8]byte{},
				Score:         score,
				CorrectAnswer: "x",
			}).Returning("uuid").
				Scan(t.Context(), &quizUUID)
			require.NoError(t, err)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			newName := ""
			newScore := mrand.IntN(25) + 1

			reqJSON, _ := json.Marshal(contracts.PatchQuizRequest{
				Name: &newName,
				Meta: &quiz.Frontmatter{
					Score: newScore,
					Kind:  quiz.Input,
				},
			})
			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNoContent, rec.Code)
		})

		t.Run("Score", func(t *testing.T) {
			i := testutils.NewTestInjector(t,
				repositories.RepositoryPackage,
			)
			do.ProvideValue(i, slog.Default())
			do.Provide(i, testrunner.NewManager)

			file := testutils.TestQuiz(t)
			defer file.Close()

			path, err := filepath.Abs(file.Name())
			require.NoError(t, err)

			var quizUUID uuid.UUID
			score := 1
			db := do.MustInvoke[*bun.DB](i)
			err = db.NewInsert().Model(&models.Quiz{
				Path:          path,
				Checksum:      [8]byte{},
				Score:         score,
				CorrectAnswer: "x",
			}).Returning("uuid").
				Scan(t.Context(), &quizUUID)
			require.NoError(t, err)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			newName := "normalName"
			newScore := 0

			reqJSON, _ := json.Marshal(contracts.PatchQuizRequest{
				Name: &newName,
				Meta: &quiz.Frontmatter{
					Score: newScore,
					Kind:  quiz.Input,
				},
			})
			req := httptest.NewRequest(
				http.MethodPatch,
				fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
				bytes.NewReader(reqJSON),
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	})

	t.Run("Invalid score", func(t *testing.T) {
		i := testutils.NewTestInjector(t,
			repositories.RepositoryPackage,
		)
		do.ProvideValue(i, slog.Default())
		do.Provide(i, testrunner.NewManager)

		file := testutils.TestQuiz(t)
		defer file.Close()

		path, err := filepath.Abs(file.Name())
		require.NoError(t, err)

		var quizUUID uuid.UUID
		score := 1
		db := do.MustInvoke[*bun.DB](i)
		err = db.NewInsert().Model(&models.Quiz{
			Path:          path,
			Checksum:      [8]byte{},
			Score:         score,
			CorrectAnswer: "x",
		}).Returning("uuid").
			Scan(t.Context(), &quizUUID)
		require.NoError(t, err)

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		newScore := -5423

		reqJSON, _ := json.Marshal(contracts.PatchQuizRequest{
			Meta: &quiz.Frontmatter{
				Score: newScore,
				Kind:  quiz.Input,
			},
		})
		req := httptest.NewRequest(
			http.MethodPatch,
			fmt.Sprintf("/api/v1/quiz/%v", quizUUID),
			bytes.NewReader(reqJSON),
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)

	})
}

func TestQuizHandler_GetParsed(t *testing.T) {

	i := testutils.NewTestInjector(t,
		repositories.RepositoryPackage,
	)
	do.ProvideValue(i, slog.Default())
	do.Provide(i, testrunner.NewManager)

	t.Run("Valid", func(t *testing.T) {
		i = i.Scope("valid")
		t.Run("Input", func(t *testing.T) {
			i = i.Scope("input")

			f := testutils.TestQuiz(t)

			var quizUUID uuid.UUID
			score := 1
			db := do.MustInvoke[*bun.DB](i)
			err := db.NewInsert().Model(&models.Quiz{
				Path:          f.Name(),
				Checksum:      [8]byte{},
				Score:         score,
				CorrectAnswer: "x",
			}).Returning("uuid").
				Scan(t.Context(), &quizUUID)
			require.NoError(t, err)

			err = f.Close()
			require.NoError(t, err)

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/api/v1/quiz/%v/parsed", quizUUID),
				nil,
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var contract contracts.ParsedQuizResponse
			err = json.NewDecoder(rec.Body).Decode(&contract)
			require.NoError(t, err)

			require.Equal(t, score, contract.Quiz.Meta.Score)
			require.Equal(t, contract.Quiz.Body, "there is a body!")
		})

		t.Run("Not Input", func(t *testing.T) {
			i = i.Scope("notInput")

			do.Provide(i, handler.NewTestService)
			router, err := server.ChiServer(i)
			require.NoError(t, err)

			f := testutils.TestRadioQuiz(t)

			var quizUUID uuid.UUID
			score := 1
			db := do.MustInvoke[*bun.DB](i)
			err = db.NewInsert().Model(&models.Quiz{
				Path:          f.Name(),
				Checksum:      [8]byte{},
				Score:         score,
				CorrectAnswer: "x",
			}).Returning("uuid").
				Scan(t.Context(), &quizUUID)
			require.NoError(t, err)

			err = f.Close()
			require.NoError(t, err)

			req := httptest.NewRequest(
				http.MethodGet,
				fmt.Sprintf("/api/v1/quiz/%v/parsed", quizUUID),
				nil,
			)
			req.Header.Set("Content-Type", "application/json")
			testutils.AddTeacherCookie(t, req)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var contract contracts.ParsedQuizResponse
			err = json.NewDecoder(rec.Body).Decode(&contract)
			require.NoError(t, err)

			require.Equal(t, score, contract.Quiz.Meta.Score)
			require.Equal(t, contract.Quiz.Body, "what's the question?")
			require.Len(t, contract.Quiz.Options.Radio.Choices, 3)
			require.Equal(t, contract.Quiz.Answer.Radio.ChoiceIdx, 0)
		})

	})

	t.Run("Invalid", func(t *testing.T) {

		i = i.Scope("invalid")

		do.Provide(i, handler.NewTestService)
		router, err := server.ChiServer(i)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/quiz/%v/parsed", uuid.Max),
			nil,
		)
		req.Header.Set("Content-Type", "application/json")
		testutils.AddTeacherCookie(t, req)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		bodyString := rec.Body.String()
		require.Equal(t, http.StatusNotFound, rec.Code, bodyString)
	})
}
