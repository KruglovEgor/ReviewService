package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"reviewservice/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

// TeamRepository реализует domain.TeamRepository для PostgreSQL
type TeamRepository struct {
	db *sql.DB
}

// NewTeamRepository создаёт новый экземпляр TeamRepository
func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

// Create создаёт новую команду
func (r *TeamRepository) Create(ctx context.Context, team *domain.Team) error {
	query := `INSERT INTO teams (team_name) VALUES ($1)`

	_, err := r.db.ExecContext(ctx, query, team.TeamName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return domain.ErrTeamExists
		}
		return fmt.Errorf("failed to create team: %w", err)
	}

	return nil
}

// Get получает команду по имени вместе с участниками
func (r *TeamRepository) Get(ctx context.Context, teamName string) (*domain.Team, error) {
	// Проверяем существование команды
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`
	err := r.db.QueryRowContext(ctx, checkQuery, teamName).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check team existence: %w", err)
	}
	if !exists {
		return nil, domain.ErrNotFound
	}

	// Получаем участников команды
	query := `
		SELECT user_id, username, is_active
		FROM users
		WHERE team_name = $1
		ORDER BY username
	`

	rows, err := r.db.QueryContext(ctx, query, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()

	members := make([]domain.TeamMember, 0)
	for rows.Next() {
		var member domain.TeamMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.IsActive); err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating team members: %w", err)
	}

	return &domain.Team{
		TeamName: teamName,
		Members:  members,
	}, nil
}

// Exists проверяет существование команды
func (r *TeamRepository) Exists(ctx context.Context, teamName string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, teamName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team existence: %w", err)
	}

	return exists, nil
}
