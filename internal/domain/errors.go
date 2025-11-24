package domain

import "errors"

// Доменные ошибки для бизнес-логики
var (
	// ErrTeamExists - команда с таким именем уже существует
	ErrTeamExists = errors.New("team already exists")
	
	// ErrPRExists - PR с таким ID уже существует
	ErrPRExists = errors.New("pull request already exists")
	
	// ErrPRMerged - попытка изменить уже смердженный PR
	ErrPRMerged = errors.New("cannot modify merged pull request")
	
	// ErrNotAssigned - ревьювер не назначен на этот PR
	ErrNotAssigned = errors.New("reviewer is not assigned to this PR")
	
	// ErrNoCandidate - нет доступных кандидатов для назначения
	ErrNoCandidate = errors.New("no active replacement candidate in team")
	
	// ErrNotFound - ресурс не найден
	ErrNotFound = errors.New("resource not found")
	
	// ErrInvalidInput - некорректные входные данные
	ErrInvalidInput = errors.New("invalid input data")
)

// ErrorCode представляет код ошибки API
type ErrorCode string

const (
	CodeTeamExists  ErrorCode = "TEAM_EXISTS"
	CodePRExists    ErrorCode = "PR_EXISTS"
	CodePRMerged    ErrorCode = "PR_MERGED"
	CodeNotAssigned ErrorCode = "NOT_ASSIGNED"
	CodeNoCandidate ErrorCode = "NO_CANDIDATE"
	CodeNotFound    ErrorCode = "NOT_FOUND"
)

// MapErrorToCode преобразует доменную ошибку в код API
func MapErrorToCode(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrTeamExists):
		return CodeTeamExists
	case errors.Is(err, ErrPRExists):
		return CodePRExists
	case errors.Is(err, ErrPRMerged):
		return CodePRMerged
	case errors.Is(err, ErrNotAssigned):
		return CodeNotAssigned
	case errors.Is(err, ErrNoCandidate):
		return CodeNoCandidate
	case errors.Is(err, ErrNotFound):
		return CodeNotFound
	default:
		return CodeNotFound
	}
}
