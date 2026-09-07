package middlewares

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	jwtutils "github.com/egot3/fathom/internal/JWTutils"
	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/logging"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

type generation struct {
	mu       sync.RWMutex
	testUUID uuid.UUID
	allowed  map[uuid.UUID]bool
	sfg      singleflight.Group
}

type userCache struct {
	gen atomic.Pointer[generation]
}

func (c *userCache) currentGeneration(testUUID uuid.UUID) *generation {
	for {
		g := c.gen.Load()
		if g != nil && g.testUUID == testUUID {
			return g
		}
		newGen := &generation{testUUID: testUUID}
		if c.gen.CompareAndSwap(g, newGen) {
			return newGen
		}
	}
}

// BEHOLD, THE HOLY CODE
// Most iterated function
func IsInGroup(testUUIDGetter func(uint64) uuid.UUID, allowedChecker func(context.Context, uuid.UUID, uint64) (bool, error), deadlineGetter func(uint64) (time.Time, error)) func(http.Handler) http.Handler {
	cache := &userCache{}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := logging.LoggerFromContext(r.Context()).With(slog.String("layer", "middleware"))

			runnerKey, err := strconv.ParseUint(chi.URLParam(r, "runnerKey"), 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(carefulness.ErrMalformedRequest)
				return
			}
			logger = logger.With(slog.Uint64("runner_key", runnerKey))

			upd := testUUIDGetter(runnerKey)
			if upd == uuid.Nil {
				w.WriteHeader(http.StatusLocked)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "No test is running"})
				return
			}

			g := cache.currentGeneration(upd)

			claims, ok := (r.Context().Value("claims")).(jwtutils.Claims)
			if !ok {
				logger.Error("Failed to retrieve jwt claims")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve jwt's claims"})
				return
			}

			userUUID := claims.UserID

			g.mu.Lock()
			if g.testUUID != upd {
				g.testUUID = upd
				g.allowed = make(map[uuid.UUID]bool)
			}

			is, ok := g.allowed[userUUID]
			g.mu.Unlock()

			if !ok {
				d, err := deadlineGetter(runnerKey)
				if err != nil {
					w.WriteHeader(http.StatusLocked)
					return
				}
				if time.Until(d) <= 5*time.Second {
					w.WriteHeader(http.StatusForbidden)
					json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't register new contestants 5s before the end"})
					return
				}

				allowVal, err, _ := g.sfg.Do(userUUID.String(), func() (any, error) {
					g.mu.RLock()
					res, ex := g.allowed[userUUID]
					g.mu.RUnlock()

					if ex {
						return res, nil
					}

					key, err := strconv.ParseUint(r.PathValue("runnerKey"), 10, 64)
					if err != nil {
						return false, err
					}

					allow, fetchErr := allowedChecker(r.Context(), userUUID, key)
					if fetchErr != nil {
						return false, fetchErr
					}

					g.mu.Lock()
					g.allowed[userUUID] = allow
					g.mu.Unlock()

					return allow, nil
				})

				if err != nil {
					logger.Error("Couldn't check if user is in allowed group",
						slog.String("userUUID", userUUID.String()),
					)
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't check if user is in allowed group"})
					return
				}

				is = allowVal.(bool)
			}

			if !is {
				logger.Info("User wasn't found in group")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Can't join the test, as user was not in the allowed group during first request"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
