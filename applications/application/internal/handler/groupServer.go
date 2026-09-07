package handler

import (
	"context"
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
	"github.com/google/uuid"
)

// AppendUsers implements [Service].
func (c *chiService) AppendUsers(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		logger.Error("Bad uuid")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("group_uuid", groupUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.AppendUsersRequest
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

	if len(req.Appendants) == 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Can't proccess adding 0 users"})
		return
	}

	err = c.groupRepo.AppendUsers(ctx, groupUUID, req.Appendants)
	if err != nil {
		logger.Error("Failed to append new users to group",
			slog.String("Error", err.Error()),
		)
		if partial, ok := errors.AsType[carefulness.PartialSuccess](err); ok {
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(partial.JSONError())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "couldn't insert new users to group"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteGroup implements [Service].
func (c *chiService) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("group_uuid", groupUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	err := c.groupRepo.DeleteGroup(ctx, groupUUID)
	if err != nil {
		logger.Error("Failed to get group",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested group not found"})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetGroup implements [Service].
func (c *chiService) GetGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("group_uuid", groupUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	group, err := c.groupRepo.Group(ctx, groupUUID)
	if err != nil {
		logger.Error("Failed to get group",
			slog.String("Error", err.Error()),
		)
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested group not found"})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.GetGroupResponse{
		Group: group,
	})
}

// PatchGroup implements [Service].
func (c *chiService) PatchGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("group_uuid", groupUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.PatchGroupRequest
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
			w.WriteHeader(http.StatusNoContent)

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

	if req.Name == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if len(*req.Name) > 256 {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "group name is too long, must be <256"})
		return
	}
	if len(*req.Name) < 4 {
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "group name is too short, must be <4"})
		return
	}

	err = c.groupRepo.UpdateGroup(ctx, groupUUID, *req.Name)
	if err != nil {
		logger.Error("Failed to patch group",
			slog.String("Error", err.Error()),
		)
		if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(conflict.JSONError())
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Requested group wasn't found"})
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PostGroup implements [Service].
func (c *chiService) PostGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	var req contracts.PostGroupRequest
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
	logger = logger.With(slog.String("group_name", req.Name))
	ctx = logging.WithLogger(ctx, logger)

	if len(req.Name) < 3 {
		logger.Info("Attempt to create group with invalid nickname",
			slog.String("group_name", req.Name),
		)
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "group name is too short"})
		return
	}
	if len(req.Name) > 255 {
		logger.Info("Attempt to create group with invalid nickname",
			slog.String("group_name", req.Name),
		)
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "group name is too big"})
		return
	}

	group, err := c.groupRepo.NewGroup(ctx, req.Name)
	if err != nil {
		logger.Error("Failed to create new group",
			slog.String("Error", err.Error()),
		)
		if conflict, ok := errors.AsType[carefulness.Conflict](err); ok {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(conflict.JSONError())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(req.Appendants) != 0 {
		err = c.groupRepo.AppendUsers(ctx, group.UUID, req.Appendants)
		if err != nil {
			logger.Error("Failed to append users to new group",
				slog.String("Error", err.Error()),
			)
			json.NewEncoder(w).Encode(`{"status": [201, 500]}`)
			w.WriteHeader(http.StatusMultiStatus)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

// RemoveUsers implements [Service].
func (c *chiService) RemoveUsers(w http.ResponseWriter, r *http.Request) {
	logger := logging.LoggerFromContext(r.Context()).With(
		slog.String("layer", "handler"),
	)
	ctx := logging.WithLogger(r.Context(), logger)
	w.Header().Set("Content-Type", "application/json")

	groupUUID, ok := (r.Context().Value("uuid")).(uuid.UUID)
	if !ok {
		logger.Error("Bad uuid")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Unable to retrieve uuid"})
		return
	}
	logger = logger.With(slog.String("group_uuid", groupUUID.String()))
	ctx = logging.WithLogger(ctx, logger)

	var req contracts.RemoveUsersRequest
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
	ctx = logging.WithLogger(ctx, logger.With(slog.Int("removants_len", len(req.Removants))))

	if len(req.Removants) == 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(carefulness.JSONError{Error: "Can't proccess adding 0 users"})
		return
	}

	err = c.groupRepo.RemoveUsers(ctx, groupUUID, req.Removants)
	if err != nil {
		logger.Error("Failed to delete users from group",
			slog.String("Error", err.Error()),
		)
		if partial, ok := errors.AsType[carefulness.PartialSuccess](err); ok {
			w.WriteHeader(http.StatusMultiStatus)
			json.NewEncoder(w).Encode(partial.JSONError())
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *chiService) ListGroups(w http.ResponseWriter, r *http.Request) {
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
	ctx = logging.WithLogger(ctx, logger)

	groups, total, err := c.groupRepo.ListGroups(ctx, pageInt, sizeInt)
	if err != nil {
		logger.Error("Failed to get group",
			slog.String("Error", err.Error()),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(contracts.ListGroupsResponse{
		Groups: groups,
		Total:  total,
		Page:   pageInt,
		Size:   sizeInt,
	})
}

func (c *chiService) AllowedToTest(ctx context.Context, userUUID uuid.UUID, key uint64) (bool, error) {
	info, ok := c.manager.Get(key)
	if !ok {
		return false, fmt.Errorf("key not found in runner")
	}
	return c.groupRepo.IsInAny(ctx, info.Groups(), userUUID)
}
