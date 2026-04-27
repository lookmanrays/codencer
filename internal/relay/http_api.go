package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type apiError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": apiError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type responseRecorder struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{header: make(http.Header), statusCode: http.StatusOK}
}

func (r *responseRecorder) Header() http.Header            { return r.header }
func (r *responseRecorder) Write(data []byte) (int, error) { return r.body.Write(data) }
func (r *responseRecorder) WriteHeader(statusCode int)     { r.statusCode = statusCode }
