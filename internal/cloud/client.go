package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/KevG1t/sdd-memory/internal/store"
)

type Client struct {
	Endpoint   string
	Token      string
	Project    string
	LocalStore *store.LocalStore
	HTTPClient *http.Client
}

func NewClient(endpoint, token, project string, localStore *store.LocalStore) *Client {
	return &Client{
		Endpoint:   endpoint,
		Token:      token,
		Project:    project,
		LocalStore: localStore,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Sync performs a full push then pull.
func (c *Client) Sync() error {
	log.Println("Starting sync...")

	// 1. Push pending mutations
	if err := c.Push(); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// 2. Pull remote mutations
	if err := c.Pull(); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	log.Println("Sync completed successfully.")
	return nil
}

// Push local mutations to the server.
func (c *Client) Push() error {
	pending, err := c.LocalStore.GetPendingMutations()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		log.Println("No pending local mutations to push.")
		return nil
	}

	payload := PushPayload{
		Mutations: make([]MutationPayload, 0, len(pending)),
	}

	var syncedIDs []int64
	for _, p := range pending {
		// Only observations are pushed
		if p.TableName != "observations" {
			syncedIDs = append(syncedIDs, p.ID) // Ignore non-observations
			continue
		}

		mp := MutationPayload{
			ChunkID:   p.RecordID,
			Operation: p.Operation,
		}
		if p.Payload != nil {
			if strPayload, ok := p.Payload.(string); ok {
				mp.Data = json.RawMessage(strPayload)
			}
		}
		payload.Mutations = append(payload.Mutations, mp)
		syncedIDs = append(syncedIDs, p.ID)
	}

	if len(payload.Mutations) == 0 {
		return c.LocalStore.MarkMutationsSynced(syncedIDs)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/sync/push?project=%s", c.Endpoint, c.Project), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	err = c.doRequestWithBackoff(req, func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return nil
	})

	if err != nil {
		return err
	}

	return c.LocalStore.MarkMutationsSynced(syncedIDs)
}

// Pull remote mutations from the server.
func (c *Client) Pull() error {
	lastID, err := c.LocalStore.GetLastMutationID(c.Project)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/sync/pull?project=%s&last_mutation_id=%d", c.Endpoint, c.Project, lastID), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	var pullResp PullResponse
	err = c.doRequestWithBackoff(req, func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(&pullResp)
	})

	if err != nil {
		return err
	}

	if len(pullResp.Mutations) == 0 {
		log.Println("No new remote mutations to pull.")
		return nil
	}

	for _, m := range pullResp.Mutations {
		if m.Operation == "upsert" {
			var obs store.Observation
			if err := json.Unmarshal(m.Data, &obs); err != nil {
				log.Printf("Failed to unmarshal observation %s: %v", m.ChunkID, err)
				continue
			}
			if err := c.LocalStore.SaveObservationFromSync(&obs); err != nil {
				log.Printf("Failed to save remote observation %s: %v", m.ChunkID, err)
			}
		} else if m.Operation == "delete" {
			if err := c.LocalStore.DeleteObservationFromSync(m.ChunkID); err != nil {
				log.Printf("Failed to delete remote observation %s: %v", m.ChunkID, err)
			}
		}
		
		if m.ID > lastID {
			lastID = m.ID
		}
	}

	return c.LocalStore.SetLastMutationID(c.Project, lastID)
}

func (c *Client) doRequestWithBackoff(req *http.Request, handler func(*http.Response) error) error {
	maxRetries := 3
	backoff := 1 * time.Second

	for i := 0; i <= maxRetries; i++ {
		// Clone request body if we need to retry
		var reqBody []byte
		if req.Body != nil && req.GetBody == nil {
			reqBody, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(reqBody))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(reqBody)), nil
			}
		}

		resp, err := c.HTTPClient.Do(req)
		
		// Determine if we should retry
		isRetryable := false
		if err != nil {
			isRetryable = true // Network error
		} else if resp.StatusCode >= 500 {
			isRetryable = true // 5xx Server Error
			resp.Body.Close()
		} else {
			// Success or 4xx error (not retryable)
			defer resp.Body.Close()
			return handler(resp)
		}

		if i == maxRetries || !isRetryable {
			if err != nil {
				return err
			}
			return fmt.Errorf("request failed after retries")
		}

		log.Printf("Network issue or 5xx error, retrying in %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
	}
	return nil
}
