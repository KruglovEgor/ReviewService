package service

import (
	"context"
	"testing"

	"reviewservice/internal/domain"
	"reviewservice/internal/testutil"
	"go.uber.org/zap"
)

// TestUserService_SetIsActive_DeactivateReassignsReviewers проверяет,
// что при деактивации пользователя его открытые PR переназначаются
func TestUserService_SetIsActive_DeactivateReassignsReviewers(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"u1":     {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
			"u2":     {UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
			"u3":     {UserID: "u3", Username: "Charlie", TeamName: "backend", IsActive: true},
			"author": {UserID: "author", Username: "Author", TeamName: "frontend", IsActive: true},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR 1",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"u1", "u2"},
			},
			"pr2": {
				PullRequestID:     "pr2",
				PullRequestName:   "Test PR 2",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"u1"},
			},
		},
	}

	svc := NewUserService(userRepo, prRepo, logger)

	// Деактивируем пользователя u1
	user, err := svc.SetIsActive(context.Background(), "u1", false)

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, user, "User should not be nil")
	testutil.AssertEqual(t, user.IsActive, false, "User should be deactivated")

	// Проверяем что u1 больше не ревьювер в pr1
	pr1 := prRepo.PRs["pr1"]
	hasU1 := false
	for _, r := range pr1.AssignedReviewers {
		if r == "u1" {
			hasU1 = true
			break
		}
	}
	testutil.AssertEqual(t, hasU1, false, "u1 should not be reviewer in pr1")

	// Проверяем что в pr1 есть замена (u2 или u3)
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 2, "pr1 should have 2 reviewers")

	// Проверяем что u1 больше не ревьювер в pr2
	pr2 := prRepo.PRs["pr2"]
	hasU1InPr2 := false
	for _, r := range pr2.AssignedReviewers {
		if r == "u1" {
			hasU1InPr2 = true
			break
		}
	}
	testutil.AssertEqual(t, hasU1InPr2, false, "u1 should not be reviewer in pr2")

	// В pr2 должна быть замена (u2 или u3)
	testutil.AssertEqual(t, len(pr2.AssignedReviewers), 1, "pr2 should have 1 reviewer")
}

// TestUserService_SetIsActive_DeactivateWithNoCandidates проверяет,
// что если нет кандидатов для замены, ревьювер просто удаляется
func TestUserService_SetIsActive_DeactivateWithNoCandidates(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"u1":     {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
			"u2":     {UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: false}, // неактивен
			"author": {UserID: "author", Username: "Author", TeamName: "frontend", IsActive: true},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: map[string]*domain.PullRequest{
			"pr1": {
				PullRequestID:     "pr1",
				PullRequestName:   "Test PR",
				AuthorID:          "author",
				Status:            domain.PRStatusOpen,
				AssignedReviewers: []string{"u1"},
			},
		},
	}

	svc := NewUserService(userRepo, prRepo, logger)

	// Деактивируем u1 (единственного активного в команде)
	user, err := svc.SetIsActive(context.Background(), "u1", false)

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, user, "User should not be nil")

	// PR должен остаться без ревьюверов
	pr1 := prRepo.PRs["pr1"]
	testutil.AssertEqual(t, len(pr1.AssignedReviewers), 0, "pr1 should have no reviewers")
}

// TestUserService_SetIsActive_ActivateDoesNotReassign проверяет,
// что при активации пользователя ничего не переназначается
func TestUserService_SetIsActive_ActivateDoesNotReassign(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"u1": {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: false},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: make(map[string]*domain.PullRequest),
	}

	svc := NewUserService(userRepo, prRepo, logger)

	// Активируем пользователя
	user, err := svc.SetIsActive(context.Background(), "u1", true)

	testutil.AssertNil(t, err, "No error expected")
	testutil.AssertNotNil(t, user, "User should not be nil")
	testutil.AssertEqual(t, user.IsActive, true, "User should be activated")

	// Проверяем что PRs не трогали
	testutil.AssertEqual(t, len(prRepo.PRs), 0, "No PRs should be affected")
}
