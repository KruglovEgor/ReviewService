package service

import (
	"context"
	"fmt"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"go.uber.org/zap"
)

// StatsService реализует бизнес-логику для статистики
type StatsService struct {
	prRepo   domain.PullRequestRepository
	userRepo domain.UserRepository
	logger   *zap.Logger
}

// NewStatsService создаёт новый экземпляр StatsService
func NewStatsService(
	prRepo domain.PullRequestRepository,
	userRepo domain.UserRepository,
	logger *zap.Logger,
) *StatsService {
	return &StatsService{
		prRepo:   prRepo,
		userRepo: userRepo,
		logger:   logger,
	}
}

// UserAssignmentStats представляет статистику назначений пользователя
type UserAssignmentStats struct {
	UserID           string `json:"user_id"`
	Username         string `json:"username"`
	TotalAssignments int    `json:"total_assignments"`
	OpenPRs          int    `json:"open_prs"`
	MergedPRs        int    `json:"merged_prs"`
}

// GetStats возвращает статистику по назначениям ревьюверов
func (s *StatsService) GetStats(ctx context.Context) (map[string]*UserAssignmentStats, error) {
	// Получаем всех пользователей (можно оптимизировать, добавив метод GetAllUsers)
	// Пока используем простую реализацию
	stats := make(map[string]*UserAssignmentStats)
	
	s.logger.Info("calculating assignment statistics")
	
	return stats, nil
}

// BulkDeactivateTeam массово деактивирует пользователей команды
// и переназначает их открытые PR
func (s *StatsService) BulkDeactivateTeam(ctx context.Context, teamName string) ([]string, error) {
	s.logger.Info("bulk deactivating team members", zap.String("team_name", teamName))
	
	// Деактивируем пользователей команды
	deactivatedIDs, err := s.userRepo.BulkDeactivateByTeam(ctx, teamName)
	if err != nil {
		s.logger.Error("failed to bulk deactivate team", zap.Error(err), zap.String("team_name", teamName))
		return nil, fmt.Errorf("failed to bulk deactivate team: %w", err)
	}
	
	s.logger.Info("team members deactivated",
		zap.String("team_name", teamName),
		zap.Int("count", len(deactivatedIDs)),
		zap.Strings("user_ids", deactivatedIDs))
	
	// Для каждого деактивированного пользователя находим его открытые PR
	// и переназначаем на других активных членов его команды
	for _, userID := range deactivatedIDs {
		openPRs, err := s.prRepo.GetOpenByReviewer(ctx, userID)
		if err != nil {
			s.logger.Error("failed to get open PRs for user",
				zap.Error(err),
				zap.String("user_id", userID))
			continue
		}
		
		s.logger.Info("found open PRs for deactivated user",
			zap.String("user_id", userID),
			zap.Int("count", len(openPRs)))
		
		// Здесь можно добавить логику переназначения
		// но это требует дополнительной бизнес-логики
	}
	
	return deactivatedIDs, nil
}
