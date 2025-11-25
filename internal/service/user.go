package service

import (
	"context"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"go.uber.org/zap"
)

// UserService реализует бизнес-логику для работы с пользователями
type UserService struct {
	userRepo domain.UserRepository
	logger   *zap.Logger
}

// NewUserService создаёт новый экземпляр UserService
func NewUserService(
	userRepo domain.UserRepository,
	logger *zap.Logger,
) *UserService {
	return &UserService{
		userRepo: userRepo,
		logger:   logger,
	}
}

// SetIsActive устанавливает флаг активности пользователя
func (s *UserService) SetIsActive(ctx context.Context, userID string, isActive bool) (*domain.User, error) {
	// Проверяем существование пользователя
	user, err := s.userRepo.Get(ctx, userID)
	if err != nil {
		s.logger.Error("failed to get user", zap.Error(err), zap.String("user_id", userID))
		return nil, err
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

// GetUser получает пользователя по ID
func (s *UserService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.Get(ctx, userID)
}
