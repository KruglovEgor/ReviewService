package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/repository/postgres"
	"go.uber.org/zap"
)

// TeamService реализует бизнес-логику для работы с командами
type TeamService struct {
	teamRepo  domain.TeamRepository
	userRepo  domain.UserRepository
	txManager *postgres.TxManager
	logger    *zap.Logger
}

// NewTeamService создаёт новый экземпляр TeamService
func NewTeamService(
	teamRepo domain.TeamRepository,
	userRepo domain.UserRepository,
	txManager *postgres.TxManager,
	logger *zap.Logger,
) *TeamService {
	return &TeamService{
		teamRepo:  teamRepo,
		userRepo:  userRepo,
		txManager: txManager,
		logger:    logger,
	}
}

// CreateTeam создаёт команду и добавляет/обновляет её участников
// Операция выполняется в транзакции для атомарности
func (s *TeamService) CreateTeam(ctx context.Context, team *domain.Team) (*domain.Team, error) {
	// Проверяем, существует ли команда
	exists, err := s.teamRepo.Exists(ctx, team.TeamName)
	if err != nil {
		s.logger.Error("failed to check team existence", zap.Error(err), zap.String("team_name", team.TeamName))
		return nil, fmt.Errorf("failed to check team existence: %w", err)
	}

	if exists {
		return nil, domain.ErrTeamExists
	}

	// Выполняем всю операцию в транзакции
	err = s.txManager.WithinTransaction(ctx, func(tx *sql.Tx) error {
		// Создаём команду
		query := `INSERT INTO teams (team_name) VALUES ($1)`
		if _, err := tx.ExecContext(ctx, query, team.TeamName); err != nil {
			return fmt.Errorf("failed to create team: %w", err)
		}

		s.logger.Info("team created in transaction", zap.String("team_name", team.TeamName))

		// Создаём/обновляем пользователей
		for _, member := range team.Members {
			// Проверяем существование пользователя
			var exists bool
			checkQuery := `SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1)`
			if err := tx.QueryRowContext(ctx, checkQuery, member.UserID).Scan(&exists); err != nil {
				return fmt.Errorf("failed to check user existence: %w", err)
			}

			if !exists {
				// Создаём нового пользователя
				insertQuery := `
					INSERT INTO users (user_id, username, team_name, is_active)
					VALUES ($1, $2, $3, $4)
				`
				if _, err := tx.ExecContext(ctx, insertQuery, member.UserID, member.Username, team.TeamName, member.IsActive); err != nil {
					return fmt.Errorf("failed to create user %s: %w", member.UserID, err)
				}
				s.logger.Info("user created in transaction",
					zap.String("user_id", member.UserID),
					zap.String("team_name", team.TeamName))
			} else {
				// Обновляем существующего пользователя
				updateQuery := `
					UPDATE users
					SET username = $2, team_name = $3, is_active = $4
					WHERE user_id = $1
				`
				if _, err := tx.ExecContext(ctx, updateQuery, member.UserID, member.Username, team.TeamName, member.IsActive); err != nil {
					return fmt.Errorf("failed to update user %s: %w", member.UserID, err)
				}
				s.logger.Info("user updated in transaction",
					zap.String("user_id", member.UserID),
					zap.String("team_name", team.TeamName))
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Error("failed to create team in transaction", zap.Error(err), zap.String("team_name", team.TeamName))
		return nil, err
	}

	s.logger.Info("team created successfully", zap.String("team_name", team.TeamName), zap.Int("members", len(team.Members)))

	// Возвращаем созданную команду
	return s.teamRepo.Get(ctx, team.TeamName)
}

// GetTeam получает команду с участниками
func (s *TeamService) GetTeam(ctx context.Context, teamName string) (*domain.Team, error) {
	team, err := s.teamRepo.Get(ctx, teamName)
	if err != nil {
		s.logger.Error("failed to get team", zap.Error(err), zap.String("team_name", teamName))
		return nil, err
	}

	return team, nil
}
