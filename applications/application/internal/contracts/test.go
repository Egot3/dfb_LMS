package contracts

import (
	"time"

	"github.com/egot3/fathom/internal/models"
	"github.com/egot3/fathom/internal/quiz"
	"github.com/google/uuid"
)

type GetTestResponse struct {
	Test models.Test `json:"test"`
}

type PostTestRequest struct {
	Name    string     `json:"name"`
	Quizzes uuid.UUIDs `json:"quizzes"`
}

type AddQuizzesToTestRequest struct {
	QuizUUIDs uuid.UUIDs `json:"quiz_uuids"`
}

type PatchTestRequest struct {
	Name *string `json:"name"`
}

type ExtendTestRequest struct {
	ExtendBy string `json:"extend_by"`
}

type RemoveQuizzesRequest struct {
	QuizUUIDs uuid.UUIDs `json:"quiz_uuids"`
}

type StartRequest struct {
	Duration    string     `json:"duration"`
	TestUUID    uuid.UUID  `json:"test_uuid"`
	GroupsUUIDs uuid.UUIDs `json:"group_uuids"`
}

type GetQuizFromRunningResponse struct {
	Quiz quiz.Quiz `json:"quiz"`
}

type ListTestsResponse struct {
	Page  int           `json:"page"`
	Size  int           `json:"size"`
	Total int           `json:"total"`
	Tests []models.Test `json:"tests"`
}

type ExportTestRequest struct {
	Description string `json:"description"`
}

type GetQuizzesUUIDs struct {
	UUIDs uuid.UUIDs `json:"quiz_uuids"`
}

type RunningInfo struct {
	models.Test
	Key      uint64    `json:"key"`
	Deadline time.Time `json:"deadline"`
	IsPaused bool      `json:"is_paused"`
}

type RunningInfoResponse struct {
	TestInfos []RunningInfo `json:"tests"`
}
