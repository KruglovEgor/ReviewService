package service_test

import (
	"context"
	"testing"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/service"
	"github.com/KruglovEgor/ReviewService/tests/testutil"
	"go.uber.org/zap"
)

// TestPullRequestService_CreatePullRequest tests PR creation scenarios
func TestPullRequestService_CreatePullRequest(t *testing.T) {
	tests := []struct {
		name       string
		prID       string
		prName     string
		authorID   string
		setupMocks func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		wantErr    error
		validatePR func(*testing.T, *domain.PullRequest)
	}{
		{
			name:     "successfully creates PR with reviewers from same team",
			prID:     "pr-001",
			prName:   "Add feature",
			authorID: "u1",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u3"] = &domain.User{
					UserID: "u3", Username: "Charlie", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: nil,
			validatePR: func(t *testing.T, pr *domain.PullRequest) {
				testutil.AssertEqual(t, pr.PullRequestID, "pr-001", "PR ID")
				testutil.AssertEqual(t, pr.PullRequestName, "Add feature", "PR name")
				testutil.AssertEqual(t, pr.AuthorID, "u1", "Author ID")
				testutil.AssertEqual(t, pr.Status, domain.PRStatusOpen, "Status should be OPEN")
				testutil.AssertTrue(t, len(pr.AssignedReviewers) <= 2, "Should assign max 2 reviewers")
				testutil.AssertNotContains(t, pr.AssignedReviewers, "u1", "Author should not be reviewer")
			},
		},
		{
			name:     "returns error when PR already exists",
			prID:     "pr-exists",
			prName:   "Duplicate",
			authorID: "u1",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-exists"] = &domain.PullRequest{PullRequestID: "pr-exists"}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: domain.ErrPRExists,
		},
		{
			name:     "returns error when author not found",
			prID:     "pr-002",
			prName:   "Test",
			authorID: "nonexistent",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// No users
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name:     "creates PR with no reviewers when team has only author",
			prID:     "pr-solo",
			prName:   "Solo work",
			authorID: "u1",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: nil,
			validatePR: func(t *testing.T, pr *domain.PullRequest) {
				testutil.AssertLen(t, pr.AssignedReviewers, 0, "Should have no reviewers when team has only author")
			},
		},
		{
			name:     "skips inactive team members when selecting reviewers",
			prID:     "pr-003",
			prName:   "Test inactive",
			authorID: "u1",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "backend", IsActive: false, // Inactive
				}
				userRepo.Users["u3"] = &domain.User{
					UserID: "u3", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: nil,
			validatePR: func(t *testing.T, pr *domain.PullRequest) {
				testutil.AssertLen(t, pr.AssignedReviewers, 1, "Should skip inactive members")
				testutil.AssertNotContains(t, pr.AssignedReviewers, "u2", "Should not include inactive user")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			prRepo := testutil.NewMockPRRepository()
			userRepo := testutil.NewMockUserRepository()
			tt.setupMocks(prRepo, userRepo)

			logger := zap.NewNop()
			svc := service.NewPullRequestService(prRepo, userRepo, logger)

			// Act
			pr, err := svc.CreatePullRequest(context.Background(), tt.prID, tt.prName, tt.authorID)

			// Assert
			if tt.wantErr != nil {
				testutil.AssertErrorIs(t, err, tt.wantErr, "Error mismatch")
				return
			}

			testutil.AssertNoError(t, err)
			testutil.AssertNotNil(t, pr, "PR should not be nil")

			if tt.validatePR != nil {
				tt.validatePR(t, pr)
			}
		})
	}
}

// TestPullRequestService_MergePullRequest tests PR merge scenarios
func TestPullRequestService_MergePullRequest(t *testing.T) {
	tests := []struct {
		name       string
		prID       string
		setupMocks func(*testutil.MockPRRepository)
		wantErr    error
		wantStatus domain.PRStatus
	}{
		{
			name: "successfully merges open PR",
			prID: "pr-001",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-001"] = &domain.PullRequest{
					PullRequestID: "pr-001",
					Status:        domain.PRStatusOpen,
				}
			},
			wantErr:    nil,
			wantStatus: domain.PRStatusMerged,
		},
		{
			name: "idempotent merge - already merged PR returns same PR",
			prID: "pr-merged",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-merged"] = &domain.PullRequest{
					PullRequestID: "pr-merged",
					Status:        domain.PRStatusMerged,
				}
			},
			wantErr:    nil,
			wantStatus: domain.PRStatusMerged,
		},
		{
			name:       "returns error when PR not found",
			prID:       "pr-nonexistent",
			setupMocks: func(prRepo *testutil.MockPRRepository) {},
			wantErr:    domain.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			prRepo := testutil.NewMockPRRepository()
			userRepo := testutil.NewMockUserRepository()
			tt.setupMocks(prRepo)

			logger := zap.NewNop()
			svc := service.NewPullRequestService(prRepo, userRepo, logger)

			// Act
			pr, err := svc.MergePullRequest(context.Background(), tt.prID)

			// Assert
			if tt.wantErr != nil {
				testutil.AssertErrorIs(t, err, tt.wantErr)
				return
			}

			testutil.AssertNoError(t, err)
			testutil.AssertNotNil(t, pr)
			testutil.AssertEqual(t, pr.Status, tt.wantStatus, "Status after merge")
		})
	}
}

// TestPullRequestService_ReassignReviewer tests reviewer reassignment
func TestPullRequestService_ReassignReviewer(t *testing.T) {
	tests := []struct {
		name       string
		prID       string
		oldUserID  string
		setupMocks func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		wantErr    error
		validate   func(*testing.T, *domain.PullRequest, string)
	}{
		{
			name:      "successfully reassigns reviewer to active team member",
			prID:      "pr-001",
			oldUserID: "u2",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-001"] = &domain.PullRequest{
					PullRequestID:     "pr-001",
					AuthorID:          "u1",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u2", "u3"},
				}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "backend", IsActive: false, // Being removed
				}
				userRepo.Users["u3"] = &domain.User{
					UserID: "u3", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u4"] = &domain.User{
					UserID: "u4", TeamName: "backend", IsActive: true, // Replacement candidate
				}
			},
			wantErr: nil,
			validate: func(t *testing.T, pr *domain.PullRequest, replacedBy string) {
				testutil.AssertNotContains(t, pr.AssignedReviewers, "u2", "Old reviewer should be removed")
				testutil.AssertContains(t, pr.AssignedReviewers, replacedBy, "New reviewer should be assigned")
				testutil.AssertNotEqual(t, replacedBy, "u1", "Author should not be assigned")
				testutil.AssertNotEqual(t, replacedBy, "u3", "Already assigned reviewer should not be selected")
			},
		},
		{
			name:      "returns error when PR not found",
			prID:      "pr-nonexistent",
			oldUserID: "u2",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// Empty
			},
			wantErr: domain.ErrNotFound,
		},
		{
			name:      "returns error when user not assigned to PR",
			prID:      "pr-001",
			oldUserID: "u99",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-001"] = &domain.PullRequest{
					PullRequestID:     "pr-001",
					AuthorID:          "u1",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u2"},
				}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u99"] = &domain.User{
					UserID: "u99", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: domain.ErrNotAssigned,
		},
		{
			name:      "returns error when PR is already merged",
			prID:      "pr-merged",
			oldUserID: "u2",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-merged"] = &domain.PullRequest{
					PullRequestID:     "pr-merged",
					AuthorID:          "u1",
					Status:            domain.PRStatusMerged,
					AssignedReviewers: []string{"u2"},
				}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
			},
			wantErr: domain.ErrPRMerged,
		},
		{
			name:      "returns error when no replacement candidates available",
			prID:      "pr-001",
			oldUserID: "u2",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-001"] = &domain.PullRequest{
					PullRequestID:     "pr-001",
					AuthorID:          "u1",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u2"},
				}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "backend", IsActive: false,
				}
				// No other active team members
			},
			wantErr: domain.ErrNoCandidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			prRepo := testutil.NewMockPRRepository()
			userRepo := testutil.NewMockUserRepository()
			tt.setupMocks(prRepo, userRepo)

			logger := zap.NewNop()
			svc := service.NewPullRequestService(prRepo, userRepo, logger)

			// Act
			pr, replacedBy, err := svc.ReassignReviewer(context.Background(), tt.prID, tt.oldUserID)

			// Assert
			if tt.wantErr != nil {
				testutil.AssertErrorIs(t, err, tt.wantErr)
				return
			}

			testutil.AssertNoError(t, err)
			testutil.AssertNotNil(t, pr)

			if tt.validate != nil {
				tt.validate(t, pr, replacedBy)
			}
		})
	}
}

// TestPullRequestService_ListPullRequests tests PR listing
func TestPullRequestService_ListPullRequests(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		setupMocks func(*testutil.MockPRRepository)
		wantCount  int
	}{
		{
			name:   "lists all PRs when no status filter",
			status: "",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID: "pr-1", Status: domain.PRStatusOpen,
				}
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID: "pr-2", Status: domain.PRStatusMerged,
				}
			},
			wantCount: 2,
		},
		{
			name:   "filters only open PRs",
			status: "OPEN",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID: "pr-1", Status: domain.PRStatusOpen,
				}
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID: "pr-2", Status: domain.PRStatusMerged,
				}
				prRepo.PRs["pr-3"] = &domain.PullRequest{
					PullRequestID: "pr-3", Status: domain.PRStatusOpen,
				}
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			prRepo := testutil.NewMockPRRepository()
			userRepo := testutil.NewMockUserRepository()
			tt.setupMocks(prRepo)

			logger := zap.NewNop()
			svc := service.NewPullRequestService(prRepo, userRepo, logger)

			// Act
			prs, err := svc.ListPullRequests(context.Background(), tt.status)

			// Assert
			testutil.AssertNoError(t, err)
			testutil.AssertLen(t, prs, tt.wantCount, "Number of PRs returned")
		})
	}
}
