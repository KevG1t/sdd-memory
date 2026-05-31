package cloud

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const ProjectContextKey contextKey = "project"

// RequireAuth middleware verifies the Authorization header.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("SDD_CLOUD_SECRET")
		if secret == "" {
			// If not configured, we might deny all or we might log a warning.
			// Let's assume strict: if not configured, no access.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token != secret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// RequireProject middleware verifies the project is allowed.
// It assumes the project is passed as a URL query parameter or path parameter.
// Let's assume it's passed as a query parameter `?project=xxx` for simplicity,
// or we can extract it. The specification says "requested project".
// For POST /sync/push and GET /sync/pull, project will be a query parameter or path segment.
func RequireProject(db *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("project")
		if project == "" {
			http.Error(w, "Project is required", http.StatusBadRequest)
			return
		}

		allowedProjects := os.Getenv("SDD_ALLOWED_PROJECTS")
		isAllowed := false

		// Check environment variable
		if allowedProjects != "" {
			projects := strings.Split(allowedProjects, ",")
			for _, p := range projects {
				if strings.TrimSpace(p) == project {
					isAllowed = true
					break
				}
			}
		}

		// Check database if not explicitly allowed in env
		if !isAllowed {
			var isActive bool
			err := db.QueryRow("SELECT is_active FROM cloud_project_controls WHERE project = $1", project).Scan(&isActive)
			if err != nil {
				if err == sql.ErrNoRows {
					http.Error(w, "Forbidden: Project not allowed", http.StatusForbidden)
				} else {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}

			if !isActive {
				http.Error(w, "Forbidden: Project inactive", http.StatusForbidden)
				return
			}
		}

		// Add project to context
		ctx := context.WithValue(r.Context(), ProjectContextKey, project)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
