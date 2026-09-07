package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/contracts"
	"github.com/egot3/fathom/internal/logging"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// GetAnswer implements [Service].
func (c *chiService) GetAnswer(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse userUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	quizUUID, err := uuid.Parse(chi.URLParam(r, "quiz_uuid"))
	if err != nil {
		logger.Error("couldn't parse quizUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	testUUID, err := uuid.Parse(chi.URLParam(r, "test_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("group_uuid", groupUUID.String()),
		slog.String("user_uuid", userUUID.String()),
		slog.String("test_uuid", testUUID.String()),
		slog.String("quiz_uuid", quizUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	answer, err := c.answerRepo.Answer(ctx, userUUID, testUUID, groupUUID, quizUUID)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested answer is not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	correct, err := c.quizRepo.CorrectAnswer(ctx, quizUUID)
	if err != nil {
		logger.Error("couldn't get the right answer",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to get correct answer"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.AnswerResponse{
		Answer: contracts.Answer{
			GroupUUID: groupUUID,
			TestUUID:  testUUID,
			UserUUID:  userUUID,
			QuizUUID:  quizUUID,
			Chosen:    answer,
			Correct:   correct,
		},
	})
}

// GetGroupTotals implements [Service].
func (c *chiService) GetGroupTotals(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	testUUID, err := uuid.Parse(chi.URLParam(r, "test_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("group_uuid", groupUUID.String()),
		slog.String("test_uuid", testUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	groupTotals, err := c.answerRepo.GroupTestTotals(ctx, testUUID, groupUUID)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested answer is not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	logger.Info("group totals retrieved",
		slog.String("groupTotals", fmt.Sprintf("%+v", groupTotals)),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.TotalsResponse{
		Totals: groupTotals,
	})
}

// GetTestTotals implements [Service].
func (c *chiService) GetTestTotals(w http.ResponseWriter, r *http.Request) {
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
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("test_uuid", testUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	testTotals, err := c.answerRepo.TestTotals(ctx, testUUID)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested answer is not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.TotalsResponse{
		Totals: testTotals,
	})
}

// GetUserTotal implements [Service].
func (c *chiService) GetUserTotal(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	testUUID, err := uuid.Parse(chi.URLParam(r, "test_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("group_uuid", groupUUID.String()),
		slog.String("user_uuid", userUUID.String()),
		slog.String("test_uuid", testUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	userTotal, err := c.answerRepo.Total(ctx, userUUID, testUUID, groupUUID)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested answer is not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.TotalResponse{
		Total: userTotal,
	})
}

// GetUserTotals implements [Service].
func (c *chiService) GetUserTotals(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("user_uuid", userUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	err = r.ParseForm()
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

	logger = logger.With(slog.Int("page", pageInt), slog.Int("size", sizeInt))

	userTotals, total, err := c.answerRepo.UserTotals(ctx, userUUID, pageInt, sizeInt)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "there is no totals to list"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.TotalsResponse{
		Totals: userTotals,
		Total:  total,
		Page:   pageInt,
		Size:   sizeInt,
	})
}

// PostAnswer implements [Service].
func (c *chiService) PostAnswer(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	quizUUID, err := uuid.Parse(chi.URLParam(r, "quiz_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("quiz_uuid", quizUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.PostAnswerRequest
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

	q, err := tr.Get(quizUUID)
	if err != nil {
		logger.Error("couldn't get quiz",
			slog.String("Error", err.Error()),
		)
		switch {
		case errors.Is(err, testrunner.ErrQuizNotCached):
			w.WriteHeader(http.StatusNotFound)
		case errors.Is(err, testrunner.ErrRunnerInactive):
			w.WriteHeader(http.StatusLocked)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}

		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
		return
	}

	score := q.EvaluateScore(req.Value)

	answered, err := json.Marshal(req.Value)
	if err != nil {
		logger.Error("couldn't marshal answer to json(how?)",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusTeapot)
		return
	}
	err = c.answerRepo.SetAnswer(ctx, tr.Test(), groupUUID, userUUID, quizUUID, string(answered), score)
	if err != nil {
		logger.Error("couldn't set an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't set an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Totalize implements [Service].
func (c *chiService) Totalize(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse userUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	currentTestUUID := tr.Test()
	logger = logger.With(
		slog.String("group_uuid", groupUUID.String()),
		slog.String("user_uuid", userUUID.String()),
		slog.String("currently_running_test_uuid", currentTestUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	err = c.answerRepo.Totalize(ctx, userUUID, currentTestUUID, groupUUID)
	if err != nil {
		logger.Error("couldn't totalize",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't totalize because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) ListTotals(w http.ResponseWriter, r *http.Request) {
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

	logger = logger.With(slog.Int("page", pageInt), slog.Int("size", sizeInt))

	totals, total, err := c.answerRepo.ListTotals(ctx, pageInt, sizeInt)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "there is no totals to list"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.TotalsResponse{
		Totals: totals,
		Total:  total,
		Page:   pageInt,
		Size:   sizeInt,
	})
}

func (c *chiService) ListUserAnswer(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, err := uuid.Parse(chi.URLParam(r, "group_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	userUUID, err := uuid.Parse(chi.URLParam(r, "user_uuid"))
	if err != nil {
		logger.Error("couldn't parse groupUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	testUUID, err := uuid.Parse(chi.URLParam(r, "test_uuid"))
	if err != nil {
		logger.Error("couldn't parse testUUID in url",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	logger = logger.With(
		slog.String("group_uuid", groupUUID.String()),
		slog.String("user_uuid", userUUID.String()),
		slog.String("test_uuid", testUUID.String()),
	)
	ctx = logging.WithLogger(ctx, logger)

	err = r.ParseForm()
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

	logger = logger.With(slog.Int("page", pageInt), slog.Int("size", sizeInt))

	userAnswers, total, err := c.answerRepo.AnswersInTest(ctx, userUUID, testUUID, groupUUID, pageInt, sizeInt)
	if err != nil {
		logger.Error("couldn't get an answer",
			slog.String("Error", err.Error()),
		)

		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "there is no totals to list"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't get an answer because of unknown error"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.AnswersResponse{
		Answers: userAnswers,
		Total:   total,
		Page:    pageInt,
		Size:    sizeInt,
	})
}
