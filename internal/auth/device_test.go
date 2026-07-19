package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kildenhq/kilden-cli/internal/config"
)

func TestRequestDeviceCodeAndPoll(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/device/code":
			if err := r.ParseForm(); err != nil || r.Form.Get("client_id") != "cid" {
				t.Errorf("unexpected device-code request: %v / %q", err, r.Form.Get("client_id"))
			}
			_, _ = w.Write([]byte(`{"device_code":"dc","user_code":"WXYZ-1234","verification_uri":"http://x/oauth/device","verification_uri_complete":"http://x/oauth/device?user_code=WXYZ-1234","expires_in":600,"interval":1}`))
		case "/oauth/token":
			polls++
			if polls < 2 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":3600,"scope":"account:read"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	dc, err := RequestDeviceCode(ctx, srv.URL, "cid", "account:read")
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.UserCode != "WXYZ-1234" || dc.Interval != 1 {
		t.Fatalf("unexpected device code: %+v", dc)
	}

	creds, err := PollForToken(ctx, srv.URL, "cid", dc)
	if err != nil {
		t.Fatalf("PollForToken: %v", err)
	}
	if creds.AccessToken != "at" || creds.RefreshToken != "rt" {
		t.Fatalf("unexpected creds: %+v", creds)
	}
	if polls != 2 {
		t.Fatalf("expected the pending poll to be retried, got %d polls", polls)
	}
}

func TestRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("unexpected refresh request: %v / %q", err, r.Form.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"rt2","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	c := &config.Credentials{Host: srv.URL, ClientID: "cid", RefreshToken: "rt"}
	if err := Refresh(context.Background(), c); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if c.AccessToken != "new" || c.RefreshToken != "rt2" {
		t.Fatalf("refresh did not update tokens: %+v", c)
	}
}
