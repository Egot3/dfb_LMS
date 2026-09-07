package handler

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	acceptutils "github.com/egot3/fathom/internal/acceptUtils"
	"github.com/egot3/fathom/internal/carefulness"
	"github.com/egot3/fathom/internal/contracts"
	exportutlis "github.com/egot3/fathom/internal/exportUtlis"
	"github.com/egot3/fathom/internal/logging"
	"github.com/egot3/fathom/internal/quiz"
	quizparser "github.com/egot3/fathom/internal/quizParser"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zeebo/xxh3"
	"go.yaml.in/yaml/v4"
)

// DeleteQuiz implements [Service].
func (c *chiService) DeleteQuiz(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Set("Content-Type", "application/json")

	quizUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		logger.Error("Bad uuid")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}

	ctx = logging.WithLogger(ctx, logger.With(
		slog.String("quizUUID", quizUUID.String()),
	))

	err := c.quizRepo.DeallocateQuiz(ctx, quizUUID)
	if err != nil {
		logger.Error("unable to deallocate quiz",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Quiz not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if s := r.Header.Get("force-delete"); s == "true" {
		quizPath, err := c.quizRepo.QuizPath(ctx, quizUUID)
		if err != nil {
			logger.Error("unable to retrieve quizPath",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to get test's path internally"})
			return
		}

		err = os.Remove(quizPath)
		if err != nil {
			logger.Error("unable to delete quiz by quizPath",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to delete quiz"})
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetQuiz implements [Service].
// teacher-only path
func (c *chiService) GetQuiz(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Set("Content-Type", "application/json")

	quizUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		logger.Error("Bad uuid")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}

	ctx = logging.WithLogger(ctx, logger.With(
		slog.String("quizUUID", quizUUID.String()),
	))

	quizPath, err := c.quizRepo.QuizPath(ctx, quizUUID)
	if err != nil {
		logger.Error("unable to retrieve quizPath",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Quiz not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	f, err := os.Open(quizPath)
	if err != nil {
		logger.Error("Failed to read file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	fm, err := quizparser.ParseFrontmatter(scanner)
	if err != nil {
		logger.Error("Unable to retrieve quiz by path",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
		sb.WriteRune('\n')
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.GetQuizResponse{Meta: fm, Body: sb.String()})
}

// ListQuiz implements [Service].
func (c *chiService) ListQuizzes(w http.ResponseWriter, r *http.Request) {
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

	quizzes, total, err := c.quizRepo.ListQuizzes(ctx, pageInt, sizeInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Quizzes not found"})
			return
		}
		if gone, ok := errors.AsType[carefulness.Gone](err); ok {
			w.WriteHeader(http.StatusGone)
			json.NewEncoder(w).Encode(gone.JSONError())
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.ListQuizResponse{
		Quizzes: quizzes,
		Total:   total,
		Page:    pageInt,
		Size:    sizeInt,
	})
}

// PostQuiz implements [Service].
func (c *chiService) PostQuiz(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Set("Content-Type", "application/json")
	var req contracts.PostQuizRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("error in register during reading",
			slog.String("error", err.Error()),
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
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Empty body"})

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
	if req.Name == "" || req.Body == "" {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("request contains no quiz's body or its name",
			slog.String("name", req.Name),
			slog.Int("body len", len(req.Body)),
		)

		return
	}

	abs, err := c.cfg.TurnToAbs(req.Name)
	if err != nil {
		logger.Error("couldn't turn filepath to abs", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to get absolute path of quiz"})
		return
	}

	does, err := c.quizRepo.CheckRegistered(ctx, abs)
	if err != nil {
		logger.Error("couldn't check if quiz is registered", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't check if quiz is registered"})
		return
	}
	if does {
		logger.Warn("quiz already exists")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(carefulness.Conflict{Conflictor: "Path"})
		return
	}

	logger = logger.With(slog.String("name", req.Name))
	ctx = logging.WithLogger(ctx, logger)

	frontmatter, err := yaml.Marshal(req.Meta)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		logger.Error("unable to process frontmatter",
			slog.String("Error", err.Error()),
		)

		return
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(frontmatter)
	sb.WriteString("---\n")
	sb.WriteString(req.Body)

	quiz, err := quizparser.ParseQuiz(strings.NewReader(sb.String()))
	if err != nil {
		logger.Error("couldn't parse quiz", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
		return
	}

	checksumUint := xxh3.HashString(sb.String())
	checksum := [8]byte(binary.BigEndian.AppendUint64(nil, checksumUint))

	answer, err := json.Marshal(quiz.Answer)
	if err != nil {
		logger.Error("couldn't marshal answer to json",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		logger.Error("couldn't ensure quiz directory exists", slog.String("Error", err.Error()))

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't ensure quiz directory exists"})
		return
	}
	err = os.WriteFile(abs, []byte(sb.String()), 0644)
	if err != nil {
		logger.Error("couldn't write file",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusMultiStatus)

		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to write file"})
		return
	}

	err = c.quizRepo.RegisterQuiz(ctx, abs, checksum, quiz.Meta.Score, answer)
	if err != nil {
		logger.Error("couldn't register quiz", slog.String("Error", err.Error()))
		if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(conflict.JSONError())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't register quiz"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) PatchQuiz(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	quizUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		logger.Error("Bad uuid")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("quizUUID", quizUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	w.Header().Set("Content-Type", "application/json")
	var req contracts.PatchQuizRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Error("error in register during reading",
			slog.String("error", err.Error()),
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
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Empty body"})

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

	if req.Name != nil && *req.Name == "" {
		req.Name = nil
	}

	abs, err := c.quizRepo.QuizPath(ctx, quizUUID)
	if err != nil {
		logger.Error("couldn't select quiz path",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var checksum *[8]byte = nil
	var score *int = nil
	var ans *quiz.QuizAnswers = nil
	if req.Body != nil || req.Meta != nil {
		f, err := os.Open(abs)
		if err != nil {
			logger.Error("couldn't open file to start reading",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't open file to start reading"})
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		fm, err := quizparser.ParseFrontmatter(scanner)
		if err != nil {
			logger.Error("couldn't parse frontmatter for quiz",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't parse frontmatter for quiz"})
			return
		}

		var buf bytes.Buffer
		buf.WriteString("---\n")

		var fmtoparse quiz.Frontmatter = fm
		if req.Meta != nil {
			score = &req.Meta.Score

			if *req.Meta != fmtoparse {
				fmtoparse = *req.Meta
			}
		}
		frontmatter, err := yaml.Marshal(fmtoparse)
		if err != nil {
			logger.Error("unable to process frontmatter",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to process frontmatter"})

			return
		}
		buf.Write(frontmatter)
		buf.WriteString("---\n\n")

		logger.Info("After writing frontmatter", slog.String("Quiz", buf.String()))

		var bodyLen int
		fileStat, err := f.Stat()
		if err == nil {
			bodyLen = int(fileStat.Size()) - (buf.Len())
		}

		var bodytoparse string = ""
		if req.Body != nil {
			bodytoparse = *req.Body
		}
		if bodytoparse == "" {
			var sb strings.Builder
			sb.Grow(bodyLen)
			for scanner.Scan() {
				line := scanner.Bytes()
				sb.Write(bytes.TrimSpace(line))
				sb.WriteRune('\n')
			}
			bodytoparse = sb.String()
		}
		logger.Info("Body parsed", slog.String("Body", bodytoparse))

		buf.WriteString(bodytoparse)
		logger.Info("After writing body", slog.String("Quiz", buf.String()))

		raw := buf.Bytes()

		q, err := quizparser.ParseQuiz(&buf)
		if err != nil {
			logger.Error("couldn't parse quiz", slog.String("Error", err.Error()), slog.String("Quiz", buf.String()))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
			return
		}

		score = &q.Meta.Score
		ans = &q.Answer

		f, err = os.OpenFile(abs, os.O_WRONLY|os.O_TRUNC, 0644) // now not readonly
		if err != nil {
			logger.Error("couldn't open file to start merging",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't open file to start merging"})
			return
		}
		defer f.Close()

		c := [8]byte(binary.BigEndian.AppendUint64(nil, xxh3.Hash(buf.Bytes())))
		checksum = &c
		_, err = f.Write(raw)
		if err != nil {
			logger.Error("unable to write file",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to write file"})
			return
		}

		err = f.Sync()
		if err != nil {
			logger.Error("couldn't flush buffer to file",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't to flush buffer to file"})
			return
		}
	}

	var newAbs *string = nil
	if req.Name != nil {
		if v, err := c.cfg.TurnToAbs(*req.Name); err == nil {
			newAbs = &v
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Error("couldn't get new abs name", slog.String("Error", err.Error()))
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to turn give path to abs"})
			return
		}
	}

	if newAbs != nil {
		if err := os.MkdirAll(filepath.Dir(*newAbs), 0o755); err != nil {
			logger.Error("couldn't ensure quiz directory exists", slog.String("Error", err.Error()))

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
			return
		}
		err := os.Rename(abs, *newAbs)
		if err != nil {
			logger.Error("couldn't get rename to abs name", slog.String("Error", err.Error()))

			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: err.Error()})
			return
		}
	}

	err = c.quizRepo.PatchQuiz(ctx, quizUUID, newAbs, score, ans, checksum)
	if err != nil {
		logger.Error("couldn't update quiz", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't register quiz"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) ExportQuizBank(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	var req contracts.ExportQuizRequest
	err := json.NewDecoder(r.Body).Decode(&req)
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

	accept, err := acceptutils.BestAccept(r.Header.Get("Accept"),
		"application/zip", "application/tar", "application/gzip",
	)
	if (err != nil) || (accept == "") {
		logger.Info("Got an unaccaptable accept header",
			slog.String("accept", accept),
			slog.String("Error", err.Error()),
		)

		w.WriteHeader(http.StatusNotAcceptable)
		return
	}

	type quizFile struct {
		uuid string
		path string
		fi   os.FileInfo // used by tar
	}
	var files []quizFile
	for _, uuid := range req.UUIDs {
		path, err := c.quizRepo.QuizPath(ctx, uuid)
		if err != nil {
			logger.Error("couldn't get path", "uuid", uuid, "error", err)
			if errors.Is(err, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: fmt.Sprintf("%v not found", uuid)})
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: fmt.Sprintf("unable to process %v", uuid)})
			}
			return
		}
		fi, err := os.Stat(path)
		if err != nil {
			logger.Error("quiz file not accessible", "path", path, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		files = append(files, quizFile{uuid: uuid.String(), path: path, fi: fi})
	}

	switch accept {
	case "application/zip":
		w.Header().Set("Content-Type", "application/zip")
		zipWriter := zip.NewWriter(w)
		logger = logger.With(slog.String("strategy", "zip"))

		for _, qf := range files {
			if err := exportutlis.AddFileToZip(zipWriter, qf.path); err != nil {
				logger.Error("error writing to zip",
					slog.String("path", qf.path),
					slog.String("Error", err.Error()),
				)
				zipWriter.Close()
				return
			}
		}
		if err := zipWriter.Close(); err != nil {
			logger.Error("error finalising zip", "error", err)
		}
		return

	case "application/tar":
		logger = logger.With(slog.String("strategy", "tar"))
		w.Header().Set("Content-Type", "application/tar")
		tarWriter := tar.NewWriter(w)

		for _, qf := range files {
			if err := exportutlis.AddFileToTar(tarWriter, qf.path, qf.fi); err != nil {
				logger.Error("error writing to tar",
					slog.String("path", qf.path),
					slog.String("Error", err.Error()),
				)

				tarWriter.Close()
				return
			}
		}
		if err := tarWriter.Close(); err != nil {
			logger.Error("error finalising tar", "error", err)
		}
		return

	case "application/gzip":
		logger = logger.With(slog.String("strategy", "tar.gz"))
		w.Header().Set("Content-Type", "application/gzip")

		gzipWriter := gzip.NewWriter(w)
		tarWriter := tar.NewWriter(gzipWriter)

		for _, qf := range files {
			if err := exportutlis.AddFileToTar(tarWriter, qf.path, qf.fi); err != nil {
				logger.Error("error writing to tar",
					slog.String("path", qf.path),
					slog.String("Error", err.Error()),
				)

				tarWriter.Close()
				return
			}
		}

		if err := tarWriter.Close(); err != nil {
			logger.Error("error finalising tar", "error", err)
		}
		if err := gzipWriter.Close(); err != nil {
			logger.Error("error finalising gz", "error", err)
		}
		return

	default:
		w.WriteHeader(http.StatusNotAcceptable)
		return

	}
}

func (c *chiService) ImportQuizBank(w http.ResponseWriter, r *http.Request) {
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

	archiveParts, handler, err := r.FormFile("imported")
	if err != nil {
		logger.Error("couldn't get file", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to parse file"})
		return
	}
	defer archiveParts.Close()

	contentType, _, err := mime.ParseMediaType(handler.Header.Get("Content-Type"))
	if err != nil {
		logger.Error("couldn't parse MIME type", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to parse MIME"})
		return
	}

	logger = logger.With(
		slog.String("file_name", handler.Filename),
		slog.String("mime", contentType),
		slog.Int64("size", handler.Size),
	)
	ctx = logging.WithLogger(ctx, logger)

	tmpDir, err := os.MkdirTemp("", "tmp-")
	if err != nil {
		logger.Error("failed to create tmpDir",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create temp dir for new quiz bank"})
		return
	}
	defer os.RemoveAll(tmpDir)

	switch contentType {
	case "application/zip":
		zipReader, err := zip.NewReader(archiveParts, handler.Size)
		if err != nil {
			logger.Error("couldn't create new zip-reader", slog.String("Error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to create zip reader"})
			return
		}

		for _, f := range zipReader.File {
			cleanPath := filepath.Clean(f.FileInfo().Name())
			destPath := filepath.Join(tmpDir, cleanPath)
			absPath, err := filepath.Abs(destPath)
			if err != nil {
				logger.Error("zip-slip detected", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it was suspected to be unsafe"})
				return
			}
			if !strings.HasPrefix(absPath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				logger.Error("real zip-slip")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it IS unsafe(https://developer.android.com/privacy-and-security/risks/zip-path-traversal)"})
				return
			}

			if f.FileInfo().IsDir() {
				if err := os.Mkdir(absPath, 0o750); err != nil {
					logger.Error("couldn't create dir", slog.String("Error", err.Error()))
					if errors.Is(err, os.ErrExist) {
						continue
					}
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to create dir from zip"})
					return
				}
				continue
			}

			rc, err := f.Open()
			if err != nil {
				logger.Error("couldn't create reader for file from zip reader", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't read file from zip"})
				return
			}
			defer rc.Close()

			var buf bytes.Buffer

			tee := io.TeeReader(rc, &buf)

			q, err := quizparser.ParseQuiz(tee)
			if err != nil {
				logger.Error("invalid quiz", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't parse quiz" + err.Error()})
				return
			}

			dest, err := os.Create(absPath)
			if err != nil {
				logger.Error("couldn't create file for zip file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create file"})
				return
			}
			_, err = io.Copy(dest, &buf)
			if err != nil {
				logger.Error("couldn't write zip entry to file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't write zip entry to file"})
				return
			}
			rc.Close()
			dest.Close()

			checksumUint := xxh3.Hash(buf.Bytes())
			checksum := [8]byte(binary.BigEndian.AppendUint64(nil, checksumUint))

			answer, err := json.Marshal(q.Answer)
			if err != nil {
				logger.Error("couldn't marshal answer to json",
					slog.String("Error", err.Error()),
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = c.quizRepo.RegisterQuiz(ctx, absPath, checksum, q.Meta.Score, answer)
			if err != nil {
				logger.Error("couldn't register quiz", slog.String("Error", err.Error()))
				if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(conflict.JSONError())
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't register quiz"})
				return
			}
		}

		err = os.Rename(tmpDir, filepath.Join("", handler.Filename[:len(handler.Filename)-4]))
		w.WriteHeader(http.StatusNoContent)
		return
	case "application/tar":
		tarReader := tar.NewReader(archiveParts)

		for {
			f, err := tarReader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				logger.Error("unable to get next tar entry",
					slog.String("Error", err.Error()),
				)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't read tar archive"})
				return
			}

			cleanPath := filepath.Clean(f.FileInfo().Name())
			destPath := filepath.Join(tmpDir, cleanPath)
			absPath, err := filepath.Abs(destPath)
			if err != nil {
				logger.Error("zip-slip detected", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it was suspected to be unsafe"})
				return
			}
			if !strings.HasPrefix(absPath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				logger.Error("real tar-slip")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it IS unsafe(https://developer.android.com/privacy-and-security/risks/zip-path-traversal)"})
				return
			}

			if f.FileInfo().IsDir() {
				if err := os.Mkdir(absPath, 0o750); err != nil {
					logger.Error("couldn't create dir", slog.String("Error", err.Error()))
					if errors.Is(err, os.ErrExist) {
						continue
					}
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to create dir from zip"})
					return
				}
				continue
			}

			var buf bytes.Buffer
			tee := io.TeeReader(tarReader, &buf)
			q, err := quizparser.ParseQuiz(tee)
			if err != nil {
				logger.Error("invalid quiz", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't parse quiz" + err.Error()})
				return
			}

			dest, err := os.Create(absPath)
			if err != nil {
				logger.Error("couldn't create file for tar file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create file"})
				return
			}

			_, err = io.Copy(dest, &buf)
			if err != nil {
				logger.Error("couldn't write tar entry to file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't write zip entry to file"})
				return
			}
			dest.Close()

			checksumUint := xxh3.Hash(buf.Bytes())
			checksum := [8]byte(binary.BigEndian.AppendUint64(nil, checksumUint))

			answer, err := json.Marshal(q.Answer)
			if err != nil {
				logger.Error("couldn't marshal answer to json",
					slog.String("Error", err.Error()),
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = c.quizRepo.RegisterQuiz(ctx, absPath, checksum, q.Meta.Score, answer)
			if err != nil {
				logger.Error("couldn't register quiz", slog.String("Error", err.Error()))
				if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(conflict.JSONError())
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't register quiz"})
				return
			}
		}

		err = os.Rename(tmpDir, filepath.Join("", handler.Filename[:len(handler.Filename)-4]))
		w.WriteHeader(http.StatusNoContent)
		return
	case "application/gzip":
		gzipReader, err := gzip.NewReader(archiveParts)
		if err != nil {
			logger.Error("couldn't create gzip reader",
				slog.String("Error", err.Error()),
			)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create gzip reader"})

			return
		}
		tarReader := tar.NewReader(gzipReader)

		for {
			f, err := tarReader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				logger.Error("unable to get next tar entry",
					slog.String("Error", err.Error()),
				)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't read tar archive"})
				return
			}

			cleanPath := filepath.Clean(f.FileInfo().Name())
			destPath := filepath.Join(tmpDir, cleanPath)
			absPath, err := filepath.Abs(destPath)
			if err != nil {
				logger.Error("gzip-slip detected", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it was suspected to be unsafe"})
				return
			}
			if !strings.HasPrefix(absPath, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
				logger.Error("real gzip-slip")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "can't use this zip as it IS unsafe(https://developer.android.com/privacy-and-security/risks/zip-path-traversal)"})
				return
			}

			if f.FileInfo().IsDir() {
				if err := os.Mkdir(absPath, 0o750); err != nil {
					logger.Error("couldn't create dir", slog.String("Error", err.Error()))
					if errors.Is(err, os.ErrExist) {
						continue
					}
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(carefulness.JSONError{Error: "unable to create dir from zip"})
					return
				}
				continue
			}

			var buf bytes.Buffer
			tee := io.TeeReader(tarReader, &buf)

			q, err := quizparser.ParseQuiz(tee)
			if err != nil {
				logger.Error("invalid quiz", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't parse quiz" + err.Error()})
				return
			}

			dest, err := os.Create(absPath)
			if err != nil {
				logger.Error("couldn't create file for tar file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't create file"})
				return
			}
			_, err = io.Copy(dest, &buf)
			if err != nil {
				logger.Error("couldn't write tar entry to file", slog.String("Error", err.Error()))
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't write zip entry to file"})
				return
			}
			dest.Close()

			checksumUint := xxh3.Hash(buf.Bytes())
			checksum := [8]byte(binary.BigEndian.AppendUint64(nil, checksumUint))

			answer, err := json.Marshal(q.Answer)
			if err != nil {
				logger.Error("couldn't marshal answer to json",
					slog.String("Error", err.Error()),
				)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = c.quizRepo.RegisterQuiz(ctx, absPath, checksum, q.Meta.Score, answer)
			if err != nil {
				logger.Error("couldn't register quiz", slog.String("Error", err.Error()))
				if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(conflict.JSONError())
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't register quiz"})
				return
			}

		}

		err = os.Rename(tmpDir, filepath.Join("", handler.Filename[:len(handler.Filename)-4]))
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}
}

func (c *chiService) ParsedQuiz(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)

	w.Header().Set("Content-Type", "application/json")

	quizUUID, err := uuid.Parse(chi.URLParam(r, "quiz_uuid"))
	if err != nil {
		logger.Error("Bad uuid", slog.String("Error", err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Bad UUID"})
		return
	}

	ctx = logging.WithLogger(ctx, logger.With(
		slog.String("quizUUID", quizUUID.String()),
	))

	quiz, err := c.quizRepo.Quiz(ctx, quizUUID)
	if err != nil {
		logger.Error("unable to retrieve quiz",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Quiz not found"})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	checkString := fmt.Sprintf("%q", hex.EncodeToString(quiz.Checksum[:]))

	if match := r.Header.Get("If-None-Match"); match == checkString {
		w.WriteHeader(http.StatusNotModified) // caching goes brrrrr
		return
	}

	w.Header().Set("ETag", checkString)
	w.Header().Set("Cache-Control", "public, immutable, must-revalidate")

	f, err := os.Open(quiz.Path)
	if err != nil {
		logger.Error("Failed to read file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	q, err := quizparser.ParseQuiz(f)
	if err != nil {
		logger.Error("Unable to retrieve quiz by path",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(contracts.ParsedQuizResponse{Quiz: *q}); err != nil {
		logger.Error("Couldn't encode body", slog.String("Error", err.Error()))
	}
}

func (c *chiService) IsRunning(quiz uuid.UUID) bool {
	return c.manager.IsQuizRunning(quiz)
}
