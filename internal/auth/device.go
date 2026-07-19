// Package auth implements the OAuth 2.0 Device Authorization Grant (RFC 8628)
// the CLI uses to sign in, plus refresh-token exchange.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kildenhq/kilden-cli/internal/config"
)

const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// DeviceCode is the panel's response to a device-code request: what to show
// the user and how to poll for approval.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// RequestDeviceCode starts the flow: it asks the panel for a user code and the
// verification URL to send the human to.
func RequestDeviceCode(ctx context.Context, host, clientID, scope string) (*DeviceCode, error) {
	form := url.Values{"client_id": {clientID}, "scope": {scope}}
	tr, status, body, err := postForm(ctx, host+"/oauth/device/code", form)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("could not start sign-in (HTTP %d): %s", status, strings.TrimSpace(string(body)))
	}
	var dc DeviceCode
	if err := json.Unmarshal(body, &dc); err != nil {
		return nil, err
	}
	_ = tr
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	return &dc, nil
}

// PollForToken polls the token endpoint on the interval the panel asked for,
// honoring authorization_pending and slow_down, until the user approves or the
// code expires.
func PollForToken(ctx context.Context, host, clientID string, dc *DeviceCode) (*config.Credentials, error) {
	interval := time.Duration(dc.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the code expired before it was approved — run `kd login` again")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		form := url.Values{
			"grant_type":  {deviceGrantType},
			"device_code": {dc.DeviceCode},
			"client_id":   {clientID},
		}
		tr, status, _, err := postForm(ctx, host+"/oauth/token", form)
		if err != nil {
			return nil, err
		}

		switch {
		case status == http.StatusOK && tr.AccessToken != "":
			return credsFrom(host, clientID, tr), nil
		case tr.Error == "authorization_pending":
			continue
		case tr.Error == "slow_down":
			interval += 5 * time.Second
		case tr.Error == "access_denied":
			return nil, fmt.Errorf("the request was denied in the browser")
		case tr.Error == "expired_token":
			return nil, fmt.Errorf("the code expired before it was approved — run `kd login` again")
		case tr.Error != "":
			if tr.ErrorDesc != "" {
				return nil, fmt.Errorf("%s: %s", tr.Error, tr.ErrorDesc)
			}
			return nil, fmt.Errorf("sign-in failed: %s", tr.Error)
		default:
			return nil, fmt.Errorf("unexpected token response (HTTP %d)", status)
		}
	}
}

// Refresh exchanges the stored refresh token for a fresh access token, updating
// c in place. The caller is responsible for persisting c afterwards.
func Refresh(ctx context.Context, c *config.Credentials) error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.RefreshToken},
		"client_id":     {c.ClientID},
	}
	tr, status, _, err := postForm(ctx, c.Host+"/oauth/token", form)
	if err != nil {
		return err
	}
	if status != http.StatusOK || tr.AccessToken == "" {
		return fmt.Errorf("could not refresh the session — run `kd login` again")
	}
	c.AccessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		c.RefreshToken = tr.RefreshToken
	}
	if tr.TokenType != "" {
		c.TokenType = tr.TokenType
	}
	if tr.Scope != "" {
		c.Scopes = tr.Scope
	}
	c.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return nil
}

func postForm(ctx context.Context, endpoint string, form url.Values) (*tokenResponse, int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr tokenResponse
	// A non-JSON body (e.g. an HTML error page) leaves tr zero; callers key
	// off the status code in that case.
	_ = json.Unmarshal(body, &tr)
	return &tr, resp.StatusCode, body, nil
}

func credsFrom(host, clientID string, tr *tokenResponse) *config.Credentials {
	return &config.Credentials{
		Host:         host,
		ClientID:     clientID,
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		Scopes:       tr.Scope,
		Expiry:       time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}
}
