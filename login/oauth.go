package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	clientID    = "skylight-mobile"
	redirectURI = "skylight-family://welcome"
	scope       = "everything"
)

// buildAuthorizeURL builds the /oauth/authorize URL for the PKCE flow.
func buildAuthorizeURL(baseURL, challenge, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "login")
	return strings.TrimRight(baseURL, "/") + "/oauth/authorize?" + q.Encode()
}

// parseCallback validates an OAuth callback URL and returns the authorization
// code. It rejects URLs carrying an error param and a state mismatch.
func parseCallback(rawURL, expectedState string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse callback url: %w", err)
	}
	q := u.Query()
	if e := q.Get("error"); e != "" {
		return "", fmt.Errorf("authorization error: %s %s", e, q.Get("error_description"))
	}
	if got := q.Get("state"); got != expectedState {
		return "", fmt.Errorf("state mismatch: got %q, want %q", got, expectedState)
	}
	code := q.Get("code")
	if code == "" {
		return "", fmt.Errorf("callback missing code")
	}
	return code, nil
}

// tokenResponse holds the fields we use from POST /oauth/token.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

// exchangeCode trades an authorization code + PKCE verifier for tokens.
func exchangeCode(client *http.Client, baseURL, code, verifier string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)

	endpoint := strings.TrimRight(baseURL, "/") + "/oauth/token"
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	return &tr, nil
}
