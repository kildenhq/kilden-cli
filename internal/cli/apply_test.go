package cli

import "testing"

func TestParseApplyDocYAML(t *testing.T) {
	raw := []byte(`
insights:
  - slug: signup-funnel
    type: funnel
    name: Signup funnel
    config:
      steps: ["$pageview", "signup_completed"]
      window_days: 14
`)
	doc, err := parseApplyDoc(raw)
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if len(doc.Insights) != 1 {
		t.Fatalf("insights = %d, want 1", len(doc.Insights))
	}
	in := doc.Insights[0]
	if in.Slug != "signup-funnel" || in.Type != "funnel" || in.Name != "Signup funnel" {
		t.Errorf("insight = %+v", in)
	}
	if in.Config["window_days"] != 14 {
		t.Errorf("window_days = %v", in.Config["window_days"])
	}
}

func TestParseApplyDocJSON(t *testing.T) {
	// YAML is a JSON superset, so the same decoder accepts JSON specs.
	raw := []byte(`{"insights":[{"slug":"s","type":"trend","name":"T","config":{"events":["x"]}}]}`)
	doc, err := parseApplyDoc(raw)
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if len(doc.Insights) != 1 || doc.Insights[0].Slug != "s" {
		t.Errorf("doc = %+v", doc)
	}
}

func TestParseApplyDocAcceptsDocWithDuplicateSlugs(t *testing.T) {
	// Parsing does not dedupe; the apply preflight rejects duplicates. Here we
	// only assert the parse keeps both entries so the preflight has something
	// to catch.
	raw := []byte("insights:\n  - slug: a\n    type: trend\n    name: One\n    config: {events: [x]}\n  - slug: a\n    type: trend\n    name: Two\n    config: {events: [y]}\n")
	doc, err := parseApplyDoc(raw)
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if len(doc.Insights) != 2 {
		t.Fatalf("insights = %d, want 2", len(doc.Insights))
	}
}
