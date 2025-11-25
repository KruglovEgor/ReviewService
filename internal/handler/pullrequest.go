package handler

import (
	"net/http"

	"go.uber.org/zap"
	"reviewservice/internal/domain"
	"reviewservice/internal/service"
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
		h.logger.Error("failed to decode reassign request", zap.Error(err))
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	h.logger.Debug("reassign request",
		zap.String("pr_id", req.PullRequestID),
		zap.String("old_user_id", req.OldUserID))

	// Валидация
	if req.PullRequestID == "" || req.OldUserID == "" {
		h.logger.Error("empty fields in reassign request",
			zap.String("pr_id", req.PullRequestID),
			zap.String("old_user_id", req.OldUserID))
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

// ListPullRequests обрабатывает GET /pullRequest/list
func (h *PullRequestHandler) ListPullRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // Опционально: OPEN, MERGED или пусто (все)

	// Валидация статуса если указан
	if status != "" && status != string(domain.PRStatusOpen) && status != string(domain.PRStatusMerged) {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	prs, err := h.prService.ListPullRequests(r.Context(), status)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	response := map[string]interface{}{
		"pull_requests": prs,
		"total":         len(prs),
	}

	writeJSON(w, http.StatusOK, response)
}
