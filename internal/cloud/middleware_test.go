package cloud

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRequireAuth(t *testing.T) {
	// Setup
	os.Setenv("SDD_CLOUD_SECRET", "test-secret")
	defer os.Unsetenv("SDD_CLOUD_SECRET")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(nextHandler)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"Valid Token", "Bearer test-secret", http.StatusOK},
		{"Missing Header", "", http.StatusUnauthorized},
		{"Invalid Token", "Bearer wrong-secret", http.StatusUnauthorized},
		{"Malformed Header", "Basic test-secret", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantStatus)
			}
		})
	}
}

func TestRequireProject_EnvAllowed(t *testing.T) {
	// Setup
	os.Setenv("SDD_ALLOWED_PROJECTS", "project1,project2")
	defer os.Unsetenv("SDD_ALLOWED_PROJECTS")

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.Context().Value(ProjectContextKey)
		if project != "project1" {
			t.Errorf("expected project1 in context, got %v", project)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireProject(nil, nextHandler) // DB is nil, but we shouldn't hit it if in env

	req, _ := http.NewRequest("GET", "/?project=project1", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestRequireProject_MissingProject(t *testing.T) {
	handler := RequireProject(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}
