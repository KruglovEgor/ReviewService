package service_test

import (
	"context"
	"testing"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/service"
	"github.com/KruglovEgor/ReviewService/tests/testutil"
	"go.uber.org/zap"
)

// TestStatsService_GetStats tests statistics retrieval
func TestStatsService_GetStats(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		validate   func(*testing.T, *service.GlobalStats)
	}{
		{
			name: "calculates correct PR statistics",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID:     "pr-1",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u2", "u3"},
				}
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID:     "pr-2",
					Status:            domain.PRStatusMerged,
					AssignedReviewers: []string{"u2"},
				}
				prRepo.PRs["pr-3"] = &domain.PullRequest{
					PullRequestID:     "pr-3",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{},
				}

				userRepo.Users["u2"] = &domain.User{UserID: "u2", Username: "Bob"}
				userRepo.Users["u3"] = &domain.User{UserID: "u3", Username: "Charlie"}
			},
			validate: func(t *testing.T, stats *service.GlobalStats) {
				testutil.AssertEqual(t, stats.PRStats.TotalPRs, 3, "Total PRs")
				testutil.AssertEqual(t, stats.PRStats.OpenPRs, 2, "Open PRs")
				testutil.AssertEqual(t, stats.PRStats.MergedPRs, 1, "Merged PRs")

				// Average: (2 + 1 + 0) / 3 = 1.00
				testutil.AssertEqual(t, stats.PRStats.AvgReviewersPerPR, 1.0, "Average reviewers per PR")

				testutil.AssertNotNil(t, stats.UserStats, "User stats should exist")
				testutil.AssertEqual(t, len(stats.UserStats), 2, "Should have stats for 2 users")

				// User u2: 2 assignments (1 open, 1 merged)
				u2Stats, exists := stats.UserStats["u2"]
				testutil.AssertTrue(t, exists, "u2 stats should exist")
				if exists {
					testutil.AssertEqual(t, u2Stats.TotalAssignments, 2, "u2 total assignments")
					testutil.AssertEqual(t, u2Stats.OpenPRs, 1, "u2 open PRs")
					testutil.AssertEqual(t, u2Stats.MergedPRs, 1, "u2 merged PRs")
				}

				// User u3: 1 assignment (1 open, 0 merged)
				u3Stats, exists := stats.UserStats["u3"]
				testutil.AssertTrue(t, exists, "u3 stats should exist")
				if exists {
					testutil.AssertEqual(t, u3Stats.TotalAssignments, 1, "u3 total assignments")
					testutil.AssertEqual(t, u3Stats.OpenPRs, 1, "u3 open PRs")
					testutil.AssertEqual(t, u3Stats.MergedPRs, 0, "u3 merged PRs")
				}
			},
		},
		{
			name: "handles empty repository",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// No PRs
			},
			validate: func(t *testing.T, stats *service.GlobalStats) {
				testutil.AssertEqual(t, stats.PRStats.TotalPRs, 0, "Total PRs")
				testutil.AssertEqual(t, stats.PRStats.OpenPRs, 0, "Open PRs")
				testutil.AssertEqual(t, stats.PRStats.MergedPRs, 0, "Merged PRs")
				testutil.AssertEqual(t, stats.PRStats.AvgReviewersPerPR, 0.0, "Avg reviewers")
				testutil.AssertEqual(t, len(stats.UserStats), 0, "User stats should be empty")
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
			svc := service.NewStatsService(prRepo, userRepo, logger)

			// Act
			stats, err := svc.GetStats(context.Background())

			// Assert
			testutil.AssertNoError(t, err)
			testutil.AssertNotNil(t, stats)

			if tt.validate != nil {
				tt.validate(t, stats)
			}
		})
	}
}

// TestStatsService_BulkDeactivateTeam tests bulk team deactivation
func TestStatsService_BulkDeactivateTeam(t *testing.T) {
	tests := []struct {
		name       string
		teamName   string
		setupMocks func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		validate   func(*testing.T, *service.BulkDeactivateResult)
		wantErr    error
	}{
		{
			name:     "successfully deactivates team and reassigns open PRs",
			teamName: "backend",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// Backend team members to deactivate
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true,
				}

				// Frontend team members (replacement candidates)
				userRepo.Users["u3"] = &domain.User{
					UserID: "u3", Username: "Charlie", TeamName: "frontend", IsActive: true,
				}
				userRepo.Users["u4"] = &domain.User{
					UserID: "u4", Username: "Dave", TeamName: "frontend", IsActive: true,
				}

				// Open PRs with backend reviewers
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID:     "pr-1",
					AuthorID:          "u3", // Frontend author
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u1", "u2"},
				}

				// Merged PR should not be reassigned
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID:     "pr-2",
					AuthorID:          "u3",
					Status:            domain.PRStatusMerged,
					AssignedReviewers: []string{"u1"},
				}
			},
			validate: func(t *testing.T, result *service.BulkDeactivateResult) {
				testutil.AssertLen(t, result.DeactivatedUsers, 2, "Deactivated users")
				testutil.AssertContains(t, result.DeactivatedUsers, "u1", "u1 should be deactivated")
				testutil.AssertContains(t, result.DeactivatedUsers, "u2", "u2 should be deactivated")

				testutil.AssertTrue(t, result.ReassignedPRs > 0, "Should have reassigned PRs")
				testutil.AssertEqual(t, result.Errors, 0, "Should have no errors")
			},
			wantErr: nil,
		},
		{
			name:     "returns error when team not found",
			teamName: "nonexistent",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// No users in this team
			},
			validate: nil,
			wantErr:  domain.ErrNotFound,
		},
		{
			name:     "handles team with no active members",
			teamName: "inactive-team",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "inactive-team", IsActive: false,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "inactive-team", IsActive: false,
				}
			},
			validate: func(t *testing.T, result *service.BulkDeactivateResult) {
				testutil.AssertLen(t, result.DeactivatedUsers, 0, "No users should be deactivated")
				testutil.AssertEqual(t, result.ReassignedPRs, 0, "No PRs should be reassigned")
			},
			wantErr: nil,
		},
		{
			name:     "removes reviewers when no replacement candidates available",
			teamName: "solo-team",
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				// Only one team with all members being deactivated
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", Username: "Alice", TeamName: "solo-team", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", Username: "Bob", TeamName: "solo-team", IsActive: true,
				}

				// Open PR with these reviewers
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID:     "pr-1",
					AuthorID:          "u1",
					Status:            domain.PRStatusOpen,
					AssignedReviewers: []string{"u2"},
				}
			},
			validate: func(t *testing.T, result *service.BulkDeactivateResult) {
				testutil.AssertLen(t, result.DeactivatedUsers, 2, "Both users should be deactivated")
				// When no candidates, reviewers are removed without replacement
				testutil.AssertEqual(t, result.Errors, 0, "Removal without replacement is not an error")
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			prRepo := testutil.NewMockPRRepository()
			userRepo := testutil.NewMockUserRepository()
			tt.setupMocks(prRepo, userRepo)

			logger := zap.NewNop()
			svc := service.NewStatsService(prRepo, userRepo, logger)

			// Act
			result, err := svc.BulkDeactivateTeam(context.Background(), tt.teamName)

			// Assert
			if tt.wantErr != nil {
				testutil.AssertErrorIs(t, err, tt.wantErr)
				return
			}

			testutil.AssertNoError(t, err)
			testutil.AssertNotNil(t, result)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
