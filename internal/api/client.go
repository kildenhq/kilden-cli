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

// LivetailTicket mints a short-lived ticket for the live-tail websocket.
func (c *Client) LivetailTicket(ctx context.Context, projectID string) (*LivetailTicket, error) {
	var t LivetailTicket
	return &t, c.do(ctx, http.MethodPost, "/projects/"+projectID+"/livetail-ticket", nil, nil, &t)
}

// IdentitySecrets lists a project's identity-verification secrets. The secret
// value itself is never returned here — only kid, timestamps and revoked state.
func (c *Client) IdentitySecrets(ctx context.Context, projectID string) ([]IdentitySecret, error) {
	var env listEnvelope[IdentitySecret]
	return env.Data, c.do(ctx, http.MethodGet, "/projects/"+projectID+"/identity-secrets", nil, nil, &env)
}

// CreateIdentitySecret mints a secret under kid. The full value is only ever
// returned here.
func (c *Client) CreateIdentitySecret(ctx context.Context, projectID, kid string) (*IdentitySecret, error) {
	var s IdentitySecret
	body := map[string]any{"kid": kid}
	return &s, c.do(ctx, http.MethodPost, "/projects/"+projectID+"/identity-secrets", nil, body, &s)
}

// DisableIdentitySecret revokes a secret by id; the enricher stops accepting
// its kid within one refresh.
func (c *Client) DisableIdentitySecret(ctx context.Context, projectID string, id int) (*IdentitySecret, error) {
	var s IdentitySecret
	path := "/projects/" + projectID + "/identity-secrets/" + strconv.Itoa(id) + "/disable"
	return &s, c.do(ctx, http.MethodPatch, path, nil, nil, &s)
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

// Insights lists a project's config-as-code (slugged) insights. Insights
// created in the panel carry no slug and are not returned here.
func (c *Client) Insights(ctx context.Context, projectID string) ([]Insight, error) {
	var env listEnvelope[Insight]
	// Read the body before returning env.Data: Go only orders function calls
	// left-to-right, not plain field reads, so `return env.Data, c.do(...)`
	// could evaluate env.Data before c.do fills it.
	err := c.do(ctx, http.MethodGet, "/projects/"+projectID+"/insights", nil, nil, &env)
	return env.Data, err
}

// Insight returns one saved insight by slug.
func (c *Client) Insight(ctx context.Context, projectID, slug string) (*Insight, error) {
	var in Insight
	return &in, c.do(ctx, http.MethodGet, "/projects/"+projectID+"/insights/"+url.PathEscape(slug), nil, nil, &in)
}

// ApplyInsight upserts an insight by slug (idempotent create-or-update) and
// returns the stored form.
func (c *Client) ApplyInsight(ctx context.Context, projectID, slug string, spec Insight) (*Insight, error) {
	var in Insight
	body := map[string]any{"type": spec.Type, "name": spec.Name, "config": spec.Config}
	return &in, c.do(ctx, http.MethodPut, "/projects/"+projectID+"/insights/"+url.PathEscape(slug), nil, body, &in)
}

// DeleteInsight removes a saved insight by slug.
func (c *Client) DeleteInsight(ctx context.Context, projectID, slug string) error {
	return c.do(ctx, http.MethodDelete, "/projects/"+projectID+"/insights/"+url.PathEscape(slug), nil, nil, nil)
}

// --- config-as-code (docs/55–59) ---
//
// One shape per resource: list, get one, upsert by its stable identity, delete.
// Every method reads the body into `out` BEFORE returning it: Go only orders
// function calls left-to-right, not plain field reads, so `return env.Data,
// c.do(...)` could evaluate env.Data before c.do fills it.

func (c *Client) configPath(projectID, resource, id string) string {
	path := "/projects/" + projectID + "/" + resource
	if id != "" {
		path += "/" + url.PathEscape(id)
	}
	return path
}

// Cohorts lists a project's config-as-code cohorts. Cohorts created in the
// panel carry no slug and are not returned here.
func (c *Client) Cohorts(ctx context.Context, projectID string) ([]Cohort, error) {
	var env listEnvelope[Cohort]
	err := c.do(ctx, http.MethodGet, c.configPath(projectID, "cohorts", ""), nil, nil, &env)
	return env.Data, err
}

func (c *Client) Cohort(ctx context.Context, projectID, slug string) (*Cohort, error) {
	var out Cohort
	return &out, c.do(ctx, http.MethodGet, c.configPath(projectID, "cohorts", slug), nil, nil, &out)
}

func (c *Client) ApplyCohort(ctx context.Context, projectID string, spec Cohort) (*Cohort, error) {
	var out Cohort
	body := map[string]any{"name": spec.Name, "definition": spec.Definition}
	return &out, c.do(ctx, http.MethodPut, c.configPath(projectID, "cohorts", spec.Slug), nil, body, &out)
}

func (c *Client) DeleteCohort(ctx context.Context, projectID, slug string) error {
	return c.do(ctx, http.MethodDelete, c.configPath(projectID, "cohorts", slug), nil, nil, nil)
}

// MaterializeCohort recomputes membership now instead of waiting out the sweep.
func (c *Client) MaterializeCohort(ctx context.Context, projectID, slug string) (*Cohort, error) {
	var out Cohort
	return &out, c.do(ctx, http.MethodPost, c.configPath(projectID, "cohorts", slug)+"/materialize", nil, nil, &out)
}

// Flags lists a project's feature flags. Unlike the slugged resources this is
// ALL of them: `key` is the table's real identity, so config-as-code and the
// panel address the same rows (docs/56 §3.1).
func (c *Client) Flags(ctx context.Context, projectID string) ([]Flag, error) {
	var env listEnvelope[Flag]
	err := c.do(ctx, http.MethodGet, c.configPath(projectID, "flags", ""), nil, nil, &env)
	return env.Data, err
}

func (c *Client) Flag(ctx context.Context, projectID, key string) (*Flag, error) {
	var out Flag
	return &out, c.do(ctx, http.MethodGet, c.configPath(projectID, "flags", key), nil, nil, &out)
}

func (c *Client) ApplyFlag(ctx context.Context, projectID string, spec Flag) (*Flag, error) {
	var out Flag
	body := map[string]any{
		"name": spec.Name, "active": spec.Active,
		"rollout_percentage": spec.RolloutPercentage,
		"filters":            orEmptyMap(spec.Filters),
	}
	if spec.Variants != nil {
		body["variants"] = spec.Variants
	}
	return &out, c.do(ctx, http.MethodPut, c.configPath(projectID, "flags", spec.Key), nil, body, &out)
}

func (c *Client) DeleteFlag(ctx context.Context, projectID, key string) error {
	return c.do(ctx, http.MethodDelete, c.configPath(projectID, "flags", key), nil, nil, nil)
}

func (c *Client) Units(ctx context.Context, projectID string) ([]Unit, error) {
	var env listEnvelope[Unit]
	err := c.do(ctx, http.MethodGet, c.configPath(projectID, "units", ""), nil, nil, &env)
	return env.Data, err
}

func (c *Client) Unit(ctx context.Context, projectID, slug string) (*Unit, error) {
	var out Unit
	return &out, c.do(ctx, http.MethodGet, c.configPath(projectID, "units", slug), nil, nil, &out)
}

func (c *Client) ApplyUnit(ctx context.Context, projectID string, spec Unit) (*Unit, error) {
	var out Unit
	body := map[string]any{
		"type": spec.Type, "name": spec.Name, "content": spec.Content,
		"targeting":          orEmptyMap(spec.Targeting),
		"rollout_percentage": spec.RolloutPercentage,
		"display":            orEmptyMap(spec.Display),
	}
	if spec.Status != "" {
		body["status"] = spec.Status
	}
	if spec.StartsAt != "" {
		body["starts_at"] = spec.StartsAt
	}
	if spec.EndsAt != "" {
		body["ends_at"] = spec.EndsAt
	}
	return &out, c.do(ctx, http.MethodPut, c.configPath(projectID, "units", spec.Slug), nil, body, &out)
}

func (c *Client) DeleteUnit(ctx context.Context, projectID, slug string) error {
	return c.do(ctx, http.MethodDelete, c.configPath(projectID, "units", slug), nil, nil, nil)
}

func (c *Client) Campaigns(ctx context.Context, projectID string) ([]Campaign, error) {
	var env listEnvelope[Campaign]
	err := c.do(ctx, http.MethodGet, c.configPath(projectID, "campaigns", ""), nil, nil, &env)
	return env.Data, err
}

func (c *Client) Campaign(ctx context.Context, projectID, slug string) (*Campaign, error) {
	var out Campaign
	return &out, c.do(ctx, http.MethodGet, c.configPath(projectID, "campaigns", slug), nil, nil, &out)
}

func (c *Client) ApplyCampaign(ctx context.Context, projectID string, spec Campaign) (*Campaign, error) {
	var out Campaign
	reentry := spec.Reentry
	if reentry == "" {
		reentry = "never"
	}
	body := map[string]any{
		"name": spec.Name, "reentry": reentry,
		"nodes": spec.Nodes, "edges": spec.Edges,
	}
	for key, value := range map[string]string{
		"status": spec.Status, "cohort": spec.Cohort, "exit_event": spec.ExitEvent,
		"from_name": spec.FromName, "reply_to_email": spec.ReplyToEmail,
	} {
		if value != "" {
			body[key] = value
		}
	}
	for key, value := range map[string]*int{
		"reentry_days": spec.ReentryDays, "frequency_cap": spec.FrequencyCap,
		"frequency_cap_window_hours": spec.FrequencyCapWindowHours,
	} {
		if value != nil {
			body[key] = *value
		}
	}
	if spec.QuietHours != nil {
		body["quiet_hours"] = spec.QuietHours
	}
	return &out, c.do(ctx, http.MethodPut, c.configPath(projectID, "campaigns", spec.Slug), nil, body, &out)
}

func (c *Client) DeleteCampaign(ctx context.Context, projectID, slug string) error {
	return c.do(ctx, http.MethodDelete, c.configPath(projectID, "campaigns", slug), nil, nil, nil)
}

func (c *Client) Experiments(ctx context.Context, projectID string) ([]Experiment, error) {
	var env listEnvelope[Experiment]
	err := c.do(ctx, http.MethodGet, c.configPath(projectID, "experiments", ""), nil, nil, &env)
	return env.Data, err
}

func (c *Client) Experiment(ctx context.Context, projectID, key string) (*Experiment, error) {
	var out Experiment
	return &out, c.do(ctx, http.MethodGet, c.configPath(projectID, "experiments", key), nil, nil, &out)
}

func (c *Client) ApplyExperiment(ctx context.Context, projectID string, spec Experiment) (*Experiment, error) {
	var out Experiment
	body := map[string]any{
		"flag": spec.Flag, "name": spec.Name,
		"control_variant":         spec.ControlVariant,
		"attribution_window_days": spec.AttributionWindowDays,
		"primary_metric":          spec.PrimaryMetric,
	}
	if spec.Status != "" {
		body["status"] = spec.Status
	}
	if spec.Hypothesis != "" {
		body["hypothesis"] = spec.Hypothesis
	}
	if spec.MinimumDetectableEffect != nil {
		body["minimum_detectable_effect"] = *spec.MinimumDetectableEffect
	}
	if spec.SecondaryMetrics != nil {
		body["secondary_metrics"] = spec.SecondaryMetrics
	}
	if spec.GuardrailMetrics != nil {
		body["guardrail_metrics"] = spec.GuardrailMetrics
	}
	return &out, c.do(ctx, http.MethodPut, c.configPath(projectID, "experiments", spec.Key), nil, body, &out)
}

func (c *Client) DeleteExperiment(ctx context.Context, projectID, key string) error {
	return c.do(ctx, http.MethodDelete, c.configPath(projectID, "experiments", key), nil, nil, nil)
}

// orEmptyMap keeps `present` fields present: the panel validates `filters` and
// `targeting` with `present`, so a nil map has to travel as {} and not be
// dropped from the JSON body.
func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
