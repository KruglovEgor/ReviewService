package service

import (
	"context"
	"testing"

	"reviewservice/internal/domain"
	"reviewservice/internal/testutil"

	"go.uber.org/zap"
)

// TestUserService_SetIsActive_DeactivateFallsBackToOtherTeams проверяет,
// что при деактивации пользователя если в его команде нет кандидатов,
// то сначала ищутся кандидаты в команде автора PR, а потом в других командах
func TestUserService_SetIsActive_DeactivateFallsBackToOtherTeams(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"u1":     {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
			"u2":     {UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: false}, // неактивен
			"f1":     {UserID: "f1", Username: "Charlie", TeamName: "frontend", IsActive: true},
			"f2":     {UserID: "f2", Username: "Dave", TeamName: "frontend", IsActive: true},
			"author": {UserID: "author", Username: "Author", TeamName: "frontend", IsActive: true}, // автор в frontend
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"u1"}, // ревьювер из backend
			},
		},
	}

	svc := NewUserService(userRepo, prRepo, logger)

	// Деактивируем u1 (единственного активного в backend)
	user, err := svc.SetIsActive(context.Background(), "u1", false)

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, user, "User should not be nil")

	// PR должен получить ревьювера из команды автора (frontend), а не из других команд
	pr1 := prRepo.PRs["pr1"]
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 1, "pr1 should have 1 reviewer")

	// Проверяем что новый ревьювер из frontend (f1 или f2, но не author)
	newReviewer := pr1.AssignedReviewers[0]
	isFrontend := newReviewer == "f1" || newReviewer == "f2"
	testutil.AssertEqual(t, isFrontend, true, "new reviewer should be from author's team (frontend)")
}

// TestUserService_SetIsActive_PrioritizesAuthorTeam проверяет,
// что при деактивации сначала ищем в команде ревьювера, потом в команде автора, потом в других
func TestUserService_SetIsActive_PrioritizesAuthorTeam(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			// Команда backend (ревьювера)
			"b1": {UserID: "b1", Username: "BackendUser", TeamName: "backend", IsActive: true},
			"b2": {UserID: "b2", Username: "BackendUser2", TeamName: "backend", IsActive: false}, // неактивен
			// Команда frontend (автора)
			"f1":     {UserID: "f1", Username: "FrontendUser", TeamName: "frontend", IsActive: true},
			"author": {UserID: "author", Username: "Author", TeamName: "frontend", IsActive: true},
			// Другая команда
			"d1": {UserID: "d1", Username: "DesignUser", TeamName: "design", IsActive: true},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR",
				AuthorID:          "author", // автор из frontend
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"b1"}, // ревьювер из backend
			},
		},
	}

	svc := NewUserService(userRepo, prRepo, logger)

	// Деактивируем b1
	user, err := svc.SetIsActive(context.Background(), "b1", false)

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, user, "User should not be nil")

	// PR должен получить ревьювера из команды автора (frontend), а НЕ из design
	pr1 := prRepo.PRs["pr1"]
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 1, "pr1 should have 1 reviewer")

	newReviewer := pr1.AssignedReviewers[0]
	// Должен быть f1 (единственный активный из frontend кроме автора)
	testutil.AssertEqual(t, newReviewer, "f1", "new reviewer should be f1 from author's team")
}

// TestStatsService_BulkDeactivateTeam_FallsBackToOtherTeams проверяет,
// что при массовой деактивации команды ревьюверы переназначаются сначала на команду автора, потом на другие команды
func TestStatsService_BulkDeactivateTeam_FallsBackToOtherTeams(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"b1":     {UserID: "b1", Username: "Alice", TeamName: "backend", IsActive: true},
			"b2":     {UserID: "b2", Username: "Bob", TeamName: "backend", IsActive: true},
			"f1":     {UserID: "f1", Username: "Charlie", TeamName: "frontend", IsActive: true},
			"f2":     {UserID: "f2", Username: "Dave", TeamName: "frontend", IsActive: true},
			"author": {UserID: "author", Username: "Author", TeamName: "frontend", IsActive: true}, // автор в frontend
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR 1",
				AuthorID:          "author", // автор из frontend
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"b1", "b2"}, // ревьюверы из backend
			},
			"pr2": {
				PullRequestID:     "pr2",
				PullRequestName:   "Test PR 2",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"b1"},
			},
		},
	}

	svc := NewStatsService(prRepo, userRepo, logger)

	// Деактивируем всю команду backend
	result, err := svc.BulkDeactivateTeam(context.Background(), "backend")

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, result, "Result should not be nil")
	testutil.AssertEqual(t, len(result.DeactivatedUsers), 2, "2 users should be deactivated")
	testutil.AssertEqual(t, result.ReassignedPRs, 3, "3 PR assignments should be changed")

	// Проверяем что в pr1 и pr2 теперь ревьюверы из команды автора (frontend)
	pr1 := prRepo.PRs["pr1"]
	pr2 := prRepo.PRs["pr2"]

	// pr1 должен иметь 2 ревьюверов (замена b1 и b2 на f1 и f2)
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 2, "pr1 should have 2 reviewers")
	for _, reviewer := range pr1.AssignedReviewers {
		isFrontend := reviewer == "f1" || reviewer == "f2"
		testutil.AssertEqual(t, isFrontend, true, "all reviewers should be from author's team (frontend)")
	}

	// pr2 должен иметь 1 ревьювера (замена b1)
	testutil.AssertEqual(t, len(pr2.AssignedReviewers), 1, "pr2 should have 1 reviewer")
	isFrontend := pr2.AssignedReviewers[0] == "f1" || pr2.AssignedReviewers[0] == "f2"
	testutil.AssertEqual(t, isFrontend, true, "reviewer should be from author's team (frontend)")
}

// TestStatsService_BulkDeactivateTeam_NoOtherTeams проверяет,
// что если нет других команд, ревьюверы просто удаляются
func TestStatsService_BulkDeactivateTeam_NoOtherTeams(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"b1":     {UserID: "b1", Username: "Alice", TeamName: "backend", IsActive: true},
			"b2":     {UserID: "b2", Username: "Bob", TeamName: "backend", IsActive: true},
			"author": {UserID: "author", Username: "Author", TeamName: "backend", IsActive: true},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"b1", "b2"},
			},
		},
	}

	svc := NewStatsService(prRepo, userRepo, logger)

	// Деактивируем всю команду backend (других команд нет, автор тоже в backend)
	result, err := svc.BulkDeactivateTeam(context.Background(), "backend")

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, result, "Result should not be nil")

	// PR должен остаться без ревьюверов
	pr1 := prRepo.PRs["pr1"]
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 0, "pr1 should have no reviewers")
}
