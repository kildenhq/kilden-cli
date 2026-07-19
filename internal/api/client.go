// Package api is a thin, typed client over the panel's management API
// (/api/v1). It carries the bearer token and refreshes it transparently when
// it is about to expire.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kildenhq/kilden-cli/internal/auth"
	"github.com/kildenhq/kilden-cli/internal/config"
)

// Client talks to one panel as one signed-in user.
type Client struct {
	creds *config.Credentials
	http  *http.Client
}

// New builds a client around stored credentials.
func New(creds *config.Credentials) *Client {
	return &Client{creds: creds, http: &http.Client{Timeout: 30 * time.Second}}
}

// ensureToken refreshes the access token when it is expired or within 30s of
// it, persisting the new token so the next invocation starts fresh.
func (c *Client) ensureToken(ctx context.Context) error {
	if time.Now().Before(c.creds.Expiry.Add(-30 * time.Second)) {
		return nil
	}
	if c.creds.RefreshToken == "" {
		return config.ErrNotLoggedIn
	}
	if err := auth.Refresh(ctx, c.creds); err != nil {
		return err
	}
	return c.creds.Save()
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if err := c.ensureToken(ctx); err != nil {
		return err
	}

	endpoint := c.creds.Host + "/api/v1" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("the session was rejected — run `kd login` again")
	}
	if resp.StatusCode >= 400 {
		return apiError(resp.StatusCode, data)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func apiError(status int, data []byte) error {
	var e struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(data, &e)
	switch {
	case e.Message != "":
		return fmt.Errorf("%s (HTTP %d)", e.Message, status)
	case e.Error != "":
		return fmt.Errorf("%s (HTTP %d)", e.Error, status)
	default:
		return fmt.Errorf("request failed (HTTP %d)", status)
	}
}

// --- endpoints ---

// Whoami returns the signed-in account and its current context.
func (c *Client) Whoami(ctx context.Context) (*User, error) {
	var u User
	return &u, c.do(ctx, http.MethodGet, "/user", nil, nil, &u)
}

// Teams lists the user's teams.
func (c *Client) Teams(ctx context.Context) ([]Team, error) {
	var env listEnvelope[Team]
	return env.Data, c.do(ctx, http.MethodGet, "/teams", nil, nil, &env)
}

// Context returns the active team + project.
func (c *Client) Context(ctx context.Context) (*Context, error) {
	var out Context
	return &out, c.do(ctx, http.MethodGet, "/context", nil, nil, &out)
}

// SwitchTeam sets the active team.
func (c *Client) SwitchTeam(ctx context.Context, teamID int) (*Context, error) {
	var out Context
	return &out, c.do(ctx, http.MethodPut, "/context", nil, map[string]any{"team_id": teamID}, &out)
}

// SwitchProject sets the active project (and pulls its team along).
func (c *Client) SwitchProject(ctx context.Context, projectID string) (*Context, error) {
	var out Context
	return &out, c.do(ctx, http.MethodPut, "/context", nil, map[string]any{"project_id": projectID}, &out)
}

// Projects lists the projects of a team.
func (c *Client) Projects(ctx context.Context, teamID int) ([]Project, error) {
	var env listEnvelope[Project]
	return env.Data, c.do(ctx, http.MethodGet, "/teams/"+strconv.Itoa(teamID)+"/projects", nil, nil, &env)
}

// Project returns one project.
func (c *Client) Project(ctx context.Context, projectID string) (*Project, error) {
	var p Project
	return &p, c.do(ctx, http.MethodGet, "/projects/"+projectID, nil, nil, &p)
}

// CreateProject creates a project inside a team.
func (c *Client) CreateProject(ctx context.Context, teamID int, name, timezone string) (*Project, error) {
	var p Project
	body := map[string]any{"name": name, "timezone": timezone}
	return &p, c.do(ctx, http.MethodPost, "/teams/"+strconv.Itoa(teamID)+"/projects", nil, body, &p)
}

// Keys lists a project's active write keys (secret keys are masked).
func (c *Client) Keys(ctx context.Context, projectID string) ([]APIKey, error) {
	var env listEnvelope[APIKey]
	return env.Data, c.do(ctx, http.MethodGet, "/projects/"+projectID+"/api-keys", nil, nil, &env)
}

// CreateKey mints a write key. The full value is only ever returned here.
func (c *Client) CreateKey(ctx context.Context, projectID, kind, label string) (*APIKey, error) {
	var k APIKey
	body := map[string]any{"kind": kind, "label": label}
	return &k, c.do(ctx, http.MethodPost, "/projects/"+projectID+"/api-keys", nil, body, &k)
}

// Events returns recent events for a project, newest first.
func (c *Client) Events(ctx context.Context, projectID string, filters url.Values) (*EventsPage, error) {
	var page EventsPage
	return &page, c.do(ctx, http.MethodGet, "/projects/"+projectID+"/events", filters, nil, &page)
}

// Catalog returns the observed event names and property keys for a project.
func (c *Client) Catalog(ctx context.Context, projectID string) (*Catalog, error) {
	var cat Catalog
	return &cat, c.do(ctx, http.MethodGet, "/projects/"+projectID+"/properties", nil, nil, &cat)
}
