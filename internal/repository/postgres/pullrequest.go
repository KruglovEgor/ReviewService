package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PullRequestRepository реализует domain.PullRequestRepository для PostgreSQL
type PullRequestRepository struct {
	db *sql.DB
}

// NewPullRequestRepository создаёт новый экземпляр PullRequestRepository
func NewPullRequestRepository(db *sql.DB) *PullRequestRepository {
	return &PullRequestRepository{db: db}
}

// Create создаёт новый PR
func (r *PullRequestRepository) Create(ctx context.Context, pr *domain.PullRequest) error {
	query := `
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	createdAt := time.Now()
	if pr.CreatedAt != nil {
		createdAt = *pr.CreatedAt
	}

	_, err := r.db.ExecContext(ctx, query, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return domain.ErrPRExists
			}
			if pgErr.Code == "23503" { // foreign_key_violation
				return domain.ErrNotFound
			}
		}
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	return nil
}

// Get получает PR по ID
func (r *PullRequestRepository) Get(ctx context.Context, prID string) (*domain.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id = $1
	`

	var pr domain.PullRequest
	var createdAt time.Time
	var mergedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, prID).Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&createdAt,
		&mergedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	pr.CreatedAt = &createdAt
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}

	// Получаем ревьюверов
	reviewers, err := r.GetReviewers(ctx, prID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewers: %w", err)
	}
	pr.AssignedReviewers = reviewers

	return &pr, nil
}

// Update обновляет PR
func (r *PullRequestRepository) Update(ctx context.Context, pr *domain.PullRequest) error {
	query := `
		UPDATE pull_requests
		SET pull_request_name = $2, author_id = $3, status = $4, merged_at = $5
		WHERE pull_request_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.MergedAt)
	if err != nil {
		return fmt.Errorf("failed to update pull request: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Merge помечает PR как смердженный (идемпотентная операция)
func (r *PullRequestRepository) Merge(ctx context.Context, prID string) (*domain.PullRequest, error) {
	// Получаем текущее состояние PR
	pr, err := r.Get(ctx, prID)
	if err != nil {
		return nil, err
	}

	// Если уже смерджен, просто возвращаем текущее состояние (идемпотентность)
	if pr.Status == domain.PRStatusMerged {
		return pr, nil
	}

	// Обновляем статус и время мерджа
	mergedAt := time.Now()
	query := `
		UPDATE pull_requests
		SET status = $2, merged_at = $3
		WHERE pull_request_id = $1
	`

	_, err = r.db.ExecContext(ctx, query, prID, domain.PRStatusMerged, mergedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to merge pull request: %w", err)
	}

	pr.Status = domain.PRStatusMerged
	pr.MergedAt = &mergedAt

	return pr, nil
}

// GetByReviewer получает PR'ы, где пользователь назначен ревьювером
func (r *PullRequestRepository) GetByReviewer(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	query := `
		SELECT DISTINCT p.pull_request_id, p.pull_request_name, p.author_id, p.status, p.created_at
		FROM pull_requests p
		INNER JOIN pr_reviewers pr ON p.pull_request_id = pr.pull_request_id
		WHERE pr.user_id = $1
		ORDER BY p.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests by reviewer: %w", err)
	}
	defer rows.Close()

	prs := make([]domain.PullRequestShort, 0)
	for rows.Next() {
		var pr domain.PullRequestShort
		var createdAt time.Time
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan pull request: %w", err)
		}
		prs = append(prs, pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pull requests: %w", err)
	}

	return prs, nil
}

// GetOpenByReviewer получает открытые PR'ы пользователя
func (r *PullRequestRepository) GetOpenByReviewer(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT DISTINCT p.pull_request_id
		FROM pull_requests p
		INNER JOIN pr_reviewers pr ON p.pull_request_id = pr.pull_request_id
		WHERE pr.user_id = $1 AND p.status = $2
	`

	rows, err := r.db.QueryContext(ctx, query, userID, domain.PRStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("failed to get open pull requests: %w", err)
	}
	defer rows.Close()

	prIDs := make([]string, 0)
	for rows.Next() {
		var prID string
		if err := rows.Scan(&prID); err != nil {
			return nil, fmt.Errorf("failed to scan PR ID: %w", err)
		}
		prIDs = append(prIDs, prID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating PR IDs: %w", err)
	}

	return prIDs, nil
}

// Exists проверяет существование PR
func (r *PullRequestRepository) Exists(ctx context.Context, prID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, prID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check PR existence: %w", err)
	}

	return exists, nil
}

// AssignReviewers назначает ревьюверов на PR
func (r *PullRequestRepository) AssignReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	if len(reviewerIDs) == 0 {
		return nil
	}

	// Используем транзакцию для атомарности
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `INSERT INTO pr_reviewers (pull_request_id, user_id) VALUES ($1, $2)`

	for _, reviewerID := range reviewerIDs {
		_, err := tx.ExecContext(ctx, query, prID, reviewerID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				// Ревьювер уже назначен, пропускаем
				continue
			}
			return fmt.Errorf("failed to assign reviewer %s: %w", reviewerID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RemoveReviewer удаляет ревьювера из PR
func (r *PullRequestRepository) RemoveReviewer(ctx context.Context, prID string, reviewerID string) error {
	query := `DELETE FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, prID, reviewerID)
	if err != nil {
		return fmt.Errorf("failed to remove reviewer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotAssigned
	}

	return nil
}

// AddReviewer добавляет ревьювера в PR
func (r *PullRequestRepository) AddReviewer(ctx context.Context, prID string, reviewerID string) error {
	query := `INSERT INTO pr_reviewers (pull_request_id, user_id) VALUES ($1, $2)`

	_, err := r.db.ExecContext(ctx, query, prID, reviewerID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Ревьювер уже назначен
			return nil
		}
		return fmt.Errorf("failed to add reviewer: %w", err)
	}

	return nil
}

// GetReviewers получает список ревьюверов PR
func (r *PullRequestRepository) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	query := `
		SELECT user_id
		FROM pr_reviewers
		WHERE pull_request_id = $1
		ORDER BY assigned_at
	`

	rows, err := r.db.QueryContext(ctx, query, prID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewers: %w", err)
	}
	defer rows.Close()

	reviewers := make([]string, 0)
	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, fmt.Errorf("failed to scan reviewer: %w", err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reviewers: %w", err)
	}

	return reviewers, nil
}

// ReassignReviewer переназначает ревьювера (атомарная операция)
func (r *PullRequestRepository) ReassignReviewer(ctx context.Context, prID, oldReviewerID, newReviewerID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Удаляем старого ревьювера
	deleteQuery := `DELETE FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2`
	result, err := tx.ExecContext(ctx, deleteQuery, prID, oldReviewerID)
	if err != nil {
		return fmt.Errorf("failed to remove old reviewer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNotAssigned
	}

	// Добавляем нового ревьювера
	insertQuery := `INSERT INTO pr_reviewers (pull_request_id, user_id) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, insertQuery, prID, newReviewerID)
	if err != nil {
		return fmt.Errorf("failed to add new reviewer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetPRStats возвращает общую статистику по PR
func (r *PullRequestRepository) GetPRStats(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = $1) as open,
			COUNT(*) FILTER (WHERE status = $2) as merged,
			ROUND(COALESCE(AVG(COALESCE(reviewer_count, 0)), 0) * 100) as avg_reviewers_x100
		FROM pull_requests
		LEFT JOIN (
			SELECT pull_request_id, COUNT(*) as reviewer_count
			FROM pr_reviewers
			GROUP BY pull_request_id
		) r ON pull_requests.pull_request_id = r.pull_request_id
	`

	var total, open, merged, avgReviewersX100 int
	err := r.db.QueryRowContext(ctx, query, domain.PRStatusOpen, domain.PRStatusMerged).Scan(
		&total, &open, &merged, &avgReviewersX100,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR stats: %w", err)
	}

	stats := map[string]int{
		"total":         total,
		"open":          open,
		"merged":        merged,
		"avg_reviewers": avgReviewersX100, // Возвращаем умноженное на 100 для точности
	}

	return stats, nil
} // GetUserAssignmentStats возвращает статистику назначений по пользователям
func (r *PullRequestRepository) GetUserAssignmentStats(ctx context.Context) (map[string]*domain.UserAssignmentStats, error) {
	query := `
		SELECT 
			pr.user_id,
			COUNT(*) as total_assignments,
			COUNT(*) FILTER (WHERE p.status = $1) as open_prs,
			COUNT(*) FILTER (WHERE p.status = $2) as merged_prs
		FROM pr_reviewers pr
		INNER JOIN pull_requests p ON pr.pull_request_id = p.pull_request_id
		GROUP BY pr.user_id
	`

	rows, err := r.db.QueryContext(ctx, query, domain.PRStatusOpen, domain.PRStatusMerged)
	if err != nil {
		return nil, fmt.Errorf("failed to get user assignment stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]*domain.UserAssignmentStats)
	for rows.Next() {
		var userID string
		var total, open, merged int

		if err := rows.Scan(&userID, &total, &open, &merged); err != nil {
			return nil, fmt.Errorf("failed to scan user stats: %w", err)
		}

		stats[userID] = &domain.UserAssignmentStats{
			UserID:           userID,
			TotalAssignments: total,
			OpenPRs:          open,
			MergedPRs:        merged,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user stats: %w", err)
	}

	return stats, nil
}

// List возвращает список PR с фильтрацией по статусу
func (r *PullRequestRepository) List(ctx context.Context, status string) ([]*domain.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests
	`

	args := []interface{}{}
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}
	defer rows.Close()

	prs := make([]*domain.PullRequest, 0)
	for rows.Next() {
		var pr domain.PullRequest
		var createdAt time.Time
		var mergedAt sql.NullTime

		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &createdAt, &mergedAt); err != nil {
			return nil, fmt.Errorf("failed to scan pull request: %w", err)
		}

		pr.CreatedAt = &createdAt
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}

		// Получаем ревьюверов для каждого PR
		reviewers, err := r.GetReviewers(ctx, pr.PullRequestID)
		if err != nil {
			return nil, fmt.Errorf("failed to get reviewers for PR %s: %w", pr.PullRequestID, err)
		}
		pr.AssignedReviewers = reviewers

		prs = append(prs, &pr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pull requests: %w", err)
	}

	return prs, nil
}
