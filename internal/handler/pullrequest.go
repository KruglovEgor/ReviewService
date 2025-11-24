package handler

import (
	"net/http"

	"github.com/KruglovEgor/ReviewService/internal/domain"
	"github.com/KruglovEgor/ReviewService/internal/service"
	"go.uber.org/zap"
)

// PullRequestHandler обрабатывает HTTP запросы для работы с Pull Request'ами
type PullRequestHandler struct {
	prService *service.PullRequestService
	logger    *zap.Logger
}

// NewPullRequestHandler создаёт новый экземпляр PullRequestHandler
func NewPullRequestHandler(prService *service.PullRequestService, logger *zap.Logger) *PullRequestHandler {
	return &PullRequestHandler{
		prService: prService,
		logger:    logger,
	}
}

// CreatePullRequest обрабатывает POST /pullRequest/create
func (h *PullRequestHandler) CreatePullRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	// Валидация
	if req.PullRequestID == "" || req.PullRequestName == "" || req.AuthorID == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	pr, err := h.prService.CreatePullRequest(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}
	
	response := map[string]interface{}{
		"pr": pr,
	}
	
	writeJSON(w, http.StatusCreated, response)
}

// MergePullRequest обрабатывает POST /pullRequest/merge
func (h *PullRequestHandler) MergePullRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}
	
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	// Валидация
	if req.PullRequestID == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	pr, err := h.prService.MergePullRequest(r.Context(), req.PullRequestID)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}
	
	response := map[string]interface{}{
		"pr": pr,
	}
	
	writeJSON(w, http.StatusOK, response)
}

// ReassignReviewer обрабатывает POST /pullRequest/reassign
func (h *PullRequestHandler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	// Валидация
	if req.PullRequestID == "" || req.OldUserID == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}
	
	pr, newReviewerID, err := h.prService.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}
	
	response := map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	}
	
	writeJSON(w, http.StatusOK, response)
}
