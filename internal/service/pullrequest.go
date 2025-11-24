package service

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"go.uber.org/zap"
)

// PullRequestService реализует бизнес-логику для работы с Pull Request'ами
type PullRequestService struct {
	prRepo   domain.PullRequestRepository
	userRepo domain.UserRepository
	logger   *zap.Logger
}

// NewPullRequestService создаёт новый экземпляр PullRequestService
func NewPullRequestService(
	prRepo domain.PullRequestRepository,
	userRepo domain.UserRepository,
	logger *zap.Logger,
) *PullRequestService {
	return &PullRequestService{
		prRepo:   prRepo,
		userRepo: userRepo,
		logger:   logger,
	}
}

// CreatePullRequest создаёт новый PR и автоматически назначает до 2 ревьюверов
func (s *PullRequestService) CreatePullRequest(
	ctx context.Context,
	prID, prName, authorID string,
) (*domain.PullRequest, error) {
	// Проверяем существование PR
	exists, err := s.prRepo.Exists(ctx, prID)
	if err != nil {
		s.logger.Error("failed to check PR existence", zap.Error(err), zap.String("pr_id", prID))
		return nil, fmt.Errorf("failed to check PR existence: %w", err)
	}
	
	if exists {
		return nil, domain.ErrPRExists
	}
	
	// Получаем автора
	author, err := s.userRepo.Get(ctx, authorID)
	if err != nil {
		s.logger.Error("failed to get author", zap.Error(err), zap.String("author_id", authorID))
		return nil, err
	}
	
	// Создаём PR
	pr := &domain.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            domain.PRStatusOpen,
		AssignedReviewers: []string{},
	}
	
	if err := s.prRepo.Create(ctx, pr); err != nil {
		s.logger.Error("failed to create PR", zap.Error(err), zap.String("pr_id", prID))
		return nil, err
	}
	
	s.logger.Info("PR created", zap.String("pr_id", prID), zap.String("author_id", authorID))
	
	// Получаем команду автора
	teamMembers, err := s.userRepo.GetByTeam(ctx, author.TeamName)
	if err != nil {
		s.logger.Error("failed to get team members", zap.Error(err), zap.String("team_name", author.TeamName))
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	
	// Выбираем до 2 активных ревьюверов (исключаем автора)
	reviewers := s.selectReviewers(teamMembers, authorID, 2)
	
	// Назначаем ревьюверов
	if len(reviewers) > 0 {
		if err := s.prRepo.AssignReviewers(ctx, prID, reviewers); err != nil {
			s.logger.Error("failed to assign reviewers", zap.Error(err), zap.String("pr_id", prID))
			return nil, fmt.Errorf("failed to assign reviewers: %w", err)
		}
		pr.AssignedReviewers = reviewers
		s.logger.Info("reviewers assigned", zap.String("pr_id", prID), zap.Strings("reviewers", reviewers))
	} else {
		s.logger.Warn("no reviewers available", zap.String("pr_id", prID), zap.String("team_name", author.TeamName))
	}
	
	return pr, nil
}

// MergePullRequest помечает PR как смердженный (идемпотентная операция)
func (s *PullRequestService) MergePullRequest(ctx context.Context, prID string) (*domain.PullRequest, error) {
	pr, err := s.prRepo.Merge(ctx, prID)
	if err != nil {
		s.logger.Error("failed to merge PR", zap.Error(err), zap.String("pr_id", prID))
		return nil, err
	}
	
	s.logger.Info("PR merged", zap.String("pr_id", prID))
	
	return pr, nil
}

// ReassignReviewer переназначает ревьювера
func (s *PullRequestService) ReassignReviewer(
	ctx context.Context,
	prID, oldReviewerID string,
) (*domain.PullRequest, string, error) {
	// Получаем PR
	pr, err := s.prRepo.Get(ctx, prID)
	if err != nil {
		s.logger.Error("failed to get PR", zap.Error(err), zap.String("pr_id", prID))
		return nil, "", err
	}
	
	// Проверяем, что PR не смерджен
	if pr.Status == domain.PRStatusMerged {
		return nil, "", domain.ErrPRMerged
	}
	
	// Проверяем, что старый ревьювер назначен
	isAssigned := false
	for _, reviewerID := range pr.AssignedReviewers {
		if reviewerID == oldReviewerID {
			isAssigned = true
			break
		}
	}
	
	if !isAssigned {
		return nil, "", domain.ErrNotAssigned
	}
	
	// Получаем старого ревьювера
	oldReviewer, err := s.userRepo.Get(ctx, oldReviewerID)
	if err != nil {
		s.logger.Error("failed to get old reviewer", zap.Error(err), zap.String("reviewer_id", oldReviewerID))
		return nil, "", err
	}
	
	// Получаем команду старого ревьювера
	teamMembers, err := s.userRepo.GetByTeam(ctx, oldReviewer.TeamName)
	if err != nil {
		s.logger.Error("failed to get team members", zap.Error(err), zap.String("team_name", oldReviewer.TeamName))
		return nil, "", fmt.Errorf("failed to get team members: %w", err)
	}
	
	// Исключаем автора и текущих ревьюверов
	excludedIDs := make(map[string]bool)
	excludedIDs[pr.AuthorID] = true
	for _, reviewerID := range pr.AssignedReviewers {
		excludedIDs[reviewerID] = true
	}
	
	// Выбираем нового ревьювера
	candidates := make([]string, 0)
	for _, member := range teamMembers {
		if member.IsActive && !excludedIDs[member.UserID] {
			candidates = append(candidates, member.UserID)
		}
	}
	
	if len(candidates) == 0 {
		return nil, "", domain.ErrNoCandidate
	}
	
	// Случайно выбираем нового ревьювера
	newReviewerID := candidates[rand.Intn(len(candidates))]
	
	// Переназначаем ревьювера
	if err := s.prRepo.ReassignReviewer(ctx, prID, oldReviewerID, newReviewerID); err != nil {
		s.logger.Error("failed to reassign reviewer", 
			zap.Error(err), 
			zap.String("pr_id", prID),
			zap.String("old_reviewer", oldReviewerID),
			zap.String("new_reviewer", newReviewerID))
		return nil, "", err
	}
	
	s.logger.Info("reviewer reassigned", 
		zap.String("pr_id", prID),
		zap.String("old_reviewer", oldReviewerID),
		zap.String("new_reviewer", newReviewerID))
	
	// Обновляем список ревьюверов в PR
	for i, id := range pr.AssignedReviewers {
		if id == oldReviewerID {
			pr.AssignedReviewers[i] = newReviewerID
			break
		}
	}
	
	return pr, newReviewerID, nil
}

// GetUserReviews получает PR'ы, где пользователь назначен ревьювером
func (s *PullRequestService) GetUserReviews(ctx context.Context, userID string) (*domain.UserPullRequests, error) {
	// Проверяем существование пользователя
	_, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		// Если пользователь не найден, всё равно возвращаем пустой список
		// (согласно спецификации, эндпоинт не должен возвращать 404)
		if err == domain.ErrNotFound {
			return &domain.UserPullRequests{
				UserID:       userID,
				PullRequests: []domain.PullRequestShort{},
			}, nil
		}
		s.logger.Error("failed to get user", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}
	
	// Получаем PR'ы пользователя
	prs, err := s.prRepo.GetByReviewer(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user reviews", zap.Error(err), zap.String("user_id", userID))
		return nil, fmt.Errorf("failed to get user reviews: %w", err)
	}
	
	return &domain.UserPullRequests{
		UserID:       userID,
		PullRequests: prs,
	}, nil
}

// selectReviewers выбирает до maxCount активных ревьюверов из команды (исключая автора)
func (s *PullRequestService) selectReviewers(teamMembers []domain.User, authorID string, maxCount int) []string {
	// Фильтруем активных участников (исключая автора)
	candidates := make([]string, 0)
	for _, member := range teamMembers {
		if member.IsActive && member.UserID != authorID {
			candidates = append(candidates, member.UserID)
		}
	}
	
	// Если кандидатов меньше или равно maxCount, возвращаем всех
	if len(candidates) <= maxCount {
		return candidates
	}
	
	// Случайно выбираем maxCount ревьюверов
	// Используем алгоритм Fisher-Yates для перемешивания
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	
	return candidates[:maxCount]
}
