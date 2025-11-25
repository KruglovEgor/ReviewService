package domain

import "context"

// UserAssignmentStats представляет статистику назначений пользователя
type UserAssignmentStats struct {
	UserID           string
	Username         string
	TotalAssignments int
	OpenPRs          int
	MergedPRs        int
}

// TeamRepository определяет интерфейс для работы с командами
type TeamRepository interface {
	// Create создаёт новую команду
	Create(ctx context.Context, team *Team) error

	// Get получает команду по имени
	Get(ctx context.Context, teamName string) (*Team, error)

	// Exists проверяет существование команды
	Exists(ctx context.Context, teamName string) (bool, error)
}

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	// Create создаёт нового пользователя
	Create(ctx context.Context, user *User) error

	// Update обновляет существующего пользователя
	Update(ctx context.Context, user *User) error

	// Get получает пользователя по ID
	Get(ctx context.Context, userID string) (*User, error)

	// GetByTeam получает всех пользователей команды
	GetByTeam(ctx context.Context, teamName string) ([]User, error)

	// GetActiveUsersExcludingTeam получает всех активных пользователей кроме указанной команды
	GetActiveUsersExcludingTeam(ctx context.Context, excludeTeamName string) ([]User, error)

	// SetIsActive устанавливает флаг активности пользователя
	SetIsActive(ctx context.Context, userID string, isActive bool) error

	// BulkDeactivateByTeam массово деактивирует пользователей команды
	BulkDeactivateByTeam(ctx context.Context, teamName string) ([]string, error)
}

// PullRequestRepository определяет интерфейс для работы с PR
type PullRequestRepository interface {
	// Create создаёт новый PR
	Create(ctx context.Context, pr *PullRequest) error

	// Get получает PR по ID
	Get(ctx context.Context, prID string) (*PullRequest, error)

	// Update обновляет PR
	Update(ctx context.Context, pr *PullRequest) error

	// Merge помечает PR как смердженный
	Merge(ctx context.Context, prID string) (*PullRequest, error)

	// GetByReviewer получает PR'ы, где пользователь назначен ревьювером
	GetByReviewer(ctx context.Context, userID string) ([]PullRequestShort, error)

	// GetOpenByReviewer получает открытые PR'ы пользователя
	GetOpenByReviewer(ctx context.Context, userID string) ([]string, error)

	// Exists проверяет существование PR
	Exists(ctx context.Context, prID string) (bool, error)

	// AssignReviewers назначает ревьюверов на PR
	AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error

	// RemoveReviewer удаляет ревьювера из PR
	RemoveReviewer(ctx context.Context, prID string, reviewerID string) error

	// AddReviewer добавляет ревьювера в PR
	AddReviewer(ctx context.Context, prID string, reviewerID string) error

	// GetReviewers получает список ревьюверов PR
	GetReviewers(ctx context.Context, prID string) ([]string, error)

	// ReassignReviewer переназначает ревьювера
	ReassignReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error

	// GetPRStats возвращает общую статистику по PR (total, open, merged, avg_reviewers)
	GetPRStats(ctx context.Context) (map[string]int, error)

	// GetUserAssignmentStats возвращает статистику назначений по пользователям
	GetUserAssignmentStats(ctx context.Context) (map[string]*UserAssignmentStats, error)

	// List возвращает список PR с фильтрами
	List(ctx context.Context, status string) ([]*PullRequest, error)
}
