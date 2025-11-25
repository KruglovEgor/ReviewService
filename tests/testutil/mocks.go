// Package testutil provides common testing fixtures and mock implementations
package testutil

import (
	"context"

	"github.com/KruglovEgor/ReviewService/internal/domain"
)

// MockPRRepository implements domain.PullRequestRepository for testing
type MockPRRepository struct {
	PRs map[string]*domain.PullRequest

	// Hooks for custom behavior
	CreateFunc                 func(ctx context.Context, pr *domain.PullRequest) error
	GetFunc                    func(ctx context.Context, prID string) (*domain.PullRequest, error)
	MergeFunc                  func(ctx context.Context, prID string) (*domain.PullRequest, error)
	ReassignReviewerFunc       func(ctx context.Context, prID, oldID, newID string) error
	GetPRStatsFunc             func(ctx context.Context) (map[string]int, error)
	GetUserAssignmentStatsFunc func(ctx context.Context) (map[string]*domain.UserAssignmentStats, error)
}

// NewMockPRRepository creates a new mock PR repository
func NewMockPRRepository() *MockPRRepository {
	return &MockPRRepository{
		PRs: make(map[string]*domain.PullRequest),
	}
}

func (m *MockPRRepository) Create(ctx context.Context, pr *domain.PullRequest) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, pr)
	}
	if _, exists := m.PRs[pr.PullRequestID]; exists {
		return domain.ErrPRExists
	}
	m.PRs[pr.PullRequestID] = pr
	return nil
}

func (m *MockPRRepository) Get(ctx context.Context, prID string) (*domain.PullRequest, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, prID)
	}
	pr, ok := m.PRs[prID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return pr, nil
}

func (m *MockPRRepository) Exists(ctx context.Context, prID string) (bool, error) {
	_, exists := m.PRs[prID]
	return exists, nil
}

func (m *MockPRRepository) Update(ctx context.Context, pr *domain.PullRequest) error {
	if _, ok := m.PRs[pr.PullRequestID]; !ok {
		return domain.ErrNotFound
	}
	m.PRs[pr.PullRequestID] = pr
	return nil
}

func (m *MockPRRepository) Merge(ctx context.Context, prID string) (*domain.PullRequest, error) {
	if m.MergeFunc != nil {
		return m.MergeFunc(ctx, prID)
	}
	pr, ok := m.PRs[prID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	pr.Status = domain.PRStatusMerged
	return pr, nil
}

func (m *MockPRRepository) AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	pr, ok := m.PRs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	pr.AssignedReviewers = reviewerIDs
	return nil
}

func (m *MockPRRepository) GetByReviewer(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	var result []domain.PullRequestShort
	for _, pr := range m.PRs {
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == userID {
				result = append(result, domain.PullRequestShort{
					PullRequestID:   pr.PullRequestID,
					PullRequestName: pr.PullRequestName,
					AuthorID:        pr.AuthorID,
					Status:          pr.Status,
				})
				break
			}
		}
	}
	return result, nil
}

func (m *MockPRRepository) GetOpenByReviewer(ctx context.Context, userID string) ([]string, error) {
	var result []string
	for _, pr := range m.PRs {
		if pr.Status != domain.PRStatusOpen {
			continue
		}
		for _, reviewer := range pr.AssignedReviewers {
			if reviewer == userID {
				result = append(result, pr.PullRequestID)
				break
			}
		}
	}
	return result, nil
}

func (m *MockPRRepository) RemoveReviewer(ctx context.Context, prID string, reviewerID string) error {
	pr, ok := m.PRs[prID]
	if !ok {
		return domain.ErrNotFound
	}

	newReviewers := make([]string, 0, len(pr.AssignedReviewers))
	for _, r := range pr.AssignedReviewers {
		if r != reviewerID {
			newReviewers = append(newReviewers, r)
		}
	}
	pr.AssignedReviewers = newReviewers
	return nil
}

func (m *MockPRRepository) AddReviewer(ctx context.Context, prID string, reviewerID string) error {
	pr, ok := m.PRs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	pr.AssignedReviewers = append(pr.AssignedReviewers, reviewerID)
	return nil
}

func (m *MockPRRepository) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	pr, ok := m.PRs[prID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return pr.AssignedReviewers, nil
}

func (m *MockPRRepository) ReassignReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	if m.ReassignReviewerFunc != nil {
		return m.ReassignReviewerFunc(ctx, prID, oldReviewerID, newReviewerID)
	}

	pr, ok := m.PRs[prID]
	if !ok {
		return domain.ErrNotFound
	}

	found := false
	for i, r := range pr.AssignedReviewers {
		if r == oldReviewerID {
			pr.AssignedReviewers[i] = newReviewerID
			found = true
			break
		}
	}

	if !found {
		return domain.ErrNotFound
	}

	return nil
}

func (m *MockPRRepository) GetPRStats(ctx context.Context) (map[string]int, error) {
	if m.GetPRStatsFunc != nil {
		return m.GetPRStatsFunc(ctx)
	}

	total := len(m.PRs)
	open := 0
	merged := 0
	totalReviewers := 0

	for _, pr := range m.PRs {
		if pr.Status == domain.PRStatusOpen {
			open++
		} else if pr.Status == domain.PRStatusMerged {
			merged++
		}
		totalReviewers += len(pr.AssignedReviewers)
	}

	avgReviewers := 0
	if total > 0 {
		avgReviewers = (totalReviewers * 100) / total
	}

	return map[string]int{
		"total":         total,
		"open":          open,
		"merged":        merged,
		"avg_reviewers": avgReviewers,
	}, nil
}

func (m *MockPRRepository) GetUserAssignmentStats(ctx context.Context) (map[string]*domain.UserAssignmentStats, error) {
	if m.GetUserAssignmentStatsFunc != nil {
		return m.GetUserAssignmentStatsFunc(ctx)
	}

	stats := make(map[string]*domain.UserAssignmentStats)

	for _, pr := range m.PRs {
		for _, reviewerID := range pr.AssignedReviewers {
			if _, exists := stats[reviewerID]; !exists {
				stats[reviewerID] = &domain.UserAssignmentStats{
					UserID: reviewerID,
				}
			}

			s := stats[reviewerID]
			s.TotalAssignments++

			if pr.Status == domain.PRStatusOpen {
				s.OpenPRs++
			} else if pr.Status == domain.PRStatusMerged {
				s.MergedPRs++
			}
		}
	}

	return stats, nil
}

func (m *MockPRRepository) List(ctx context.Context, status string) ([]*domain.PullRequest, error) {
	result := make([]*domain.PullRequest, 0, len(m.PRs))

	for _, pr := range m.PRs {
		if status == "" || string(pr.Status) == status {
			result = append(result, pr)
		}
	}

	return result, nil
}

// MockUserRepository implements domain.UserRepository for testing
type MockUserRepository struct {
	Users map[string]*domain.User

	// Hooks for custom behavior
	GetFunc         func(ctx context.Context, userID string) (*domain.User, error)
	GetByTeamFunc   func(ctx context.Context, teamName string) ([]domain.User, error)
	SetIsActiveFunc func(ctx context.Context, userID string, isActive bool) error
}

// NewMockUserRepository creates a new mock user repository
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		Users: make(map[string]*domain.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if _, exists := m.Users[user.UserID]; exists {
		return domain.ErrUserExists
	}
	m.Users[user.UserID] = user
	return nil
}

func (m *MockUserRepository) Update(ctx context.Context, user *domain.User) error {
	if _, ok := m.Users[user.UserID]; !ok {
		return domain.ErrNotFound
	}
	m.Users[user.UserID] = user
	return nil
}

func (m *MockUserRepository) Get(ctx context.Context, userID string) (*domain.User, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, userID)
	}

	user, ok := m.Users[userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (m *MockUserRepository) GetByTeam(ctx context.Context, teamName string) ([]domain.User, error) {
	if m.GetByTeamFunc != nil {
		return m.GetByTeamFunc(ctx, teamName)
	}

	users := make([]domain.User, 0)
	for _, user := range m.Users {
		if user.TeamName == teamName {
			users = append(users, *user)
		}
	}
	return users, nil
}

func (m *MockUserRepository) SetIsActive(ctx context.Context, userID string, isActive bool) error {
	if m.SetIsActiveFunc != nil {
		return m.SetIsActiveFunc(ctx, userID, isActive)
	}

	user, ok := m.Users[userID]
	if !ok {
		return domain.ErrNotFound
	}
	user.IsActive = isActive
	return nil
}

func (m *MockUserRepository) BulkDeactivateByTeam(ctx context.Context, teamName string) ([]string, error) {
	var deactivated []string
	for _, user := range m.Users {
		if user.TeamName == teamName && user.IsActive {
			user.IsActive = false
			deactivated = append(deactivated, user.UserID)
		}
	}
	return deactivated, nil
}

// MockTeamRepository implements domain.TeamRepository for testing
type MockTeamRepository struct {
	Teams map[string]*domain.Team
}

// NewMockTeamRepository creates a new mock team repository
func NewMockTeamRepository() *MockTeamRepository {
	return &MockTeamRepository{
		Teams: make(map[string]*domain.Team),
	}
}

func (m *MockTeamRepository) Create(ctx context.Context, team *domain.Team) error {
	if _, exists := m.Teams[team.TeamName]; exists {
		return domain.ErrTeamExists
	}
	m.Teams[team.TeamName] = team
	return nil
}

func (m *MockTeamRepository) Get(ctx context.Context, teamName string) (*domain.Team, error) {
	team, ok := m.Teams[teamName]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return team, nil
}

func (m *MockTeamRepository) Exists(ctx context.Context, teamName string) (bool, error) {
	_, exists := m.Teams[teamName]
	return exists, nil
}
