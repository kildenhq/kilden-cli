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
