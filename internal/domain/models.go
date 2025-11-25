package domain

import "time"

// PRStatus представляет статус Pull Request
type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

// IsValid проверяет валидность статуса PR
func (s PRStatus) IsValid() bool {
	return s == PRStatusOpen || s == PRStatusMerged
}

// User представляет пользователя системы
type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

// TeamMember представляет участника команды
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// Team представляет команду
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// PullRequest представляет Pull Request с полной информацией
type PullRequest struct {
	PullRequestID     string     `json:"pull_request_id"`
	PullRequestName   string     `json:"pull_request_name"`
	AuthorID          string     `json:"author_id"`
	Status            PRStatus   `json:"status"`
	AssignedReviewers []string   `json:"assigned_reviewers"`
	CreatedAt         *time.Time `json:"createdAt,omitempty"`
	MergedAt          *time.Time `json:"mergedAt,omitempty"`
}

// PullRequestShort представляет краткую информацию о PR
type PullRequestShort struct {
	PullRequestID   string   `json:"pull_request_id"`
	PullRequestName string   `json:"pull_request_name"`
	AuthorID        string   `json:"author_id"`
	Status          PRStatus `json:"status"`
}

// UserPullRequests представляет список PR'ов пользователя
type UserPullRequests struct {
	UserID       string             `json:"user_id"`
	PullRequests []PullRequestShort `json:"pull_requests"`
}
