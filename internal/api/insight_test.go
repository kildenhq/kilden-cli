package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kildenhq/kilden-cli/internal/config"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(&config.Credentials{Host: srv.URL, AccessToken: "tok", Expiry: time.Now().Add(time.Hour)})
}

func TestApplyInsightUpsertsBySlug(t *testing.T) {
	var method, path, auth string
	var body map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "signup", "type": "funnel", "name": "Signup", "config": body["config"],
		})
	})

	got, err := c.ApplyInsight(context.Background(), "proj-1", "signup", Insight{
		Type: "funnel", Name: "Signup", Config: map[string]any{"steps": []any{"a", "b"}},
	})
	if err != nil {
		t.Fatalf("ApplyInsight: %v", err)
	}
	if method != http.MethodPut {
		t.Errorf("method = %s, want PUT", method)
	}
	if path != "/api/v1/projects/proj-1/insights/signup" {
		t.Errorf("path = %s", path)
	}
	if auth != "Bearer tok" {
		t.Errorf("auth = %q", auth)
	}
	if body["type"] != "funnel" || body["name"] != "Signup" {
		t.Errorf("body = %v", body)
	}
	if got.Slug != "signup" {
		t.Errorf("slug = %s", got.Slug)
	}
}

func TestInsightsListParsesEnvelope(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/p/insights" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"slug": "a", "type": "trend", "name": "A"}},
		})
	})
	got, err := c.Insights(context.Background(), "p")
	if err != nil {
		t.Fatalf("Insights: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "a" {
		t.Errorf("got = %v", got)
	}
}

func TestDeleteInsightSendsDelete(t *testing.T) {
	var method string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_ = json.NewEncoder(w).Encode(map[string]any{"slug": "x", "deleted": true})
	})
	if err := c.DeleteInsight(context.Background(), "p", "x"); err != nil {
		t.Fatalf("DeleteInsight: %v", err)
	}
	if method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", method)
	}
}

func TestInsightErrorSurfacesMessage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "The config.steps field is required."})
	})
	_, err := c.ApplyInsight(context.Background(), "p", "bad", Insight{Type: "funnel", Name: "Bad"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if want := "The config.steps field is required."; !contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
