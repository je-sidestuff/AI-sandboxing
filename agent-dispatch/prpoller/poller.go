// Package prpoller provides a centralized batch poller for GitHub PR comments
// using the GraphQL API. It monitors multiple PRs efficiently with a single
// query and dispatches change notifications when new comments are detected.
package prpoller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// CommentInfo holds information about a PR comment
type CommentInfo struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PRState holds the current state of a monitored PR
type PRState struct {
	Owner         string
	Repo          string
	Number        int
	LastCommentID string
	CommentCount  int
	Comments      []CommentInfo
	LastChecked   time.Time
}

// ChangeEvent represents a detected change in a PR
type ChangeEvent struct {
	PR          PRState
	NewComments []CommentInfo
	Timestamp   time.Time
}

// TerraformAction holds information about terraform to re-apply
type TerraformAction struct {
	WorkDir     string
	TFVarsFile  string
	Description string
}

// PRRegistration holds the registration info for a PR to monitor
type PRRegistration struct {
	Owner           string
	Repo            string
	Number          int
	TerraformAction *TerraformAction // optional - if set, re-apply on changes
	OnChange        func(ChangeEvent) // callback when changes detected
}

// Poller is the centralized batch poller for PR comments
type Poller struct {
	mu            sync.RWMutex
	registrations map[string]*PRRegistration
	states        map[string]*PRState
	interval      time.Duration
	client        *http.Client
	token         string
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	onChange      func(ChangeEvent) // global change handler
}

// Config holds configuration for the Poller
type Config struct {
	Interval  time.Duration
	Token     string // GitHub token
	OnChange  func(ChangeEvent)
}

// NewPoller creates a new PR comment poller
func NewPoller(cfg Config) *Poller {
	if cfg.Interval == 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Token == "" {
		cfg.Token = os.Getenv("GITHUB_TOKEN")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Poller{
		registrations: make(map[string]*PRRegistration),
		states:        make(map[string]*PRState),
		interval:      cfg.Interval,
		client:        &http.Client{Timeout: 30 * time.Second},
		token:         cfg.Token,
		ctx:           ctx,
		cancel:        cancel,
		onChange:      cfg.OnChange,
	}
}

// prKey generates a unique key for a PR
func prKey(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

// Register adds a PR to be monitored
func (p *Poller) Register(reg PRRegistration) {
	key := prKey(reg.Owner, reg.Repo, reg.Number)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.registrations[key] = &reg
}

// Unregister removes a PR from monitoring
func (p *Poller) Unregister(owner, repo string, number int) {
	key := prKey(owner, repo, number)
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.registrations, key)
	delete(p.states, key)
}

// Start begins the polling loop
func (p *Poller) Start() {
	p.wg.Add(1)
	go p.pollLoop()
}

// Stop gracefully stops the poller
func (p *Poller) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (p *Poller) pollLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Initial poll
	p.pollAll()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollAll()
		}
	}
}

func (p *Poller) pollAll() {
	p.mu.RLock()
	regs := make([]*PRRegistration, 0, len(p.registrations))
	for _, reg := range p.registrations {
		regs = append(regs, reg)
	}
	p.mu.RUnlock()

	if len(regs) == 0 {
		return
	}

	// Batch fetch comments for all registered PRs
	results := p.batchFetchComments(regs)

	// Process results and detect changes
	for key, comments := range results {
		p.mu.Lock()
		reg := p.registrations[key]
		if reg == nil {
			p.mu.Unlock()
			continue
		}

		oldState := p.states[key]
		newState := &PRState{
			Owner:        reg.Owner,
			Repo:         reg.Repo,
			Number:       reg.Number,
			CommentCount: len(comments),
			Comments:     comments,
			LastChecked:  time.Now(),
		}
		if len(comments) > 0 {
			newState.LastCommentID = comments[len(comments)-1].ID
		}

		// Detect new comments
		var newComments []CommentInfo
		if oldState != nil {
			newComments = findNewComments(oldState.Comments, comments)
		}

		p.states[key] = newState
		p.mu.Unlock()

		// Fire change event if there are new comments
		if len(newComments) > 0 {
			event := ChangeEvent{
				PR:          *newState,
				NewComments: newComments,
				Timestamp:   time.Now(),
			}

			// Call registration-specific handler
			if reg.OnChange != nil {
				reg.OnChange(event)
			}

			// Call global handler
			if p.onChange != nil {
				p.onChange(event)
			}

			// Trigger terraform if configured
			if reg.TerraformAction != nil {
				go p.triggerTerraform(reg.TerraformAction, event)
			}
		}
	}
}

// findNewComments returns comments in 'current' that aren't in 'old'
func findNewComments(old, current []CommentInfo) []CommentInfo {
	oldIDs := make(map[string]bool)
	for _, c := range old {
		oldIDs[c.ID] = true
	}

	var newComments []CommentInfo
	for _, c := range current {
		if !oldIDs[c.ID] {
			newComments = append(newComments, c)
		}
	}
	return newComments
}

// batchFetchComments fetches comments for multiple PRs using GraphQL
func (p *Poller) batchFetchComments(regs []*PRRegistration) map[string][]CommentInfo {
	results := make(map[string][]CommentInfo)

	// GraphQL query to fetch PR comments
	// We build a query with aliases for each PR
	query := buildBatchQuery(regs)
	if query == "" {
		return results
	}

	reqBody := map[string]interface{}{
		"query": query,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return results
	}

	req, err := http.NewRequestWithContext(p.ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(bodyBytes))
	if err != nil {
		return results
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return results
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return results
	}

	// Parse response
	var gqlResp graphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return results
	}

	// Extract comments for each PR
	for i, reg := range regs {
		alias := fmt.Sprintf("pr%d", i)
		key := prKey(reg.Owner, reg.Repo, reg.Number)

		prData, ok := gqlResp.Data[alias]
		if !ok {
			continue
		}

		comments := extractComments(prData)
		results[key] = comments
	}

	return results
}

type graphQLResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type prDataResponse struct {
	PullRequest struct {
		Comments struct {
			Nodes []struct {
				ID        string `json:"id"`
				Author    struct {
					Login string `json:"login"`
				} `json:"author"`
				Body      string `json:"body"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
			} `json:"nodes"`
		} `json:"comments"`
	} `json:"pullRequest"`
}

func extractComments(raw json.RawMessage) []CommentInfo {
	var prData prDataResponse
	if err := json.Unmarshal(raw, &prData); err != nil {
		return nil
	}

	var comments []CommentInfo
	for _, node := range prData.PullRequest.Comments.Nodes {
		createdAt, _ := time.Parse(time.RFC3339, node.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, node.UpdatedAt)
		comments = append(comments, CommentInfo{
			ID:        node.ID,
			Author:    node.Author.Login,
			Body:      node.Body,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}
	return comments
}

func buildBatchQuery(regs []*PRRegistration) string {
	if len(regs) == 0 {
		return ""
	}

	var buf bytes.Buffer
	buf.WriteString("query {\n")

	for i, reg := range regs {
		// Use an alias for each PR to allow multiple in one query
		fmt.Fprintf(&buf, `  pr%d: repository(owner: "%s", name: "%s") {
    pullRequest(number: %d) {
      comments(first: 100, orderBy: {field: UPDATED_AT, direction: ASC}) {
        nodes {
          id
          author { login }
          body
          createdAt
          updatedAt
        }
      }
    }
  }
`, i, reg.Owner, reg.Repo, reg.Number)
	}

	buf.WriteString("}\n")
	return buf.String()
}

// triggerTerraform runs terraform apply for the given action
func (p *Poller) triggerTerraform(action *TerraformAction, event ChangeEvent) {
	// Build terraform command
	args := []string{"apply", "-auto-approve"}
	if action.TFVarsFile != "" {
		args = append(args, "-var-file="+action.TFVarsFile)
	}

	cmd := exec.CommandContext(p.ctx, "terraform", args...)
	cmd.Dir = action.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("[prpoller] Triggering terraform apply for %s/%s#%d: %s\n",
		event.PR.Owner, event.PR.Repo, event.PR.Number, action.Description)

	if err := cmd.Run(); err != nil {
		fmt.Printf("[prpoller] Terraform apply failed: %v\n", err)
	} else {
		fmt.Printf("[prpoller] Terraform apply completed successfully\n")
	}
}

// GetState returns the current state of a monitored PR
func (p *Poller) GetState(owner, repo string, number int) *PRState {
	key := prKey(owner, repo, number)
	p.mu.RLock()
	defer p.mu.RUnlock()
	if state := p.states[key]; state != nil {
		stateCopy := *state
		return &stateCopy
	}
	return nil
}

// ListRegistered returns all registered PR keys
func (p *Poller) ListRegistered() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys := make([]string, 0, len(p.registrations))
	for k := range p.registrations {
		keys = append(keys, k)
	}
	return keys
}
