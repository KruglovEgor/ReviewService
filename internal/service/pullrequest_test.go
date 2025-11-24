package service

import (
	"context"
	"testing"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"go.uber.org/zap"
)

// Mock репозиториев для тестирования
type mockPRRepo struct {
	prs map[string]*domain.PullRequest
}

func (m *mockPRRepo) Create(ctx context.Context, pr *domain.PullRequest) error {
	if _, exists := m.prs[pr.PullRequestID]; exists {
		return domain.ErrPRExists
	}
	m.prs[pr.PullRequestID] = pr
	return nil
}

func (m *mockPRRepo) Get(ctx context.Context, prID string) (*domain.PullRequest, error) {
	pr, ok := m.prs[prID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return pr, nil
}

func (m *mockPRRepo) Exists(ctx context.Context, prID string) (bool, error) {
	_, exists := m.prs[prID]
	return exists, nil
}

func (m *mockPRRepo) AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	pr, ok := m.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	pr.AssignedReviewers = reviewerIDs
	return nil
}

// Заглушки для других методов
func (m *mockPRRepo) Update(ctx context.Context, pr *domain.PullRequest) error           { return nil }
func (m *mockPRRepo) Merge(ctx context.Context, prID string) (*domain.PullRequest, error) { return nil, nil }
func (m *mockPRRepo) GetByReviewer(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	return nil, nil
}
func (m *mockPRRepo) GetOpenByReviewer(ctx context.Context, userID string) ([]string, error) { return nil, nil }
func (m *mockPRRepo) RemoveReviewer(ctx context.Context, prID string, reviewerID string) error { return nil }
func (m *mockPRRepo) AddReviewer(ctx context.Context, prID string, reviewerID string) error { return nil }
func (m *mockPRRepo) GetReviewers(ctx context.Context, prID string) ([]string, error) { return nil, nil }
func (m *mockPRRepo) ReassignReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	return nil
}

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) Get(ctx context.Context, userID string) (*domain.User, error) {
	user, ok := m.users[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByTeam(ctx context.Context, teamName string) ([]domain.User, error) {
	users := make([]domain.User, 0)
	for _, user := range m.users {
		if user.TeamName == teamName {
			users = append(users, *user)
		}
	}
	return users, nil
}

// Заглушки
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) SetIsActive(ctx context.Context, userID string, isActive bool) error {
	return nil
}
func (m *mockUserRepo) BulkDeactivateByTeam(ctx context.Context, teamName string) ([]string, error) {
	return nil, nil
}

// Тесты
func TestCreatePullRequest_Success(t *testing.T) {
	// Arrange - подготовка
	prRepo := &mockPRRepo{prs: make(map[string]*domain.PullRequest)}
	userRepo := &mockUserRepo{
		users: map[string]*domain.User{
			"u1": {UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
			"u2": {UserID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
			"u3": {UserID: "u3", Username: "Charlie", TeamName: "backend", IsActive: true},
		},
	}
	
	logger := zap.NewNop()
	service := NewPullRequestService(prRepo, userRepo, logger)
	
	// Act - выполнение
	pr, err := service.CreatePullRequest(context.Background(), "pr-001", "Add feature", "u1")
	
	// Assert - проверка
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if pr.PullRequestID != "pr-001" {
		t.Errorf("expected pr-001, got %s", pr.PullRequestID)
	}
	
	if pr.Status != domain.PRStatusOpen {
		t.Errorf("expected OPEN status, got %s", pr.Status)
	}
	
	if len(pr.AssignedReviewers) > 2 {
		t.Errorf("expected max 2 reviewers, got %d", len(pr.AssignedReviewers))
	}
	
	// Проверяем, что автор не назначен ревьювером
	for _, reviewer := range pr.AssignedReviewers {
		if reviewer == "u1" {
			t.Error("author should not be assigned as reviewer")
		}
	}
}

func TestCreatePullRequest_DuplicateID(t *testing.T) {
	prRepo := &mockPRRepo{
		prs: map[string]*domain.PullRequest{
			"pr-001": {PullRequestID: "pr-001"},
		},
	}
	userRepo := &mockUserRepo{
		users: map[string]*domain.User{
			"u1": {UserID: "u1", TeamName: "backend", IsActive: true},
		},
	}
	
	logger := zap.NewNop()
	service := NewPullRequestService(prRepo, userRepo, logger)
	
	_, err := service.CreatePullRequest(context.Background(), "pr-001", "Test", "u1")
	
	if err != domain.ErrPRExists {
		t.Errorf("expected ErrPRExists, got %v", err)
	}
}

func TestSelectReviewers(t *testing.T) {
	service := &PullRequestService{}
	
	tests := []struct {
		name      string
		members   []domain.User
		authorID  string
		maxCount  int
		wantCount int
	}{
		{
			name: "select 2 from 3 members",
			members: []domain.User{
				{UserID: "u1", IsActive: true},
				{UserID: "u2", IsActive: true},
				{UserID: "u3", IsActive: true},
			},
			authorID:  "u1",
			maxCount:  2,
			wantCount: 2,
		},
		{
			name: "skip inactive members",
			members: []domain.User{
				{UserID: "u1", IsActive: true},
				{UserID: "u2", IsActive: false},
				{UserID: "u3", IsActive: true},
			},
			authorID:  "u1",
			maxCount:  2,
			wantCount: 1,
		},
		{
			name: "no reviewers if only author",
			members: []domain.User{
				{UserID: "u1", IsActive: true},
			},
			authorID:  "u1",
			maxCount:  2,
			wantCount: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewers := service.selectReviewers(tt.members, tt.authorID, tt.maxCount)
			
			if len(reviewers) != tt.wantCount {
				t.Errorf("expected %d reviewers, got %d", tt.wantCount, len(reviewers))
			}
			
			// Проверяем, что автор не в списке
			for _, r := range reviewers {
				if r == tt.authorID {
					t.Error("author should not be in reviewers list")
				}
			}
		})
	}
}
