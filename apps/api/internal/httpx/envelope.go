package httpx

import (
	"encoding/json"
	"net/http"
)

type Envelope struct {
	OK    bool      `json:"ok"`
	Data  any       `json:"data"`
	Error *APIError `json:"error"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const (
	CodeUnauthenticated     = "UNAUTHENTICATED"
	CodeForbidden           = "FORBIDDEN"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeWindowNotOpen       = "WINDOW_NOT_OPEN"
	CodeWindowPauseTake     = "WINDOW_PAUSE_TAKE"
	CodeHasActiveTicket     = "HAS_ACTIVE_TICKET"
	CodeTicketNotWaiting    = "TICKET_NOT_WAITING"
	CodeTicketNotCalled     = "TICKET_NOT_CALLED"
	CodeNoWaiting           = "NO_WAITING"
	CodeCalledPending       = "CALLED_PENDING"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeDisplayTokenInvalid = "DISPLAY_TOKEN_INVALID"
	CodeInternal            = "INTERNAL_ERROR"
)

func WriteOK(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, Envelope{OK: true, Data: data, Error: nil})
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Envelope{
		OK:   false,
		Data: nil,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func DecodeJSON(r *http.Request, dest any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dest)
}
