package testutils

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/egot3/fathom/internal/config"
	"github.com/egot3/fathom/internal/models"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func NewTestInjector(tb testing.TB, packages ...func(do.Injector)) do.Injector {
	tb.Helper()

	cfg := &config.Config{QuizPath: filepath.Join(tb.TempDir(), "quizzes")}

	dsn := "file::memory:?cache=private"

	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	require.NoError(tb, err)

	sqldb.SetMaxOpenConns(1)

	db := bun.NewDB(sqldb, sqlitedialect.New())

	err = RunMigrations(tb.Context(), db)
	require.NoError(tb, err)

	RegisterModels(db)

	tb.Cleanup(func() {
		err := db.Close()
		if err != nil {
			tb.Logf("failed to close test db(memory leaks go brrr): %v", err)
		}
	})

	i := do.New(
		packages...,
	)

	do.ProvideValue(i, cfg)

	do.Provide(i, func(i do.Injector) (*bun.DB, error) {
		return db, nil
	})

	return i
}

func RegisterModels(db *bun.DB) {
	db.RegisterModel((*models.GroupsUsers)(nil))
	db.RegisterModel((*models.TestsQuizzes)(nil))
	db.RegisterModel((*models.GroupsUsers)(nil))
}
