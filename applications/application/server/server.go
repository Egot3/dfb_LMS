package server

import (
	"log/slog"
	"net/http"

	"github.com/egot3/fathom/internal/handler"
	"github.com/egot3/fathom/internal/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/samber/do/v2"
)

func ChiServer(i do.Injector) (chi.Router, error) {
	r := chi.NewRouter()
	svc := do.MustInvoke[handler.Service](i)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://localhost:5173", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Cookie", "ETag", "If-None-Match", "Session-Control"},
		ExposedHeaders:   []string{"Link", "ETag", "If-None-Match", "Session-Control"},
		AllowCredentials: true,
		MaxAge:           300,
	}), middlewares.BodySizer)

	r.Method("GET", "/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middlewares.AttachLogger(do.MustInvoke[*slog.Logger](i)))
		r.Use(middlewares.TraceAttacher)

		r.Route("/user", func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.With(middlewares.ParseUUID).Get("/{uuid}", svc.GetUser)
				r.Get("/", svc.ListUsers)
			})

			r.Group(func(r chi.Router) {
				r.Use(middlewares.JWT)
				r.Use(middlewares.ParseUUID, middleware.Maybe(middlewares.UUIDRights, func(r *http.Request) bool {
					return !middlewares.IsTeacherCondition(r)
				}))

				r.Patch("/{uuid}", svc.PatchUser)
				r.Delete("/{uuid}", svc.DeleteUser)

			})
			r.Post("/register", svc.Register)
			r.Post("/login", svc.Login)
		})

		r.Route("/group", func(r chi.Router) {
			r.Get("/", svc.ListGroups)

			r.With(middlewares.JWT, middlewares.IsTeacherRights).Post("/", svc.PostGroup)

			r.Route("/{uuid}", func(r chi.Router) {
				r.Use(middlewares.ParseUUID)
				r.Get("/", svc.GetGroup)

				r.Group(func(r chi.Router) {
					r.Use(middlewares.JWT, middlewares.IsTeacherRights)
					r.Patch("/", svc.PatchGroup)
					r.Delete("/", svc.DeleteGroup)
				})

				r.Route("/user", func(r chi.Router) {
					r.Post("/", svc.AppendUsers)
					r.Delete("/", svc.RemoveUsers)
				})
			})
		})

		r.Route("/quiz", func(r chi.Router) {
			r.Use(middlewares.JWT)

			r.With(middleware.Maybe(middlewares.QuizNotRunning(svc.IsRunning), func(r *http.Request) bool {
				return (r.Method != http.MethodOptions) || !middlewares.IsTeacherCondition(r)
			})).Get("/{quiz_uuid}/parsed", svc.ParsedQuiz)

			r.Group(func(r chi.Router) {
				r.Use(middlewares.IsTeacherRights)

				r.Get("/", svc.ListQuizzes)
				r.Post("/", svc.PostQuiz)
				r.Post("/import", svc.ImportQuizBank)
				r.Get("/export", svc.ExportQuizBank)

				r.Group(func(r chi.Router) {
					r.Use(middlewares.ParseUUID)
					r.Patch("/{uuid}", svc.PatchQuiz)
					r.Get("/{uuid}", svc.GetQuiz)
					r.Delete("/{uuid}", svc.DeleteQuiz)
				})
			})
		})

		r.Route("/test", func(r chi.Router) {
			r.Use(middlewares.JWT)

			r.Group(func(r chi.Router) {
				r.Use(middlewares.IsTeacherRights)
				r.Post("/", svc.PostTest)
				r.Post("/import", svc.ImportTest)
			})

			r.Get("/", svc.ListTests)

			r.Route("/running/{runnerKey:^[0-9]{1,20}$}", func(r chi.Router) {
				r.Use(middleware.Maybe(
					middlewares.IsInGroup(svc.GetTestUUID, svc.AllowedToTest, svc.GetDeadline), func(r *http.Request) bool {
						return !middlewares.IsTeacherCondition(r)
					}))
				r.Get("/{uuid:^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$}", svc.GetQuizFromRunning)
				r.Get("/quizzes", svc.GetRunningQuizzesUUIDs)
				r.Get("/", svc.RunningInfo)

				//protected
				r.Group(func(r chi.Router) {
					r.Use(middlewares.IsTeacherRights)
					r.Post("/stop", svc.StopTest)
					r.Post("/pause", svc.PauseTest)
					r.Post("/resume", svc.ResumeTest)
					r.Post("/extend", svc.ExtendTest)
				})

			})

			r.With(middlewares.IsTeacherRights).Post("/running/start", svc.StartTest)

			r.Route("/{uuid}", func(r chi.Router) {
				r.Use(middlewares.ParseUUID)

				r.Get("/", svc.GetTest)

				r.Group(func(r chi.Router) {
					r.Use(middlewares.IsTeacherRights)

					r.Delete("/", svc.DeleteTest)
					r.Patch("/", svc.PatchTest)
					r.Post("/quizzes", svc.AddQuizzes)
					r.Delete("/quizzes", svc.RemoveQuizzes)

					r.Get("/export", svc.ExportTest)
				})
			})
		})

		r.Route("/total", func(r chi.Router) {
			r.Use(middlewares.JWT)

			r.Route("/{group_uuid}/{user_uuid}/running/{runnerKey:^[0-9]{1,20}$}", func(r chi.Router) {
				r.Use(middleware.Maybe(
					middlewares.IsInGroup(svc.GetTestUUID, svc.AllowedToTest, svc.GetDeadline), func(r *http.Request) bool {
						return !middlewares.IsTeacherCondition(r)
					}))
				r.Use(middlewares.Running(svc.GetTestUUID))

				r.Post("/{quiz_uuid}", svc.PostAnswer)
				r.Post("/", svc.Totalize)
			})

			r.Group(func(r chi.Router) {
				r.Use(middleware.Maybe(middlewares.UserRights, func(r *http.Request) bool {
					return !middlewares.IsTeacherCondition(r)
				}))
				r.Get("/all/{user_uuid}", svc.GetUserTotals)
				r.Get("/{group_uuid}/{user_uuid}/{test_uuid}/{quiz_uuid}",
					svc.GetAnswer)
			})

			r.Group(func(r chi.Router) {
				r.Use(middlewares.IsTeacherRights)

				r.Get("/", svc.ListTotals)
				r.Get("/all/all/{test_uuid}", svc.GetTestTotals)
				r.Get("/{group_uuid}/all/{test_uuid}", svc.GetGroupTotals)
			})

			r.Group(func(r chi.Router) {

				r.Use(middleware.Maybe(middlewares.UserRights, func(r *http.Request) bool {
					return !middlewares.IsTeacherCondition(r)
				}))

				r.Get("/{group_uuid}/{user_uuid}/{test_uuid}", svc.GetUserTotal)
				r.Get("/{group_uuid}/{user_uuid}/{test_uuid}/answers", svc.ListUserAnswer)
			})

		})
	})

	return r, nil
}
