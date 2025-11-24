package service

import (
	"context"
	"fmt"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"go.uber.org/zap"
)

// TeamService реализует бизнес-логику для работы с командами
type TeamService struct {
	teamRepo domain.TeamRepository
	userRepo domain.UserRepository
	logger   *zap.Logger
}

// NewTeamService создаёт новый экземпляр TeamService
func NewTeamService(
	teamRepo domain.TeamRepository,
	userRepo domain.UserRepository,
	logger *zap.Logger,
) *TeamService {
	return &TeamService{
		teamRepo: teamRepo,
		userRepo: userRepo,
		logger:   logger,
	}
}

// CreateTeam создаёт команду и добавляет/обновляет её участников
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

	// Создаём команду
	if err := s.teamRepo.Create(ctx, team); err != nil {
		s.logger.Error("failed to create team", zap.Error(err), zap.String("team_name", team.TeamName))
		return nil, err
	}

	s.logger.Info("team created", zap.String("team_name", team.TeamName))

	// Создаём/обновляем пользователей
	for _, member := range team.Members {
		user := &domain.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}

		// Пытаемся получить пользователя
		existingUser, err := s.userRepo.Get(ctx, user.UserID)
		if err != nil {
			if err == domain.ErrNotFound {
				// Пользователь не существует, создаём
				if err := s.userRepo.Create(ctx, user); err != nil {
					s.logger.Error("failed to create user", zap.Error(err), zap.String("user_id", user.UserID))
					return nil, fmt.Errorf("failed to create user %s: %w", user.UserID, err)
				}
				s.logger.Info("user created", zap.String("user_id", user.UserID), zap.String("team_name", team.TeamName))
			} else {
				s.logger.Error("failed to get user", zap.Error(err), zap.String("user_id", user.UserID))
				return nil, fmt.Errorf("failed to get user %s: %w", user.UserID, err)
			}
		} else {
			// Пользователь существует, обновляем
			existingUser.Username = user.Username
			existingUser.TeamName = user.TeamName
			existingUser.IsActive = user.IsActive

			if err := s.userRepo.Update(ctx, existingUser); err != nil {
				s.logger.Error("failed to update user", zap.Error(err), zap.String("user_id", user.UserID))
				return nil, fmt.Errorf("failed to update user %s: %w", user.UserID, err)
			}
			s.logger.Info("user updated", zap.String("user_id", user.UserID), zap.String("team_name", team.TeamName))
		}
	}

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
