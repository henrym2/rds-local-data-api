package api

import (
	"encoding/json"
	"net/http"
)

// APIError is a structured HTTP error that carries a status code, AWS error
// code, and message. Returning an *APIError from a handler method causes
// handle[T] to write the correct HTTP response automatically.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

func badRequest(msg string) *APIError {
	return &APIError{http.StatusBadRequest, "BadRequestException", msg}
}

func notFound(msg string) *APIError {
	return &APIError{http.StatusNotFound, "NotFoundException", msg}
}

func internalError(msg string) *APIError {
	return &APIError{http.StatusInternalServerError, "InternalServerErrorException", msg}
}

// ErrorResponse is the JSON body returned for all error responses.
type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// writeJSON writes v as a JSON response with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured AWS-compatible error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{Message: message, Code: code})
}
