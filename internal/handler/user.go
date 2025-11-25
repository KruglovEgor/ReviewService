package handler

import (
	"net/http"

	"go.uber.org/zap"
	"reviewservice/internal/domain"
	"reviewservice/internal/service"
)

// UserHandler обрабатывает HTTP запросы для работы с пользователями
type UserHandler struct {
	userService *service.UserService
	prService   *service.PullRequestService
	logger      *zap.Logger
}

// NewUserHandler создаёт новый экземпляр UserHandler
func NewUserHandler(
	userService *service.UserService,
	prService *service.PullRequestService,
	logger *zap.Logger,
) *UserHandler {
	return &UserHandler{
		userService: userService,
		prService:   prService,
		logger:      logger,
	}
}

// SetIsActive обрабатывает POST /users/setIsActive
func (h *UserHandler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	// Валидация
	if req.UserID == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	user, err := h.userService.SetIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	response := map[string]interface{}{
		"user": user,
	}

	writeJSON(w, http.StatusOK, response)
}

// GetReview обрабатывает GET /users/getReview
func (h *UserHandler) GetReview(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	reviews, err := h.prService.GetUserReviews(r.Context(), userID)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, reviews)
}
