package cloud

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type PushPayload struct {
	Mutations []MutationPayload `json:"mutations"`
}

type MutationPayload struct {
	ChunkID   string          `json:"chunk_id"`
	Operation string          `json:"operation"`
	Data      json.RawMessage `json:"data"` // Only for upsert
}

type PullResponse struct {
	Mutations []MutationResponse `json:"mutations"`
}

type MutationResponse struct {
	ID        int64           `json:"id"`
	ChunkID   string          `json:"chunk_id"`
	Operation string          `json:"operation"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// handlePush implements POST /sync/push
func (s *Server) handlePush() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		project, ok := r.Context().Value(ProjectContextKey).(string)
		if !ok || project == "" {
			RespondError(w, http.StatusInternalServerError, "Project context missing")
			return
		}

		var payload PushPayload
		if err := DecodeJSONBody(w, r, &payload); err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			log.Printf("Push DB begin error: %v", err)
			handleDBError(w, err)
			return
		}
		defer tx.Rollback()

		for _, mut := range payload.Mutations {
			if mut.Operation == "upsert" {
				// We expect Data to be provided for upsert
				if len(mut.Data) == 0 {
					RespondError(w, http.StatusBadRequest, fmt.Sprintf("Data required for upsert operation on chunk %s", mut.ChunkID))
					return
				}
				_, err := tx.ExecContext(ctx, `
					INSERT INTO cloud_chunks (id, project, data, updated_at)
					VALUES ($1, $2, $3, NOW())
					ON CONFLICT (id) DO UPDATE SET
						data = EXCLUDED.data,
						updated_at = EXCLUDED.updated_at
				`, mut.ChunkID, project, []byte(mut.Data))
				if err != nil {
					log.Printf("Push chunk upsert error (chunk=%s): %v", mut.ChunkID, err)
					RespondError(w, http.StatusConflict, fmt.Sprintf("Conflict updating chunk %s", mut.ChunkID))
					return
				}
			} else if mut.Operation == "delete" {
				_, err := tx.ExecContext(ctx, `
					DELETE FROM cloud_chunks WHERE id = $1 AND project = $2
				`, mut.ChunkID, project)
				if err != nil {
					log.Printf("Push chunk delete error (chunk=%s): %v", mut.ChunkID, err)
					RespondError(w, http.StatusConflict, fmt.Sprintf("Conflict deleting chunk %s", mut.ChunkID))
					return
				}
			} else {
				RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid operation: %s", mut.Operation))
				return
			}

			// Log the mutation
			_, err = tx.ExecContext(ctx, `
				INSERT INTO cloud_mutations (project, chunk_id, operation, created_at)
				VALUES ($1, $2, $3, NOW())
			`, project, mut.ChunkID, mut.Operation)
			if err != nil {
				log.Printf("Push mutation log error: %v", err)
				RespondError(w, http.StatusConflict, fmt.Sprintf("Conflict logging mutation for chunk %s", mut.ChunkID))
				return
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("Push commit error: %v", err)
			handleDBError(w, err)
			return
		}

		RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handlePull implements GET /sync/pull
func (s *Server) handlePull() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			RespondError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		project, ok := r.Context().Value(ProjectContextKey).(string)
		if !ok || project == "" {
			RespondError(w, http.StatusInternalServerError, "Project context missing")
			return
		}

		lastIDStr := r.URL.Query().Get("last_mutation_id")
		var lastID int64
		if lastIDStr != "" {
			var err error
			lastID, err = strconv.ParseInt(lastIDStr, 10, 64)
			if err != nil {
				RespondError(w, http.StatusBadRequest, "Invalid last_mutation_id")
				return
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		rows, err := s.db.QueryContext(ctx, `
			SELECT m.id, m.chunk_id, m.operation, c.data
			FROM cloud_mutations m
			LEFT JOIN cloud_chunks c ON m.chunk_id = c.id
			WHERE m.project = $1 AND m.id > $2
			ORDER BY m.id ASC
		`, project, lastID)
		if err != nil {
			log.Printf("Pull query error: %v", err)
			handleDBError(w, err)
			return
		}
		defer rows.Close()

		var mutations []MutationResponse
		for rows.Next() {
			var mut MutationResponse
			var data sql.NullString
			if err := rows.Scan(&mut.ID, &mut.ChunkID, &mut.Operation, &data); err != nil {
				log.Printf("Pull row scan error: %v", err)
				RespondError(w, http.StatusInternalServerError, "Error reading database")
				return
			}
			if data.Valid {
				mut.Data = json.RawMessage(data.String)
			}
			mutations = append(mutations, mut)
		}

		if err := rows.Err(); err != nil {
			log.Printf("Pull rows error: %v", err)
			handleDBError(w, err)
			return
		}

		if mutations == nil {
			mutations = make([]MutationResponse, 0)
		}

		RespondJSON(w, http.StatusOK, PullResponse{Mutations: mutations})
	}
}

func handleDBError(w http.ResponseWriter, err error) {
	if err == context.DeadlineExceeded || err == context.Canceled {
		RespondError(w, http.StatusGatewayTimeout, "Database timeout")
	} else {
		RespondError(w, http.StatusInternalServerError, "Database error")
	}
}
