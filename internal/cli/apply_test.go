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

func TestParseApplyDocEveryCollection(t *testing.T) {
	raw := []byte(`
cohorts:
  - slug: power-users
    name: Power users
    definition:
      conditions:
        - type: behavior
          event: checkout_completed
          count_gte: 3
          days: 30
flags:
  - key: checkout-v2
    name: New checkout
    active: true
    rollout_percentage: 50
    variants:
      - key: control
        rollout_percentage: 50
      - key: treatment
        rollout_percentage: 50
units:
  - slug: onboarding-tour
    type: tour
    name: First-run tour
    content:
      steps:
        - id: sidebar
          target: "[data-kilden-tour=sidebar]"
          body: Everything lives here.
campaigns:
  - slug: welcome-series
    name: Welcome series
    reentry: never
    nodes:
      - key: signup-trigger
        type: trigger
        config: {mode: event, event: signup}
      - key: welcome-email
        type: email
        config: {subject: Welcome, body_template: "<p>Hi</p>"}
    edges:
      - from: signup-trigger
        to: welcome-email
experiments:
  - key: checkout-test
    flag: checkout-v2
    name: Checkout conversion
    control_variant: control
    attribution_window_days: 14
    primary_metric: {kind: conversion, event: purchase_completed}
`)
	doc, err := parseApplyDoc(raw)
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if len(doc.Cohorts) != 1 || doc.Cohorts[0].Slug != "power-users" {
		t.Errorf("cohorts = %+v", doc.Cohorts)
	}
	if len(doc.Flags) != 1 || len(doc.Flags[0].Variants) != 2 {
		t.Errorf("flags = %+v", doc.Flags)
	}
	if len(doc.Units) != 1 || doc.Units[0].Type != "tour" {
		t.Errorf("units = %+v", doc.Units)
	}
	if len(doc.Campaigns) != 1 || len(doc.Campaigns[0].Nodes) != 2 {
		t.Errorf("campaigns = %+v", doc.Campaigns)
	}
	if doc.Campaigns[0].Edges[0].From != "signup-trigger" {
		t.Errorf("edge = %+v", doc.Campaigns[0].Edges[0])
	}
	if len(doc.Experiments) != 1 || doc.Experiments[0].Flag != "checkout-v2" {
		t.Errorf("experiments = %+v", doc.Experiments)
	}
	if err := validateDoc(doc); err != nil {
		t.Errorf("validateDoc: %v", err)
	}
}

func TestValidateDocRejectsMissingIdentity(t *testing.T) {
	doc, err := parseApplyDoc([]byte("cohorts:\n  - name: No slug\n"))
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if err := validateDoc(doc); err == nil {
		t.Fatal("want an error for a cohort with no slug")
	}
}

func TestValidateDocRejectsDuplicateIdentity(t *testing.T) {
	doc, err := parseApplyDoc([]byte("flags:\n  - key: a\n    name: One\n  - key: a\n    name: Two\n"))
	if err != nil {
		t.Fatalf("parseApplyDoc: %v", err)
	}
	if err := validateDoc(doc); err == nil {
		t.Fatal("want an error for two flags claiming one key")
	}
}

func TestValidateDocRejectsAnEmptySpec(t *testing.T) {
	if err := validateDoc(applyDoc{}); err == nil {
		t.Fatal("want an error for a spec with nothing in it")
	}
}

// The order is a DEPENDENCY order: a campaign can name a cohort and an
// experiment names a flag, so applying alphabetically would 422 on a document
// that is perfectly valid.
func TestApplyOrderPutsDependenciesFirst(t *testing.T) {
	position := map[string]int{}
	for i, r := range resources {
		position[r.name] = i
	}
	if position["cohort"] > position["campaign"] {
		t.Error("cohorts must be applied before campaigns")
	}
	if position["flag"] > position["experiment"] {
		t.Error("flags must be applied before experiments")
	}
}
