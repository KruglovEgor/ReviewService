package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

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

// UserAssignmentStats представляет статистику назначений пользователя (алиас для domain)
type UserAssignmentStats = domain.UserAssignmentStats

// PRStats представляет общую статистику по PR
type PRStats struct {
	TotalPRs          int     `json:"total_prs"`
	OpenPRs           int     `json:"open_prs"`
	MergedPRs         int     `json:"merged_prs"`
	AvgReviewersPerPR float64 `json:"avg_reviewers_per_pr"`
}

// GlobalStats представляет общую статистику сервиса
type GlobalStats struct {
	PRStats   PRStats                         `json:"pr_stats"`
	UserStats map[string]*UserAssignmentStats `json:"user_stats"`
}

// GetStats возвращает статистику по назначениям ревьюверов
func (s *StatsService) GetStats(ctx context.Context) (*GlobalStats, error) {
	s.logger.Info("calculating assignment statistics")

	// Получаем статистику по PR через репозиторий
	prStats, err := s.prRepo.GetPRStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR stats: %w", err)
	}

	// Получаем статистику по пользователям
	userStatsMap, err := s.prRepo.GetUserAssignmentStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user assignment stats: %w", err)
	}

	// Обогащаем данными о пользователях (username)
	enrichedUserStats := make(map[string]*UserAssignmentStats)
	for userID, stats := range userStatsMap {
		user, err := s.userRepo.Get(ctx, userID)
		if err != nil {
			s.logger.Warn("failed to get user info for stats",
				zap.String("user_id", userID),
				zap.Error(err))
			stats.Username = "unknown"
		} else {
			stats.Username = user.Username
		}
		enrichedUserStats[userID] = stats
	}

	result := &GlobalStats{
		PRStats: PRStats{
			TotalPRs:          prStats["total"],
			OpenPRs:           prStats["open"],
			MergedPRs:         prStats["merged"],
			AvgReviewersPerPR: float64(prStats["avg_reviewers"]) / 100.0, // Делим на 100 обратно
		},
		UserStats: enrichedUserStats,
	}

	s.logger.Info("statistics calculated",
		zap.Int("total_prs", result.PRStats.TotalPRs),
		zap.Int("users_with_assignments", len(enrichedUserStats)))

	return result, nil
}

// BulkDeactivateTeam массово деактивирует пользователей команды
// и переназначает их открытые PR на активных членов команды.
//
// Деактивация пользователей выполняется атомарно одним запросом.
// Переназначение PR происходит последовательно (best-effort) - если часть
// переназначений не удалась, операция продолжается, а ошибки возвращаются
// в результате для анализа.
//
// Оптимизировано для выполнения <100мс на средних объёмах.
func (s *StatsService) BulkDeactivateTeam(ctx context.Context, teamName string) (*BulkDeactivateResult, error) {
	start := time.Now()
	s.logger.Info("bulk deactivating team members", zap.String("team_name", teamName))

	// Получаем всех членов команды (активных и неактивных) одним запросом
	allMembers, err := s.userRepo.GetByTeam(ctx, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	if len(allMembers) == 0 {
		return nil, domain.ErrNotFound
	}

	// Разделяем на активных и неактивных
	var toDeactivate []string
	var activeMembers []string

	for _, member := range allMembers {
		if member.IsActive {
			toDeactivate = append(toDeactivate, member.UserID)
		} else {
			activeMembers = append(activeMembers, member.UserID)
		}
	}

	if len(toDeactivate) == 0 {
		s.logger.Info("no active users to deactivate",
			zap.String("team_name", teamName))
		return &BulkDeactivateResult{
			DeactivatedUsers: []string{},
			ReassignedPRs:    0,
		}, nil
	}

	// Деактивируем пользователей пакетом
	deactivatedIDs, err := s.userRepo.BulkDeactivateByTeam(ctx, teamName)
	if err != nil {
		s.logger.Error("failed to bulk deactivate team", zap.Error(err), zap.String("team_name", teamName))
		return nil, fmt.Errorf("failed to bulk deactivate team: %w", err)
	}

	s.logger.Info("team members deactivated",
		zap.String("team_name", teamName),
		zap.Int("count", len(deactivatedIDs)),
		zap.Strings("user_ids", deactivatedIDs))

	// Собираем все открытые PR деактивированных пользователей
	totalReassigned := 0
	reassignErrors := 0

	for _, userID := range deactivatedIDs {
		openPRs, err := s.prRepo.GetOpenByReviewer(ctx, userID)
		if err != nil {
			s.logger.Error("failed to get open PRs for user",
				zap.Error(err),
				zap.String("user_id", userID))
			reassignErrors++
			continue
		}

		if len(openPRs) == 0 {
			continue
		}

		s.logger.Info("found open PRs for deactivated user",
			zap.String("user_id", userID),
			zap.Int("count", len(openPRs)))

		// Для каждого PR пытаемся переназначить
		for _, prID := range openPRs {
			// Получаем автора PR чтобы исключить его из кандидатов
			pr, err := s.prRepo.Get(ctx, prID)
			if err != nil {
				s.logger.Error("failed to get PR for reassignment",
					zap.Error(err),
					zap.String("pr_id", prID))
				reassignErrors++
				continue
			}

			// Получаем текущих ревьюверов
			currentReviewers, err := s.prRepo.GetReviewers(ctx, prID)
			if err != nil {
				s.logger.Error("failed to get reviewers",
					zap.Error(err),
					zap.String("pr_id", prID))
				reassignErrors++
				continue
			}

			// Ищем кандидата для замены из команды автора
			author, err := s.userRepo.Get(ctx, pr.AuthorID)
			if err != nil {
				s.logger.Error("failed to get author",
					zap.Error(err),
					zap.String("author_id", pr.AuthorID))
				reassignErrors++
				continue
			}

			teamMembers, err := s.userRepo.GetByTeam(ctx, author.TeamName)
			if err != nil {
				s.logger.Error("failed to get team members for reassignment",
					zap.Error(err),
					zap.String("team", author.TeamName))
				reassignErrors++
				continue
			}

			// Фильтруем кандидатов: активные, не автор, не текущие ревьюверы
			candidates := filterCandidates(teamMembers, pr.AuthorID, currentReviewers)

			if len(candidates) == 0 {
				s.logger.Warn("no candidates for reassignment, removing reviewer without replacement",
					zap.String("pr_id", prID),
					zap.String("old_reviewer", userID),
					zap.String("team", author.TeamName),
					zap.Int("team_members_total", len(teamMembers)))

				// Просто удаляем ревьювера без замены, т.к. нет активных кандидатов
				err = s.prRepo.RemoveReviewer(ctx, prID, userID)
				if err != nil {
					s.logger.Error("failed to remove reviewer",
						zap.Error(err),
						zap.String("pr_id", prID),
						zap.String("user_id", userID))
					reassignErrors++
				} else {
					totalReassigned++ // Считаем как успешное "переназначение" (удаление)
					s.logger.Info("reviewer removed (no replacement available)",
						zap.String("pr_id", prID),
						zap.String("removed_reviewer", userID))
				}
				continue
			}

			// Выбираем случайного кандидата
			newReviewer := selectRandomCandidate(candidates)

			// Переназначаем
			err = s.prRepo.ReassignReviewer(ctx, prID, userID, newReviewer)
			if err != nil {
				s.logger.Error("failed to reassign reviewer",
					zap.Error(err),
					zap.String("pr_id", prID),
					zap.String("old", userID),
					zap.String("new", newReviewer))
				reassignErrors++
				continue
			}

			totalReassigned++
			s.logger.Info("reviewer reassigned",
				zap.String("pr_id", prID),
				zap.String("old_reviewer", userID),
				zap.String("new_reviewer", newReviewer))
		}
	}

	elapsed := time.Since(start)
	s.logger.Info("bulk deactivation completed",
		zap.String("team_name", teamName),
		zap.Int("deactivated", len(deactivatedIDs)),
		zap.Int("reassigned_prs", totalReassigned),
		zap.Int("errors", reassignErrors),
		zap.Duration("elapsed", elapsed))

	return &BulkDeactivateResult{
		DeactivatedUsers: deactivatedIDs,
		ReassignedPRs:    totalReassigned,
		Errors:           reassignErrors,
	}, nil
}

// BulkDeactivateResult содержит результаты массовой деактивации
type BulkDeactivateResult struct {
	DeactivatedUsers []string `json:"deactivated_users"`
	ReassignedPRs    int      `json:"reassigned_prs"`
	Errors           int      `json:"errors,omitempty"`
}

// filterCandidates фильтрует кандидатов для замены ревьювера
func filterCandidates(teamMembers []domain.User, authorID string, currentReviewers []string) []string {
	excluded := make(map[string]bool)
	excluded[authorID] = true
	for _, reviewerID := range currentReviewers {
		excluded[reviewerID] = true
	}

	var candidates []string
	for _, member := range teamMembers {
		if member.IsActive && !excluded[member.UserID] {
			candidates = append(candidates, member.UserID)
		}
	}

	return candidates
}

// selectRandomCandidate выбирает случайного кандидата из списка
func selectRandomCandidate(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}

	// Используем crypto/rand для production, но math/rand быстрее для нагрузки
	idx := rand.IntN(len(candidates))
	return candidates[idx]
}
