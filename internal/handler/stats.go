package handler

import (
	"net/http"

	"reviewservice/internal/service"
	"go.uber.org/zap"
)

// StatsHandler обрабатывает HTTP запросы для статистики
type StatsHandler struct {
	statsService *service.StatsService
	logger       *zap.Logger
}

// NewStatsHandler создаёт новый экземпляр StatsHandler
func NewStatsHandler(statsService *service.StatsService, logger *zap.Logger) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
		logger:       logger,
	}
}

// GetStats обрабатывает GET /stats
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.statsService.GetStats(r.Context())
	if err != nil {
		handleDomainError(w, h.logger, err)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
