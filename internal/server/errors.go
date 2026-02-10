package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ReilBleem13/MessangerV2/internal/domain"
)

type ErrorResponse struct {
	Error ErrorInfo `json:"error"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, err *domain.AppError) {
	response := ErrorResponse{
		Error: ErrorInfo{
			Code:    err.Code,
			Message: err.Message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Status)
	json.NewEncoder(w).Encode(response)
}

func hadleError(w http.ResponseWriter, err error) {
	var appErr *domain.AppError

	if errors.As(err, &appErr) {
		writeError(w, appErr)
		return
	}

	slog.Error("Unhandled error", "error", err)
	writeError(w, domain.ErrInternalServerError)
}
