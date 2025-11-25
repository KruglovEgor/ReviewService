package handler

import (
	"encoding/json"
	"net/http"

	"reviewservice/internal/domain"
	"go.uber.org/zap"
)

// ErrorResponse представляет структуру ошибки API
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail содержит детали ошибки
type ErrorDetail struct {
	Code    domain.ErrorCode `json:"code"`
	Message string           `json:"message"`
}

// writeJSON записывает JSON ответ
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// Если не можем закодировать ответ, логируем ошибку
			// но не можем отправить ответ клиенту, т.к. headers уже отправлены
		}
	}
}

// writeError записывает ошибку в формате API
func writeError(w http.ResponseWriter, logger *zap.Logger, statusCode int, err error, code domain.ErrorCode) {
	logger.Error("request error",
		zap.Error(err),
		zap.Int("status_code", statusCode),
		zap.String("error_code", string(code)))

	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: err.Error(),
		},
	}

	writeJSON(w, statusCode, response)
}

// handleDomainError обрабатывает доменные ошибки и возвращает соответствующий HTTP статус
func handleDomainError(w http.ResponseWriter, logger *zap.Logger, err error) {
	code := domain.MapErrorToCode(err)

	switch code {
	case domain.CodeTeamExists:
		writeError(w, logger, http.StatusBadRequest, err, code)
	case domain.CodePRExists, domain.CodePRMerged, domain.CodeNotAssigned, domain.CodeNoCandidate:
		writeError(w, logger, http.StatusConflict, err, code)
	case domain.CodeNotFound:
		writeError(w, logger, http.StatusNotFound, err, code)
	default:
		// Для неизвестных ошибок возвращаем 500 Internal Server Error
		writeError(w, logger, http.StatusInternalServerError, err, code)
	}
}

// decodeJSON декодирует JSON из request body
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
