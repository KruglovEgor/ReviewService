package service

import (
	"context"
	"math/rand/v2"

	"reviewservice/internal/domain"

	"go.uber.org/zap"
)

// UserService реализует бизнес-логику для работы с пользователями
type UserService struct {
	userRepo domain.UserRepository
	prRepo   domain.PullRequestRepository
	logger   *zap.Logger
}

// NewUserService создаёт новый экземпляр UserService
func NewUserService(
	userRepo domain.UserRepository,
	prRepo domain.PullRequestRepository,
	logger *zap.Logger,
) *UserService {
	return &UserService{
		userRepo: userRepo,
		prRepo:   prRepo,
		logger:   logger,
	}
}

// SetIsActive устанавливает флаг активности пользователя
// При деактивации (isActive=false) переназначает все открытые PR пользователя
// на активных членов его команды
func (s *UserService) SetIsActive(ctx context.Context, userID string, isActive bool) (*domain.User, error) {
	// Проверяем существование пользователя
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}

	// Если деактивируем пользователя, нужно переназначить его открытые PR
	if !isActive && user.IsActive {
		if err := s.reassignUserPRs(ctx, userID, user.TeamName); err != nil {
			s.logger.Error("failed to reassign user PRs", zap.Error(err), zap.String("user_id", userID))
			// Не прерываем деактивацию, но логируем ошибку
		}
	}

	// Обновляем статус
	if err := s.userRepo.SetIsActive(ctx, userID, isActive); err != nil {
		s.logger.Error("failed to set user active status", zap.Error(err), zap.String("user_id", userID), zap.Bool("is_active", isActive))
		return nil, err
	}

	user.IsActive = isActive

	s.logger.Info("user active status updated", zap.String("user_id", userID), zap.Bool("is_active", isActive))

	return user, nil
}

// reassignUserPRs переназначает все открытые PR деактивируемого пользователя
// на активных членов его команды
func (s *UserService) reassignUserPRs(ctx context.Context, userID string, teamName string) error {
	// Получаем все открытые PR где пользователь - ревьювер
	openPRs, err := s.prRepo.GetOpenByReviewer(ctx, userID)
	if err != nil {
		return err
	}

	if len(openPRs) == 0 {
		return nil
	}

	s.logger.Info("found open PRs for deactivated user",
		zap.String("user_id", userID),
		zap.Int("count", len(openPRs)))

	// Получаем членов команды для поиска замены
	teamMembers, err := s.userRepo.GetByTeam(ctx, teamName)
	if err != nil {
		return err
	}

	// Для каждого PR пытаемся переназначить ревьювера
	for _, prID := range openPRs {
		// Получаем PR для информации об авторе
		pr, err := s.prRepo.Get(ctx, prID)
		if err != nil {
			s.logger.Error("failed to get PR", zap.Error(err), zap.String("pr_id", prID))
			continue
		}

		// Получаем текущих ревьюверов
		currentReviewers, err := s.prRepo.GetReviewers(ctx, prID)
		if err != nil {
			s.logger.Error("failed to get reviewers", zap.Error(err), zap.String("pr_id", prID))
			continue
		}

		// Шаг 1: Ищем кандидатов в команде деактивируемого пользователя
		candidates := s.filterReassignCandidates(teamMembers, pr.AuthorID, currentReviewers, userID)

		// Шаг 2: Если не нашли, ищем в команде автора PR (если это другая команда)
		if len(candidates) == 0 {
			author, err := s.userRepo.Get(ctx, pr.AuthorID)
			if err != nil {
				s.logger.Error("failed to get author", zap.Error(err), zap.String("author_id", pr.AuthorID))
			} else if author.TeamName != teamName {
				s.logger.Info("no candidates in reviewer team, searching in author team",
					zap.String("pr_id", prID),
					zap.String("reviewer_team", teamName),
					zap.String("author_team", author.TeamName))

				authorTeamMembers, err := s.userRepo.GetByTeam(ctx, author.TeamName)
				if err != nil {
					s.logger.Error("failed to get author team members", zap.Error(err))
				} else {
					candidates = s.filterReassignCandidates(authorTeamMembers, pr.AuthorID, currentReviewers, userID)
				}
			}
		}

		// Шаг 3: Если всё ещё не нашли, ищем в других командах
		if len(candidates) == 0 {
			// Получаем автора для определения его команды (чтобы исключить её)
			author, err := s.userRepo.Get(ctx, pr.AuthorID)
			excludeTeams := teamName // команда деактивируемого
			if err == nil && author.TeamName != teamName {
				// Исключаем и команду автора, т.к. там уже искали
				s.logger.Info("no candidates in author team, searching in other teams",
					zap.String("pr_id", prID),
					zap.String("exclude_teams", teamName+","+author.TeamName))
			} else {
				s.logger.Info("no candidates in team, searching in other teams",
					zap.String("pr_id", prID),
					zap.String("exclude_team", excludeTeams))
			}

			otherUsers, err := s.userRepo.GetActiveUsersExcludingTeam(ctx, excludeTeams)
			if err != nil {
				s.logger.Error("failed to get users from other teams", zap.Error(err))
			} else {
				candidates = s.filterReassignCandidates(otherUsers, pr.AuthorID, currentReviewers, userID)
			}
		}

		if len(candidates) == 0 {
			s.logger.Warn("no candidates for reassignment, removing reviewer",
				zap.String("pr_id", prID),
				zap.String("user_id", userID))

			// Просто удаляем ревьювера без замены
			if err := s.prRepo.RemoveReviewer(ctx, prID, userID); err != nil {
				s.logger.Error("failed to remove reviewer", zap.Error(err), zap.String("pr_id", prID))
			}
			continue
		}

		// Выбираем случайного кандидата
		newReviewer := selectRandomReassignCandidate(candidates)

		// Переназначаем
		if err := s.prRepo.ReassignReviewer(ctx, prID, userID, newReviewer); err != nil {
			s.logger.Error("failed to reassign reviewer",
				zap.Error(err),
				zap.String("pr_id", prID),
				zap.String("old", userID),
				zap.String("new", newReviewer))
			continue
		}

		s.logger.Info("reviewer reassigned",
			zap.String("pr_id", prID),
			zap.String("old_reviewer", userID),
			zap.String("new_reviewer", newReviewer))
	}

	return nil
}

// filterReassignCandidates фильтрует кандидатов для замены ревьювера
func (s *UserService) filterReassignCandidates(teamMembers []domain.User, authorID string, currentReviewers []string, excludeUserID string) []string {
	excluded := make(map[string]bool)
	excluded[authorID] = true
	excluded[excludeUserID] = true
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

// selectRandomReassignCandidate выбирает случайного кандидата
func selectRandomReassignCandidate(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	idx := rand.IntN(len(candidates))
	return candidates[idx]
}

// GetUser получает пользователя по ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.Get(ctx, userID)
}
