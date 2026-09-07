package testutils

import (
	"embed"
	"errors"
	"io"
	"io/fs"

	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed placebo.md
var placebo embed.FS

func TestQuiz(t *testing.T) *os.File {
	t.Helper()

	f, err := os.CreateTemp("", "*.md")
	require.NoError(t, err)

	pb, err := placebo.ReadFile("placebo.md")
	require.NoError(t, err)

	_, err = f.Write(pb)
	require.NoError(t, err)

	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return f
}

//go:embed quizzes
var quizzes embed.FS

var foundError = errors.New("found error")

func TestRadioQuiz(t *testing.T) *os.File {
	t.Helper()

	f, err := os.CreateTemp("", "*.md")
	require.NoError(t, err)

	err = fs.WalkDir(quizzes, ".", func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if d.IsDir() || d.Name() != "radio.md" {
			return nil
		}

		quiz, err := quizzes.Open(path)
		require.NoError(t, err)
		defer quiz.Close()

		_, err = io.Copy(f, quiz)
		require.NoError(t, err)

		require.NoError(t, f.Sync())

		return foundError
	})
	if !errors.Is(err, foundError) {
		require.NoError(t, err)
	}

	return f
}
