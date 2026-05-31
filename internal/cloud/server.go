package cloud

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Server represents the cloud sync server.
type Server struct {
	db *sql.DB
}

// NewServer initializes a new cloud sync server.
func NewServer(db *sql.DB) *Server {
	return &Server{
		db: db,
	}
}

// Start runs the HTTP server.
func (s *Server) Start(port string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth())

	// Sync routes require authentication and project authorization
	syncMux := http.NewServeMux()
	syncMux.HandleFunc("/sync/push", s.handlePush())
	syncMux.HandleFunc("/sync/pull", s.handlePull())

	// Route all /sync/ traffic through the middleware chain
	mux.Handle("/sync/", RequireAuth(RequireProject(s.db, syncMux.ServeHTTP)))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Cloud server listening on port %s", port)
	return srv.ListenAndServe()
}

// handleHealth returns a 200 OK if the DB is reachable, else 503.
func (s *Server) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := s.db.Ping(); err != nil {
			log.Printf("Health check failed (DB unreachable): %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Database unavailable\n")
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK\n")
	}
}
