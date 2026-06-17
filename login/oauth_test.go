package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestBuildAuthorizeURL(t *testing.T) {
	got := buildAuthorizeURL("https://app.ourskylight.com/", "CHAL", "STATE")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/oauth/authorize" {
		t.Fatalf("path = %q", u.Path)
	}
	q := u.Query()
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "skylight-mobile",
		"redirect_uri":          "skylight-family://welcome",
		"scope":                 "everything",
		"state":                 "STATE",
		"code_challenge":        "CHAL",
		"code_challenge_method": "S256",
	}
	for k, want := range checks {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestParseCallbackOK(t *testing.T) {
	code, err := parseCallback("skylight-family://welcome?code=ABC&state=S", "S")
	if err != nil {
		t.Fatal(err)
	}
	if code != "ABC" {
		t.Fatalf("code = %q", code)
	}
}

func TestParseCallbackStateMismatch(t *testing.T) {
	if _, err := parseCallback("skylight-family://welcome?code=ABC&state=X", "S"); err == nil {
		t.Fatal("expected state mismatch error")
	}
}

func TestParseCallbackError(t *testing.T) {
	if _, err := parseCallback("skylight-family://welcome?error=access_denied&error_description=no&state=S", "S"); err == nil {
		t.Fatal("expected error from error param")
	}
}

func TestExchangeCodeOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" ||
			r.FormValue("client_id") != "skylight-mobile" ||
			r.FormValue("code") != "CODE" ||
			r.FormValue("code_verifier") != "VERIFIER" ||
			r.FormValue("redirect_uri") != "skylight-family://welcome" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"AT","refresh_token":"RT","token_type":"Bearer","expires_in":7200,"scope":"everything"}`))
	}))
	defer srv.Close()

	tr, err := exchangeCode(srv.Client(), srv.URL, "CODE", "VERIFIER")
	if err != nil {
		t.Fatal(err)
	}
	if tr.AccessToken != "AT" || tr.RefreshToken != "RT" {
		t.Fatalf("got %+v", tr)
	}
}

func TestExchangeCodeNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()
	if _, err := exchangeCode(srv.Client(), srv.URL, "C", "V"); err == nil {
		t.Fatal("expected error on non-2xx")
	}
}
