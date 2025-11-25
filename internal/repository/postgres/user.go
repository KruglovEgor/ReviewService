package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UserRepository реализует domain.UserRepository для PostgreSQL
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository создаёт новый экземпляр UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create создаёт нового пользователя
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (user_id, username, team_name, is_active)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(ctx, query, user.UserID, user.Username, user.TeamName, user.IsActive)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" { // unique_violation
				return fmt.Errorf("user already exists: %w", err)
			}
			if pgErr.Code == "23503" { // foreign_key_violation
				return fmt.Errorf("team not found: %w", err)
			}
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// Update обновляет существующего пользователя
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET username = $2, team_name = $3, is_active = $4
		WHERE user_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, user.UserID, user.Username, user.TeamName, user.IsActive)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return fmt.Errorf("team not found: %w", err)
		}
		return fmt.Errorf("failed to update user: %w", err)
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

// Get получает пользователя по ID
func (r *UserRepository) Get(ctx context.Context, userID string) (*domain.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active
		FROM users
		WHERE user_id = $1
	`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.UserID,
		&user.Username,
		&user.TeamName,
		&user.IsActive,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// GetByTeam получает всех пользователей команды
func (r *UserRepository) GetByTeam(ctx context.Context, teamName string) ([]domain.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active
		FROM users
		WHERE team_name = $1
		ORDER BY username
	`

	rows, err := r.db.QueryContext(ctx, query, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by team: %w", err)
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// SetIsActive устанавливает флаг активности пользователя
func (r *UserRepository) SetIsActive(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = $2 WHERE user_id = $1`

	result, err := r.db.ExecContext(ctx, query, userID, isActive)
	if err != nil {
		return fmt.Errorf("failed to set user active status: %w", err)
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

// BulkDeactivateByTeam массово деактивирует пользователей команды
// Возвращает список ID деактивированных пользователей
func (r *UserRepository) BulkDeactivateByTeam(ctx context.Context, teamName string) ([]string, error) {
	query := `
		UPDATE users
		SET is_active = false
		WHERE team_name = $1 AND is_active = true
		RETURNING user_id
	`

	rows, err := r.db.QueryContext(ctx, query, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to bulk deactivate users: %w", err)
	}
	defer rows.Close()

	deactivatedIDs := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan deactivated user ID: %w", err)
		}
		deactivatedIDs = append(deactivatedIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating deactivated users: %w", err)
	}

	return deactivatedIDs, nil
}
