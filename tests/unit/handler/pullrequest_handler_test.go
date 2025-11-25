package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/handler"
	"github.com/KruglovEgor/ReviewService/internal/service"
	"github.com/KruglovEgor/ReviewService/tests/testutil"
	"go.uber.org/zap"
)

// TestPullRequestHandler_CreatePullRequest tests PR creation endpoint
func TestPullRequestHandler_CreatePullRequest(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		expectedStatus int
		validate       func(*testing.T, *http.Response)
	}{
		{
			name: "successfully creates PR",
			requestBody: map[string]string{
				"pull_request_id":   "pr-001",
				"pull_request_name": "Add feature",
				"author_id":         "u1",
			},
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "backend", IsActive: true,
				}
			},
			expectedStatus: http.StatusCreated,
			validate: func(t *testing.T, resp *http.Response) {
				var response struct {
					PR domain.PullRequest `json:"pr"`
				}
				testutil.AssertJSONResponse(t, resp, http.StatusCreated, &response)
				testutil.AssertEqual(t, response.PR.PullRequestID, "pr-001", "PR ID")
				testutil.AssertEqual(t, response.PR.Status, domain.PRStatusOpen, "PR Status")
			},
		},
		{
			name:           "returns 400 for invalid JSON",
			requestBody:    "invalid json",
			setupMocks:     func(_ *testutil.MockPRRepository, _ *testutil.MockUserRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "returns 400 for missing required fields",
			requestBody: map[string]string{
				"pull_request_id": "pr-002",
				// Missing pull_request_name and author_id
			},
			setupMocks:     func(_ *testutil.MockPRRepository, _ *testutil.MockUserRepository) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "returns 409 when PR already exists",
			requestBody: map[string]string{
				"pull_request_id":   "pr-exists",
				"pull_request_name": "Duplicate",
				"author_id":         "u1",
			},
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-exists"] = &domain.PullRequest{PullRequestID: "pr-exists"}
				userRepo.Users["u1"] = &domain.User{
					UserID: "u1", TeamName: "backend", IsActive: true,
				}
				userRepo.Users["u2"] = &domain.User{
					UserID: "u2", TeamName: "backend", IsActive: true,
				}
			},
			expectedStatus: http.StatusConflict,
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
			h := handler.NewPullRequestHandler(svc, logger)

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				testutil.AssertNoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Act
			h.CreatePullRequest(w, req)
			resp := w.Result()

			// Assert
			testutil.AssertHTTPStatus(t, resp, tt.expectedStatus)

			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

// TestPullRequestHandler_MergePullRequest tests PR merge endpoint
func TestPullRequestHandler_MergePullRequest(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*testutil.MockPRRepository)
		expectedStatus int
	}{
		{
			name: "successfully merges PR",
			requestBody: map[string]string{
				"pull_request_id": "pr-001",
			},
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-001"] = &domain.PullRequest{
					PullRequestID: "pr-001",
					Status:        domain.PRStatusOpen,
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "idempotent - merging already merged PR returns 200",
			requestBody: map[string]string{
				"pull_request_id": "pr-merged",
			},
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-merged"] = &domain.PullRequest{
					PullRequestID: "pr-merged",
					Status:        domain.PRStatusMerged,
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "returns 404 when PR not found",
			requestBody: map[string]string{
				"pull_request_id": "pr-nonexistent",
			},
			setupMocks:     func(_ *testutil.MockPRRepository) {},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "returns 400 for invalid JSON",
			requestBody:    "invalid",
			setupMocks:     func(_ *testutil.MockPRRepository) {},
			expectedStatus: http.StatusBadRequest,
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
			h := handler.NewPullRequestHandler(svc, logger)

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				testutil.AssertNoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Act
			h.MergePullRequest(w, req)
			resp := w.Result()

			// Assert
			testutil.AssertHTTPStatus(t, resp, tt.expectedStatus)
		})
	}
}

// TestPullRequestHandler_ReassignReviewer tests reviewer reassignment endpoint
func TestPullRequestHandler_ReassignReviewer(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMocks     func(*testutil.MockPRRepository, *testutil.MockUserRepository)
		expectedStatus int
	}{
		{
			name: "successfully reassigns reviewer",
			requestBody: map[string]string{
				"pull_request_id": "pr-001",
				"old_user_id":     "u2",
			},
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
				userRepo.Users["u3"] = &domain.User{
					UserID: "u3", TeamName: "backend", IsActive: true,
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "returns 409 when user not assigned",
			requestBody: map[string]string{
				"pull_request_id": "pr-001",
				"old_user_id":     "u99",
			},
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
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "returns 409 when PR already merged",
			requestBody: map[string]string{
				"pull_request_id": "pr-merged",
				"old_user_id":     "u2",
			},
			setupMocks: func(prRepo *testutil.MockPRRepository, userRepo *testutil.MockUserRepository) {
				prRepo.PRs["pr-merged"] = &domain.PullRequest{
					PullRequestID:     "pr-merged",
					Status:            domain.PRStatusMerged,
					AssignedReviewers: []string{"u2"},
				}
			},
			expectedStatus: http.StatusConflict,
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
			h := handler.NewPullRequestHandler(svc, logger)

			body, err := json.Marshal(tt.requestBody)
			testutil.AssertNoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Act
			h.ReassignReviewer(w, req)
			resp := w.Result()

			// Assert
			testutil.AssertHTTPStatus(t, resp, tt.expectedStatus)
		})
	}
}

// TestPullRequestHandler_ListPullRequests tests PR listing endpoint
func TestPullRequestHandler_ListPullRequests(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		setupMocks     func(*testutil.MockPRRepository)
		expectedStatus int
		wantCount      int
	}{
		{
			name:        "lists all PRs when no filter",
			queryParams: "",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID: "pr-1", Status: domain.PRStatusOpen,
				}
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID: "pr-2", Status: domain.PRStatusMerged,
				}
			},
			expectedStatus: http.StatusOK,
			wantCount:      2,
		},
		{
			name:        "filters open PRs only",
			queryParams: "?status=OPEN",
			setupMocks: func(prRepo *testutil.MockPRRepository) {
				prRepo.PRs["pr-1"] = &domain.PullRequest{
					PullRequestID: "pr-1", Status: domain.PRStatusOpen,
				}
				prRepo.PRs["pr-2"] = &domain.PullRequest{
					PullRequestID: "pr-2", Status: domain.PRStatusMerged,
				}
			},
			expectedStatus: http.StatusOK,
			wantCount:      1,
		},
		{
			name:           "returns 400 for invalid status",
			queryParams:    "?status=INVALID",
			setupMocks:     func(_ *testutil.MockPRRepository) {},
			expectedStatus: http.StatusBadRequest,
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
			h := handler.NewPullRequestHandler(svc, logger)

			req := httptest.NewRequest(http.MethodGet, "/pullRequest/list"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Act
			h.ListPullRequests(w, req)
			resp := w.Result()

			// Assert
			testutil.AssertHTTPStatus(t, resp, tt.expectedStatus)

			if tt.expectedStatus == http.StatusOK {
				var response struct {
					PullRequests []*domain.PullRequest `json:"pull_requests"`
					Total        int                   `json:"total"`
				}
				testutil.AssertJSONResponse(t, resp, http.StatusOK, &response)
				testutil.AssertEqual(t, response.Total, tt.wantCount, "Total count")
			}
		})
	}
}
