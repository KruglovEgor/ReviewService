package handler

import (
	"net/http"

	"reviewservice/internal/domain"
	"reviewservice/internal/service"
	"go.uber.org/zap"
)

// TeamHandler обрабатывает HTTP запросы для работы с командами
type TeamHandler struct {
	teamService  *service.TeamService
	statsService *service.StatsService
	logger       *zap.Logger
}

// NewTeamHandler создаёт новый экземпляр TeamHandler
func NewTeamHandler(
	teamService *service.TeamService,
	statsService *service.StatsService,
	logger *zap.Logger,
) *TeamHandler {
	return &TeamHandler{
		teamService:  teamService,
		statsService: statsService,
		logger:       logger,
	}
}

// CreateTeam обрабатывает POST /team/add
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req domain.Team
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	// Валидация
	if req.TeamName == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	team, err := h.teamService.CreateTeam(r.Context(), &req)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	response := map[string]interface{}{
		"team": team,
	}

	writeJSON(w, http.StatusCreated, response)
}

// GetTeam обрабатывает GET /team/get
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	team, err := h.teamService.GetTeam(r.Context(), teamName)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// BulkDeactivateTeam обрабатывает POST /team/deactivate
func (h *TeamHandler) BulkDeactivateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamName string `json:"team_name"`
	}

	if err := decodeJSON(r, &req); err != nil {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	if req.TeamName == "" {
		writeError(w, h.logger, http.StatusBadRequest, domain.ErrInvalidInput, domain.CodeNotFound)
		return
	}

	result, err := h.statsService.BulkDeactivateTeam(r.Context(), req.TeamName)
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
