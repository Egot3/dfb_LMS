package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strconv"
	"time"

	jwtutils "github.com/egot3/fathom/internal/JWTutils"
	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/contracts"
	exportutlis "github.com/egot3/fathom/internal/exportUtlis"
	"github.com/egot3/fathom/internal/logging"
	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/quiz"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"go.yaml.in/yaml/v4"
)

// AddQuizzes implements [Service].
func (c *chiService) AddQuizzes(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve test uuid"})
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.AddQuizzesToTestRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(req.QuizUUIDs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	err = c.testRepo.BundleQuizzesToTest(ctx, testUUID, req.QuizUUIDs)
	if err != nil {
		logger.Error("couldn't append quizzes to test",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't append quizzes to test test"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteTest implements [Service].
func (c *chiService) DeleteTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve test uuid"})
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	err := c.testRepo.DeleteTest(ctx, testUUID)
	if err != nil {
		logger.Error("couldn't retrive test", slog.String("Error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "requested test not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ExtendTest implements [Service].
func (c *chiService) ExtendTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.ExtendTestRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger = logger.With(slog.String("extend_by", req.ExtendBy))
	ctx = logging.WithLogger(ctx, logger)

	extendBy, err := time.ParseDuration(req.ExtendBy)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()}) //always parseError
		return
	}
	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()}) // my own error
		return
	}

	err = tr.ExtendTime(extendBy)
	if err != nil {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()}) // my own error
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTest implements [Service].
func (c *chiService) GetTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve test uuid"})
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	test, err := c.testRepo.Test(ctx, testUUID)
	if err != nil {
		logger.Error("couldn't retrive test", slog.String("Error", err.Error()))
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "requested test not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.GetTestResponse{Test: test})
}

// PatchTest implements [Service].
func (c *chiService) PatchTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve test uuid"})
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.PatchTestRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	if req.Name == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	name := *req.Name
	if len(name) < 3 {
		logger.Info("Attempt to create testt with invalid nickname",
			slog.String("test_name", name),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "test name is too short"})
		return
	}
	if len(name) > 255 {
		logger.Info("Attempt to create test with invalid nickname",
			slog.String("test_name", name),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "test name is too big"})
		return
	}

	err = c.testRepo.UpdateTest(ctx, testUUID, name)
	if err != nil {
		logger.Info("couldn't create test",
			slog.String("Error", err.Error()),
		)
		if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(conflict.JSONError())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "requested test not found"})
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create test"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PauseTest implements [Service].
func (c *chiService) PauseTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	// there is like nothing to return
	w.Header().Set("Content-Type", "application/json")

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))

	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()})
		return
	}

	err = tr.Pause()
	if err != nil {
		logger.Error("couldn't pause test", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PostTest implements [Service].
func (c *chiService) PostTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	var req contracts.PostTestRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger = logger.With(slog.String("test_name", req.Name))
	ctx = logging.WithLogger(ctx, logger)

	if len(req.Name) < 3 {
		logger.Info("Attempt to create testt with invalid nickname",
			slog.String("test_name", req.Name),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "test name is too short"})
		return
	}
	if len(req.Name) > 255 {
		logger.Info("Attempt to create test with invalid nickname",
			slog.String("test_name", req.Name),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "test name is too big"})
		return
	}

	test, err := c.testRepo.CreateTest(ctx, req.Name)
	if err != nil {
		logger.Info("couldn't create test",
			slog.String("Error", err.Error()),
		)
		if conflict, ok := errors.AsType[*carefulness.Conflict](err); ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(conflict.JSONError())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create test"})
		return
	}

	if req.Quizzes != nil {
		err := c.testRepo.BundleQuizzesToTest(ctx, test.UUID, req.Quizzes)
		if err != nil {
			logger.Info("couldn't add quizzes to test",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't add quizzes to test"})
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveQuizzes implements [Service].
func (c *chiService) RemoveQuizzes(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve test uuid"})
		return
	}
	logger = logger.With(slog.String("test_uuid", testUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.RemoveQuizzesRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = c.testRepo.PruneQuizzesFromTest(ctx, testUUID, req.QuizUUIDs)
	if err != nil {
		logger.Info("couldn't prune quizzes from test",
			slog.String("Error", err.Error()),
		)
		if notFound, ok := errors.AsType[*carefulness.NotInTestError](err); ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(notFound.JSONError())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "none of the quizzes is in test"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't delete quiz from test"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ResumeTest implements [Service].
func (c *chiService) ResumeTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	// there is like nothing to return
	w.Header().Set("Content-Type", "application/json")

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))

	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()})
		return
	}

	err = tr.Resume()
	if err != nil {
		logger.Error("couldn't resume test", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StartTest implements [Service].
func (c *chiService) StartTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	var req contracts.StartRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("Failed to parse body",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, carefulness.ErrMalformedRequest) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

			return
		}
		if errors.Is(err, carefulness.ErrUnprocessableRequest) {
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

			return
		}
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)

			return
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logger = logger.With(slog.String("duration", req.Duration), slog.Any("requested test", req.TestUUID))
	ctx = logging.WithLogger(ctx, logger)

	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()}) //always parseError
		return
	}

	test, err := c.testRepo.Test(ctx, req.TestUUID)
	if err != nil {
		logger.Error("couldn't get pathes for test",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't find all pathes for quizzes"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get pathes for quizzes"})
		return
	}

	do, err := c.groupRepo.GroupsExist(ctx, req.GroupsUUIDs)
	if err != nil {
		logger.Error("couldn't check if groups exist",
			slog.String("Error", err.Error()),
		)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't check if groups exist"})
		return
	}

	if !do {
		logger.Info("some of requested groups don't exist")

		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "some of requested groups don't exist"})
		return
	}

	quizPathes := make([]string, len(test.Quizzes))
	quizUUIDs := make(uuid.UUIDs, len(test.Quizzes))
	lo.ForEach(test.Quizzes, func(quiz models.Quiz, i int) {
		quizPathes[i] = quiz.Path
		quizUUIDs[i] = quiz.UUID
	})

	_, err = c.manager.Start(ctx, duration, quizPathes, quizUUIDs, req.GroupsUUIDs, test.UUID)
	if err != nil {
		logger.Error("unable to start test", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)                                            // all returned errors are user dependant anyways
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to start test"}) // all errors are user readable anyway
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StopTest implements [Service].
func (c *chiService) StopTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	w.Header().Set("Content-Type", "application/json")

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))

	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()})
		return
	}

	tr.Stop()

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) GetQuizFromRunning(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	quizUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to get uuid"})
		return
	}

	logger = logger.With(slog.String("quizUUID", quizUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))

	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()})
		return
	}

	quizC, err := tr.Get(quizUUID)
	if err != nil {
		logger.Error("couldn't retrive quiz", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
		return
	}

	etag := strconv.FormatUint(quizC.Checksum, 10)
	deadline := tr.Deadline()

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d, must-revalidate", int(math.Round(time.Until(deadline).Hours()))))

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified) // caching goes brr
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.GetQuizFromRunningResponse{
		Quiz: quiz.Quiz{
			Meta:    quizC.Meta,
			Title:   quizC.Title,
			Body:    quizC.Body,
			UUID:    quizC.UUID,
			Options: quizC.Options,
		},
	})
}

func (c *chiService) ListTests(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Set("Content-Type", "application/json")

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Failed to parse form data"})
		return
	}

	pageInt, err := strconv.Atoi(r.Form.Get("page"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "given form page is not a number"})
		return
	}
	sizeInt, err := strconv.Atoi(r.Form.Get("size"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "given form size is not a number"})
		return
	}
	if sizeInt <= 0 {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "size can't be <= 0"})
		return
	}
	if pageInt < 0 {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "page can't be less than zero"})
	}

	logger.With(slog.Int("page", pageInt), slog.Int("size", sizeInt))

	claims, ok := (r.Context().Value("claims")).(jwtutils.Claims)
	if !ok {
		logger.Error("Failed to retrieve jwt claims")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve jwt's claims"})
		return
	}

	var tests []models.Test
	var total int
	if claims.IsTeacher {
		logger.Debug("got teacher request")
		tests, total, err = c.testRepo.ListTestsAdvanced(ctx, pageInt, sizeInt)
		logger.Debug("got advanced tests info", slog.Any("tests", tests))
	} else {
		logger.Debug("got pupil request", slog.Any("claims", claims))
		tests, total, err = c.testRepo.ListTests(ctx, pageInt, sizeInt)
	}

	if err != nil {
		logger.Error("couldn't select tests for listing", slog.String("Error", err.Error()))
		if gone, ok := errors.AsType[carefulness.Gone](err); ok {
			w.WriteHeader(http.StatusGone)
			json.NewEncoder(w).Encode(gone.JSONError())
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.ListTestsResponse{
		Tests: tests,
		Total: total,
		Page:  pageInt,
		Size:  sizeInt,
	})
}

// ExportTest implements [Service].
func (c *chiService) ExportTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	testUUID, err := uuid.Parse(chi.URLParam(r, "test_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
	}

	logger = logger.With(
		slog.String("test_uuid", testUUID.String()),
	)

	var req contracts.ExportTestRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("error in register during reading",
			slog.String("error", err.Error()),
		)
		switch {
		case errors.Is(err, carefulness.ErrMalformedRequest):
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest.JSONError())

		case errors.Is(err, carefulness.ErrUnprocessableRequest):
			w.WriteHeader(422)
			json.NewEncoder(w).Encode(carefulness.ErrUnprocessableRequest.JSONError())

		case errors.Is(err, io.EOF):
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Empty body"})

		case errors.Is(err, io.ErrUnexpectedEOF):
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Data loss"})

		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	logger = logger.With(
		slog.String("description", req.Description),
	)
	ctx = logging.WithLogger(ctx, logger)

	test, err := c.testRepo.Test(ctx, testUUID)
	if err != nil {
		logger.Error("couldn't select test",
			slog.String("Error", err.Error()),
		)
		if gone, ok := errors.AsType[carefulness.Gone](err); ok {
			w.WriteHeader(http.StatusGone)
			json.NewEncoder(w).Encode(gone.JSONError())
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	yamlTest := exportutlis.YamlTest{
		Kind:        exportutlis.Kind(exportutlis.Test),
		UUID:        testUUID,
		Name:        test.Name,
		Description: req.Description,
		Quizzes: lo.Map(test.Quizzes, func(quiz models.Quiz, _ int) exportutlis.YamlQuiz {
			return exportutlis.YamlQuiz{
				Kind: exportutlis.Kind(exportutlis.Quiz),
				UUID: quiz.UUID,
			}
		}),
	}

	out, err := yaml.Marshal(yamlTest)
	if err != nil {
		logger.Error("couldn't marshal test yaml",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create the YAML file for tests"})
		return
	}

	w.Header().Set("Content-Type", "application/yaml")
	w.Write(out)
}

// ImportTest implements [Service].
func (c *chiService) ImportTest(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Add("content-type", "application/json")

	if err := r.ParseMultipartForm(8 << 20); err != nil {
		logger.Error("archive is too big", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "archive is too big!"})
		return
	}

	yamlFile, handler, err := r.FormFile("imported")
	if err != nil {
		logger.Error("couldn't get file", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to parse file"})
		return
	}
	defer yamlFile.Close()

	contentType, _, err := mime.ParseMediaType(handler.Header.Get("Content-Type"))
	if err != nil {
		logger.Error("couldn't parse MIME type", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to parse MIME"})
		return
	}
	if contentType != "application/yaml" {
		w.WriteHeader(http.StatusNotAcceptable)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: fmt.Sprintf("unsupported media type: %v", contentType)})
		return
	}

	var test exportutlis.YamlTest
	err = yaml.NewDecoder(yamlFile).Decode(&test)
	if err != nil {
		logger.Error("couldn't parse yaml file", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to parse file"})
		return
	}

	e, err := c.testRepo.ExistsByUUID(ctx, test.UUID)
	if err != nil {
		logger.Error("couldn't check test existance", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to check test existanse"})
		return
	}
	if e {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "it's either: this test already exists(probable) or you hit 1 in 18.8 sextillion chance in uuidv7 collision, either way, you address it"})
		return
	}

	for _, q := range test.Quizzes {
		e, err := c.quizRepo.ExistsByUUID(ctx, q.UUID)
		if err != nil {
			logger.Error("couldn't check quiz existance", slog.String("Error", err.Error()))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to check quiz existanse"})
			return
		}
		if !e {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "quiz from test is not found on local machine, have you imported quiz bank?"})
			return
		}
	}

	err = c.testRepo.ImportTest(ctx, test)
	if err != nil {
		logger.Error("couldn't check import test to db", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't check import test to db"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) GetRunningQuizzesUUIDs(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	w.Header().Set("Content-Type", "application/json")

	runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
		return
	}
	logger = logger.With(slog.Uint64("runner_key", runnerKey))

	tr, ok := c.manager.Get(runnerKey)
	if !ok {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: testrunner.ErrRunnerInactive.Error()})
		return
	}

	checksum := tr.Checksum()
	if checksum == 0 {
		w.WriteHeader(http.StatusLocked)
		return
	}

	etag := strconv.FormatUint(checksum, 10)
	deadline := tr.Deadline()

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d, must-revalidate", int(math.Round(time.Until(deadline).Hours()))))

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified) // caching goes brrrrr
		return
	}

	uuids := tr.GetAll()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.GetQuizzesUUIDs{
		UUIDs: uuids,
	})
}

func (c *chiService) RunningInfo(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	w.Header().Set("Content-Type", "application/json")
	ctx := logging.WithLogger(r.Context(), logger)

	testUUIDs := c.manager.AllTests()
	keys := c.manager.GetAll()

	tests, err := c.testRepo.Tests(ctx, testUUIDs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't select running test info"})
		return
	}

	testInfos := make([]contracts.RunningInfo, len(tests))
	for i, test := range tests {
		tr, ok := c.manager.Get(keys[i])
		if !ok {
			continue
		}

		testInfos[i] = contracts.RunningInfo{
			Key:      keys[i],
			Deadline: tr.Deadline(),
			IsPaused: tr.IsPaused(),
			Test:     test,
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.RunningInfoResponse{
		TestInfos: testInfos,
	})
}

func (c *chiService) GetDeadline(key uint64) (time.Time, error) {
	tr, ok := c.manager.Get(key)
	if !ok {
		return time.Time{}, testrunner.ErrRunnerInactive
	}

	return tr.Deadline(), nil
}

func (c *chiService) GetTestUUID(key uint64) uuid.UUID {
	tr, ok := c.manager.Get(key)
	if !ok {
		return uuid.Nil
	}

	return tr.Test()
}
