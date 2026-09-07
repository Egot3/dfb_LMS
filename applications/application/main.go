package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/egot3/fathom/internal/config"
	"github.com/egot3/fathom/internal/database"
	"github.com/egot3/fathom/internal/database/repositories"
	"github.com/egot3/fathom/internal/handler"
	"github.com/egot3/fathom/internal/logging"
	"github.com/egot3/fathom/internal/models"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/egot3/fathom/server"
	"github.com/go-chi/chi/v5"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	i := do.New(
		do.Eager(config.Load),
		do.Lazy(logging.NewLogger),
		database.DBPackage,
		repositories.RepositoryPackage,
	)

	cfg := do.MustInvoke[*config.Config](i)

	db := do.MustInvoke[*bun.DB](i)
	if err := database.RunMigrations(context.Background(), db); err != nil {
		log.Fatalf("Fatal migration error: %v", err)
	}
	db.RegisterModel((*models.GroupsUsers)(nil))
	db.RegisterModel((*models.UserGroupsTests)(nil))
	db.RegisterModel((*models.GroupsUsers)(nil))
	db.RegisterModel((*models.TestsQuizzes)(nil))
	if cfg.InitAdminPassword != "" && cfg.InitAdminUsername != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.InitAdminPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Couldn't create init teacher: %v", err.Error())
		}
		_, err = db.NewInsert().On("CONFLICT DO UPDATE").Model(&models.User{
			Nickname:     cfg.InitAdminUsername,
			PasswordHash: passwordHash,
			IsTeacher:    true,
		}).Exec(context.Background())
		if err != nil {
			log.Printf("Couldn't create init teacher: %v", err.Error())
		}
	}

	do.Provide(i, testrunner.NewManager)

	do.Provide(i, handler.NewTestService)
	do.Provide(i, server.ChiServer)

	log.Printf("running on %v", cfg.ServerPort)
	if err := http.ListenAndServe(":"+cfg.ServerPort, do.MustInvoke[chi.Router](i)); err != nil {
		log.Printf("Server execution finished: %v", err)
		os.Exit(0)
	}
}
