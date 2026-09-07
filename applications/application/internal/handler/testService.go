package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/egot3/fathom/internal/config"
	"github.com/egot3/fathom/internal/database/repositories/group"
	"github.com/egot3/fathom/internal/database/repositories/quiz"
	"github.com/egot3/fathom/internal/database/repositories/test"
	"github.com/egot3/fathom/internal/database/repositories/total"
	"github.com/egot3/fathom/internal/database/repositories/user"
	testrunner "github.com/egot3/fathom/internal/testRunner"
	"github.com/google/uuid"
	"github.com/samber/do/v2"
)

type chiService struct {
	userRepo   user.UserRepository
	groupRepo  group.GroupRepository
	quizRepo   quiz.QuizRepository
	testRepo   test.TestRepository
	answerRepo total.TotalRepository

	cfg     *config.Config
	manager *testrunner.Manager
}

type UserService interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	PatchUser(w http.ResponseWriter, r *http.Request)
	DeleteUser(w http.ResponseWriter, r *http.Request)
	GetUser(w http.ResponseWriter, r *http.Request)
	ListUsers(w http.ResponseWriter, r *http.Request)
}

type GroupService interface {
	PostGroup(w http.ResponseWriter, r *http.Request)
	GetGroup(w http.ResponseWriter, r *http.Request)
	DeleteGroup(w http.ResponseWriter, r *http.Request)
	PatchGroup(w http.ResponseWriter, r *http.Request)
	AppendUsers(w http.ResponseWriter, r *http.Request)
	RemoveUsers(w http.ResponseWriter, r *http.Request)
	ListGroups(w http.ResponseWriter, r *http.Request)
}

type QuizService interface {
	PostQuiz(w http.ResponseWriter, r *http.Request)
	DeleteQuiz(w http.ResponseWriter, r *http.Request)
	GetQuiz(w http.ResponseWriter, r *http.Request)
	ListQuizzes(w http.ResponseWriter, r *http.Request)
	PatchQuiz(w http.ResponseWriter, r *http.Request)
	ExportQuizBank(w http.ResponseWriter, r *http.Request)
	ImportQuizBank(w http.ResponseWriter, r *http.Request)
	ParsedQuiz(w http.ResponseWriter, r *http.Request)
}

type TestService interface {
	GetTest(w http.ResponseWriter, r *http.Request)
	GetQuizFromRunning(w http.ResponseWriter, r *http.Request)
	DeleteTest(w http.ResponseWriter, r *http.Request)
	PatchTest(w http.ResponseWriter, r *http.Request)
	PostTest(w http.ResponseWriter, r *http.Request)
	StartTest(w http.ResponseWriter, r *http.Request)
	StopTest(w http.ResponseWriter, r *http.Request)
	PauseTest(w http.ResponseWriter, r *http.Request)
	AddQuizzes(w http.ResponseWriter, r *http.Request)
	RemoveQuizzes(w http.ResponseWriter, r *http.Request)
	ExtendTest(w http.ResponseWriter, r *http.Request)
	ResumeTest(w http.ResponseWriter, r *http.Request)
	ListTests(w http.ResponseWriter, r *http.Request)
	ExportTest(w http.ResponseWriter, r *http.Request)
	ImportTest(w http.ResponseWriter, r *http.Request)
	GetRunningQuizzesUUIDs(w http.ResponseWriter, r *http.Request)
	RunningInfo(w http.ResponseWriter, r *http.Request)
}

type TotalService interface {
	PostAnswer(w http.ResponseWriter, r *http.Request)
	GetAnswer(w http.ResponseWriter, r *http.Request)

	Totalize(w http.ResponseWriter, r *http.Request)

	GetUserTotal(w http.ResponseWriter, r *http.Request)
	GetUserTotals(w http.ResponseWriter, r *http.Request)
	GetGroupTotals(w http.ResponseWriter, r *http.Request)
	GetTestTotals(w http.ResponseWriter, r *http.Request)
	ListTotals(w http.ResponseWriter, r *http.Request)
	ListUserAnswer(w http.ResponseWriter, r *http.Request)
}

type Service interface {
	UserService
	GroupService
	QuizService
	TestService
	TotalService
	AllowedToTest(ctx context.Context, userUUID uuid.UUID, key uint64) (bool, error)
	GetDeadline(key uint64) (time.Time, error)
	IsRunning(quiz uuid.UUID) bool
	GetTestUUID(uint64) uuid.UUID
}

func NewTestService(i do.Injector) (Service, error) {
	uR := do.MustInvoke[user.UserRepository](i)
	gR := do.MustInvoke[group.GroupRepository](i)
	qR := do.MustInvoke[quiz.QuizRepository](i)
	tR := do.MustInvoke[test.TestRepository](i)
	aR := do.MustInvoke[total.TotalRepository](i)

	ma := do.MustInvoke[*testrunner.Manager](i)
	cf := do.MustInvoke[*config.Config](i)

	return &chiService{
		userRepo:   uR,
		quizRepo:   qR,
		groupRepo:  gR,
		testRepo:   tR,
		answerRepo: aR,
		manager:    ma,
		cfg:        cf,
	}, nil
}
