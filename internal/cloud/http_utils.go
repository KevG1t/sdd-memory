package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrorResponse represents a standard error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// RespondError sends a standard JSON error response.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// RespondJSON sends a standard JSON response.
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// DecodeJSONBody parses the JSON request body and returns standard errors.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Body == nil {
		msg := "Request body must not be empty"
		RespondError(w, http.StatusBadRequest, msg)
		return errors.New(msg)
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Strict validation

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			RespondError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := "Request body contains badly-formed JSON"
			RespondError(w, http.StatusBadRequest, msg)

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			RespondError(w, http.StatusBadRequest, msg)

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			RespondError(w, http.StatusBadRequest, msg)

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			RespondError(w, http.StatusBadRequest, msg)

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			RespondError(w, http.StatusRequestEntityTooLarge, msg)

		default:
			RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		}
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		RespondError(w, http.StatusBadRequest, msg)
		return errors.New(msg)
	}

	return nil
}
