package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// Envelope wraps all API responses for consistency.
type Envelope struct {
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// JSON writes a JSON response with the given status code and payload.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload) //nolint:errcheck
}

// Success writes a 200 JSON response wrapping data.
func Success(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Envelope{Data: data})
}

// Created writes a 201 JSON response wrapping data.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Envelope{Data: data})
}

// NoContent writes a 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// BadRequest writes a 400 JSON error response.
func BadRequest(w http.ResponseWriter, err error) {
	JSON(w, http.StatusBadRequest, Envelope{Error: err.Error()})
}

// NotFound writes a 404 JSON error response.
func NotFound(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusNotFound, Envelope{Error: msg})
}

// InternalError logs the actual error and returns a generic 500 to prevent
// exposing database internals (table names, query fragments, constraints) to clients.
func InternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	JSON(w, http.StatusInternalServerError, Envelope{Error: "internal server error"})
}
