package httpx

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Code          string            `json:"code"`
	Message       string            `json:"message"`
	CorrelationID string            `json:"correlation_id"`
	Retryable     bool              `json:"retryable"`
	FieldErrors   map[string]string `json:"field_errors,omitempty"`
}

func WriteProblem(w http.ResponseWriter, status int, problem Problem) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("X-Correlation-ID", problem.CorrelationID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
