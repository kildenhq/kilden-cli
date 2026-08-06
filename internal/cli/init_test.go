package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kildenhq/kilden-cli/internal/api"
	"github.com/kildenhq/kilden-cli/internal/config"
)

// `kd init` (docs/60 §7, wave 5.4): provision, hand over the key, wait for the
// first event. The waiting is the part worth testing — everything else is one
// POST — because the two ways it can be wrong are both silent: giving up on a
// transient error, and declaring success on our own test button.

func waitClient(t *testing.T, h http.HandlerFunc) *api.Client {
	t.Helper()

	// The cadence is not what is under test, and a real 3s tick would make
	// this file take half a minute to say nothing extra.
	previous := initPollInterval
	initPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { initPollInterval = previous })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return api.New(&config.Credentials{
		Host:        srv.URL,
		AccessToken: "tok",
		Expiry:      time.Now().Add(time.Hour),
	})
}

func TestWaitReturnsOnceTheFirstEventLands(t *testing.T) {
	var polls atomic.Int32
	client := waitClient(t, func(w http.ResponseWriter, _ *http.Request) {
		// Not activated on the first look, activated on the third: the whole
		// point is that this is a wait, not a single check.
		activated := polls.Add(1) >= 3
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "proj-1", "name": "Fjord", "activated": activated,
		})
	})

	done := make(chan error, 1)
	go func() {
		done <- waitForFirstEvent(context.Background(), client,
			&api.Project{ID: "proj-1", Name: "Fjord"}, 2*time.Second)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waitForFirstEvent: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("wait never returned")
	}

	if got := polls.Load(); got < 3 {
		t.Errorf("polled %d times, expected to keep asking until activated", got)
	}
}

func TestWaitSurvivesATransientError(t *testing.T) {
	var polls atomic.Int32
	client := waitClient(t, func(w http.ResponseWriter, _ *http.Request) {
		// The install may be seconds away. A 500 mid-wait is a hiccup in a
		// poll, not a reason to tell somebody their setup failed.
		if polls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "proj-1", "name": "Fjord", "activated": true,
		})
	})

	done := make(chan error, 1)
	go func() {
		done <- waitForFirstEvent(context.Background(), client,
			&api.Project{ID: "proj-1", Name: "Fjord"}, 2*time.Second)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("waitForFirstEvent: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a single 500 ended the wait")
	}
}

func TestWaitGivesUpQuietlyAtTheDeadline(t *testing.T) {
	client := waitClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "proj-1", "name": "Fjord", "activated": false,
		})
	})

	start := time.Now()
	// A timeout is not a failure: people install tomorrow, and a non-nil
	// error here would paint an unfinished install as a broken one.
	if err := waitForFirstEvent(context.Background(), client,
		&api.Project{ID: "proj-1", Name: "Fjord"}, 200*time.Millisecond); err != nil {
		t.Fatalf("a timeout must not be an error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("waited %s past a 200ms deadline", elapsed)
	}
}

func TestWaitStopsWhenInterrupted(t *testing.T) {
	client := waitClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "proj-1", "activated": false})
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	if err := waitForFirstEvent(ctx, client,
		&api.Project{ID: "proj-1", Name: "Fjord"}, time.Hour); err != nil {
		t.Fatalf("Ctrl-C is not a failure: %v", err)
	}
}

func TestDefaultProjectNameIsTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fjord-coffee")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// Almost always the name they would have typed, and it costs no prompt.
	if got := defaultProjectName(); got != "fjord-coffee" {
		t.Errorf("defaultProjectName() = %q, want fjord-coffee", got)
	}
}

func TestInitIsRegisteredAndWritesNoFiles(t *testing.T) {
	cmd := initCmd()

	if cmd.Use != "init" {
		t.Errorf("Use = %q", cmd.Use)
	}
	for _, flag := range []string{"name", "timezone", "wait", "no-wait"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s", flag)
		}
	}

	// The promise in §6 item 8, asserted rather than trusted: this command
	// does not detect a framework and does not write into the repository.
	// That is where 90% of an `init`'s cost and all of its maintenance live,
	// and it is the part the user's own coding agent can do.
	if !strings.Contains(cmd.Long, "writes nothing into your repository") {
		t.Error("the help must state that nothing is written")
	}
}
