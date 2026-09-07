package middlewares

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// requires "test_uuid" in URL
func TestNotRunning(uuidGetter func() uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logging.LoggerFromContext(r.Context())
			logger = logger.With(slog.String("layer", "middleware"))

			if chi.URLParam(r, "test_uuid") == uuidGetter().String() {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func Running(uuidGetter func(uint64) uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logging.LoggerFromContext(r.Context())
			logger = logger.With(slog.String("layer", "middleware"))

			runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
				return
			}
			logger = logger.With(slog.Uint64("runner_key", runnerKey))

			logger.Debug("checking if test uuid is running")
			if uuidGetter(runnerKey) != uuid.Nil {
				next.ServeHTTP(w, r)
				return
			}

			logger.Debug("test uuid is not running")
			w.WriteHeader(http.StatusLocked)
		})
	}
}

// requires quiz_uuid as URL param
func QuizNotRunning(checker func(uuid.UUID) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logging.LoggerFromContext(r.Context())
			logger = logger.With(slog.String("layer", "middleware"))

			uuid, err := uuid.Parse(chi.URLParam(r, "quiz_uuid"))
			if err != nil {
				logger.Error("couldn't parse requested quizUUID", slog.String("Error", err.Error()))
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't parse requested quizUUID"})
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			logger.Debug("checking if quiz uuid is in running")
			if checker(uuid) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			logger.Debug("it's'n't running, proceed")
			next.ServeHTTP(w, r)
		})
	}
}
