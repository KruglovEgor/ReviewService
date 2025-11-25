package service

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"reviewservice/internal/domain"
	"reviewservice/internal/testutil"
)

// TestUserService_GetUser tests getting a user
func TestUserService_GetUser(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	userRepo := &testutil.MockUserRepository{
		Users: map[string]*domain.User{
			"u1": {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		},
	}

	prRepo := &testutil.MockPRRepository{
		PRs: make(map[string]*domain.PullRequest),
	}

	svc := NewUserService(userRepo, prRepo, logger)

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "existing user",
			userID:  "u1",
			wantErr: false,
		},
		{
			name:    "non-existing user",
			userID:  "u999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := svc.GetUser(context.Background(), tt.userID)

			if tt.wantErr {
				testutil.AssertNotNil(t, err, "Error expected")
			} else {
				testutil.AssertNil(t, err, "No error expected")
				testutil.AssertNotNil(t, user, "User should not be nil")
				testutil.AssertEqual(t, user.UserID, tt.userID, "User ID")
			}
		})
	}
}

// TestPRStatus_IsValid tests PRStatus validation
func TestPRStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status domain.PRStatus
		want   bool
	}{
		{
			name:   "valid OPEN",
			status: domain.PRStatusOpen,
			want:   true,
		},
		{
			name:   "valid MERGED",
			status: domain.PRStatusMerged,
			want:   true,
		},
		{
			name:   "invalid status",
			status: domain.PRStatus("INVALID"),
			want:   false,
		},
		{
			name:   "empty status",
			status: domain.PRStatus(""),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValid()
			testutil.AssertEqual(t, got, tt.want, "IsValid result")
		})
	}
}
