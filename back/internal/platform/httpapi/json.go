package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type ErrorPayload struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

func ReadJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}

	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if decoder.More() {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteErrorWithDetails(w, r, status, code, message, nil)
}

func WriteErrorWithDetails(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	WriteJSON(w, status, ErrorPayload{
		Error: APIError{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: RequestIDFromContext(r.Context()),
		},
	})
}
