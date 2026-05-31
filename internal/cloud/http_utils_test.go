package cloud

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeJSONBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"Valid JSON", `{"test":"value"}`, http.StatusOK},
		{"Empty Body", "", http.StatusBadRequest},
		{"Malformed JSON", `{"test":"value"`, http.StatusBadRequest},
		{"Unknown Field", `{"unknown":"value"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/", bytes.NewBufferString(tt.body))
			rr := httptest.NewRecorder()

			var dst struct {
				Test string `json:"test"`
			}
			err := DecodeJSONBody(rr, req, &dst)

			if tt.wantStatus == http.StatusOK {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if rr.Code != tt.wantStatus {
					t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, tt.wantStatus)
				}
			}
		})
	}
}
