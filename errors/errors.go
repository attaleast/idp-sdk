package errors

import (
	"encoding/json"
	"net/http"
)

type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Message }

func New(code, msg string) *AppError { return &AppError{Code: code, Message: msg} }

func WriteHTTPError(w http.ResponseWriter, status int, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
}
