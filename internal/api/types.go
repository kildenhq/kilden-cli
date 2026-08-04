package api

import "encoding/json"

// Write-key kinds (non-negotiable #12): public keys identify the project in
// the browser; secret keys are backend-only.
const (
	KindPublic = "public"
	KindSecret = "secret"
)

// Team mirrors the panel's UserTeam DTO (camelCase keys, straight from
// response()->json).
type Team struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	IsPersonal bool   `json:"isPersonal"`
	Role       string `json:"role"`
	RoleLabel  string `json:"roleLabel"`
	IsCurrent  bool   `json:"isCurrent"`
}

// Project carries the summary fields the API returns. current_project on the
// user/context payloads only fills ID and Name; the project endpoints fill the
// rest.
type Project struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	TeamID                   int    `json:"team_id"`
	Timezone                 string `json:"timezone"`
	IdentityVerificationMode string `json:"identity_verification_mode"`
	PublicKey                string `json:"public_key"`
	CreatedAt                string `json:"created_at"`
}

// User is the whoami payload.
type User struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	CurrentTeam    *Team    `json:"current_team"`
	CurrentProject *Project `json:"current_project"`
}

// Context is the active team + project pair.
type Context struct {
	Team    *Team    `json:"team"`
	Project *Project `json:"project"`
}

// APIKey is a project write key. Key is masked on list, full on create.
type APIKey struct {
	ID        int    `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
}

// IdentitySecret is a project's HS256 identity-verification secret. Secret is
// only present on create (shown once); listings omit it. RevokedAt is empty
// while the secret is active.
type IdentitySecret struct {
	ID        int    `json:"id"`
	Kid       string `json:"kid"`
	Secret    string `json:"secret"`
	CreatedAt string `json:"created_at"`
	RevokedAt string `json:"revoked_at"`
}

// LivetailTicket gates the live-tail websocket handshake. The ticket is a
// short-lived (≤60s) HS256 JWT; url is the websocket endpoint to dial.
type LivetailTicket struct {
	Ticket    string `json:"ticket"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}

// LiveEvent is one frame from the live-tail websocket (one event per frame).
type LiveEvent struct {
	UUID       string          `json:"uuid"`
	Event      string          `json:"event"`
	DistinctID string          `json:"distinct_id"`
	PersonID   string          `json:"person_id"`
	Timestamp  string          `json:"timestamp"`
	Verified   bool            `json:"verified"`
	Source     string          `json:"source"`
	Properties json.RawMessage `json:"properties"`
}

// Event is one row from the events explorer.
type Event struct {
	UUID             string `json:"uuid"`
	Event            string `json:"event"`
	DistinctID       string `json:"distinct_id"`
	PersonID         string `json:"person_id"`
	Properties       string `json:"properties"`
	Verified         int    `json:"verified"`
	VerificationMode string `json:"verification_mode"`
	Timestamp        string `json:"timestamp"`
}

// EventsPage is the paginated events response.
type EventsPage struct {
	Events  []Event `json:"events"`
	Page    int     `json:"page"`
	PerPage int     `json:"perPage"`
	HasMore bool    `json:"hasMore"`
}

// Catalog is the observed event names and property keys for a project.
type Catalog struct {
	Events []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"events"`
	Properties []struct {
		Key   string `json:"key"`
		Count int    `json:"count"`
	} `json:"properties"`
}

type listEnvelope[T any] struct {
	Data []T `json:"data"`
}

// Insight is a saved insight managed as config-as-code, addressed by a stable
// slug so `kd apply` is idempotent. Config is the raw per-type parameter map
// (schema-less on the wire; the panel validates its shape). UpdatedAt is only
// populated on reads and is dropped from exported specs (yaml:"-").
type Insight struct {
	Slug      string         `json:"slug" yaml:"slug"`
	Type      string         `json:"type" yaml:"type"`
	Name      string         `json:"name" yaml:"name"`
	Config    map[string]any `json:"config" yaml:"config"`
	UpdatedAt string         `json:"updated_at,omitempty" yaml:"-"`
}

// --- config-as-code resources (docs/55–59) ---
//
// Every one is addressed by a stable identity the caller chooses — `slug`
// where the table gained one, `key` where it already had it — so `kd apply` is
// idempotent. UpdatedAt is only populated on reads and is dropped from
// exported specs (yaml:"-").

// Cohort is a materialized audience (docs/55). MembersCount and
// LastMaterializedAt are read-only.
type Cohort struct {
	Slug               string         `json:"slug" yaml:"slug"`
	Name               string         `json:"name" yaml:"name"`
	Definition         map[string]any `json:"definition" yaml:"definition"`
	MembersCount       int            `json:"members_count,omitempty" yaml:"-"`
	LastMaterializedAt string         `json:"last_materialized_at,omitempty" yaml:"-"`
	UpdatedAt          string         `json:"updated_at,omitempty" yaml:"-"`
}

// Flag is a feature flag (docs/56), addressed by the key the table already
// had. Variants nil means a boolean flag.
type Flag struct {
	Key               string           `json:"key" yaml:"key"`
	Name              string           `json:"name" yaml:"name"`
	Active            bool             `json:"active" yaml:"active"`
	RolloutPercentage int              `json:"rollout_percentage" yaml:"rollout_percentage"`
	Filters           map[string]any   `json:"filters" yaml:"filters,omitempty"`
	Variants          []map[string]any `json:"variants,omitempty" yaml:"variants,omitempty"`
	UpdatedAt         string           `json:"updated_at,omitempty" yaml:"-"`
}

// Unit is an in-app unit or tour (docs/58). Type is honoured on create only:
// content is typed BY type, so changing it in place is refused with 409.
type Unit struct {
	Slug              string         `json:"slug" yaml:"slug"`
	Type              string         `json:"type" yaml:"type"`
	Name              string         `json:"name" yaml:"name"`
	Status            string         `json:"status" yaml:"status"`
	Content           map[string]any `json:"content" yaml:"content"`
	Targeting         map[string]any `json:"targeting" yaml:"targeting,omitempty"`
	RolloutPercentage int            `json:"rollout_percentage" yaml:"rollout_percentage"`
	Display           map[string]any `json:"display" yaml:"display,omitempty"`
	StartsAt          string         `json:"starts_at,omitempty" yaml:"starts_at,omitempty"`
	EndsAt            string         `json:"ends_at,omitempty" yaml:"ends_at,omitempty"`
	UpdatedAt         string         `json:"updated_at,omitempty" yaml:"-"`
}

// CampaignNode is one step of a campaign's flow. Key is its stable identity
// WITHIN the campaign: the engine's node id is derived from it (uuid v5), so
// renaming a key really is a different node — and re-applying an unchanged
// document leaves every in-flight journey exactly where it was (docs/57 §2).
type CampaignNode struct {
	Key    string         `json:"key" yaml:"key"`
	Type   string         `json:"type" yaml:"type"`
	Config map[string]any `json:"config" yaml:"config"`
}

// CampaignEdge wires two nodes by key. Outcome defaults to "next"; branch arms
// are yes/no and split arms carry their variant ("split:<key>").
type CampaignEdge struct {
	From    string `json:"from" yaml:"from"`
	To      string `json:"to" yaml:"to"`
	Outcome string `json:"outcome,omitempty" yaml:"outcome,omitempty"`
}

// Campaign is a messaging flow (docs/57). Cohort names a cohort by ITS slug,
// never a uuid, so one document applies to more than one project.
type Campaign struct {
	Slug                    string         `json:"slug" yaml:"slug"`
	Name                    string         `json:"name" yaml:"name"`
	Status                  string         `json:"status" yaml:"status"`
	Cohort                  string         `json:"cohort,omitempty" yaml:"cohort,omitempty"`
	ExitEvent               string         `json:"exit_event,omitempty" yaml:"exit_event,omitempty"`
	Reentry                 string         `json:"reentry" yaml:"reentry"`
	ReentryDays             *int           `json:"reentry_days,omitempty" yaml:"reentry_days,omitempty"`
	FrequencyCap            *int           `json:"frequency_cap,omitempty" yaml:"frequency_cap,omitempty"`
	FrequencyCapWindowHours *int           `json:"frequency_cap_window_hours,omitempty" yaml:"frequency_cap_window_hours,omitempty"`
	FromName                string         `json:"from_name,omitempty" yaml:"from_name,omitempty"`
	ReplyToEmail            string         `json:"reply_to_email,omitempty" yaml:"reply_to_email,omitempty"`
	QuietHours              map[string]any `json:"quiet_hours,omitempty" yaml:"quiet_hours,omitempty"`
	Nodes                   []CampaignNode `json:"nodes" yaml:"nodes"`
	Edges                   []CampaignEdge `json:"edges" yaml:"edges"`
	UpdatedAt               string         `json:"updated_at,omitempty" yaml:"-"`
}

// Experiment is a 1:1 lens over a multivariate flag (docs/59), which it names
// by the flag's key. A document can take it as far as `stopped`: concluding it
// and promoting a winner stay in the panel.
type Experiment struct {
	Key                     string           `json:"key" yaml:"key"`
	Flag                    string           `json:"flag" yaml:"flag"`
	Name                    string           `json:"name" yaml:"name"`
	Status                  string           `json:"status" yaml:"status"`
	Hypothesis              string           `json:"hypothesis,omitempty" yaml:"hypothesis,omitempty"`
	ControlVariant          string           `json:"control_variant" yaml:"control_variant"`
	AttributionWindowDays   int              `json:"attribution_window_days" yaml:"attribution_window_days"`
	MinimumDetectableEffect *float64         `json:"minimum_detectable_effect,omitempty" yaml:"minimum_detectable_effect,omitempty"`
	PrimaryMetric           map[string]any   `json:"primary_metric" yaml:"primary_metric"`
	SecondaryMetrics        []map[string]any `json:"secondary_metrics,omitempty" yaml:"secondary_metrics,omitempty"`
	GuardrailMetrics        []map[string]any `json:"guardrail_metrics,omitempty" yaml:"guardrail_metrics,omitempty"`
	UpdatedAt               string           `json:"updated_at,omitempty" yaml:"-"`
}
