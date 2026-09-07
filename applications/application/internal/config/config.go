package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samber/do/v2"
)

type Config struct {
	DatabaseDriver string
	SqlitePath     string
	PostgresURL    string

	ServerPort string
	LogLevel   string
	LogSinks   []string

	InitAdminUsername string
	InitAdminPassword string

	QuizPath string
}

func getEnv(key, fallback string, allowed []string) string {
	if v, ok := os.LookupEnv(key); ok && (allowed == nil || slices.Contains(allowed, v)) {
		return v
	}
	return fallback
}

func Load(i do.Injector) (*Config, error) {
	datadir := getEnv("DATA_DIRECTORY", func() string {
		_, err := os.Stat("/.dockerenv")
		if err == nil {
			return "/data"
		}
		return "./data"
	}(), nil)

	return &Config{
		DatabaseDriver: getEnv("DATABASE_DRIVER", "sqlite", []string{"sqlite", "postgres"}),
		SqlitePath:     datadir + "/fathom.db",
		PostgresURL:    os.Getenv("POSTGRES_URL"),

		ServerPort: getEnv("SERVER_PORT", "8080", nil),
		LogLevel:   getEnv("LOG_LEVEL", "info", nil),
		LogSinks:   strings.Split(os.Getenv("LOG_SINKS"), ","),

		InitAdminUsername: os.Getenv("INIT_ADMIN_USERNAME"),
		InitAdminPassword: os.Getenv("INIT_ADMIN_PASSWORD"),

		QuizPath: datadir + "/quizzes",
	}, nil
}

func (c Config) TurnToAbs(name string) (string, error) {
	return filepath.Abs(filepath.Join(c.QuizPath, name+".md"))
}
