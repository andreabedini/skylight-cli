# skylight-login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A standalone Go binary `skylight-login` that performs the real Authorization Code + PKCE login and writes the resulting bearer token into the CLI's config, using a Unix socket so the `skylight-family://` scheme handler can relay the callback URL back to the in-flight login.

**Architecture:** One binary, two subcommands. `login` generates PKCE material, listens on a Unix socket, opens the browser at `/oauth/authorize`, waits for the OS-launched scheme handler to relay the callback URL over the socket, verifies `state`, exchanges the code at `/oauth/token`, and persists the access token to `~/.config/skylight/config.json` (refresh token to a sidecar file). `callback <url>` is invoked by the `.desktop` handler; it relays the URL to the socket, or falls back to `wl-copy`/`notify-send` if no login is running.

**Tech Stack:** Go (stdlib only — `crypto/rand`, `crypto/sha256`, `encoding/base64`, `net`, `net/http`, `net/url`, `encoding/json`, `os/exec`). No third-party dependencies. Linux/XDG desktop.

## Global Constraints

- The tool lives in a **new `login/` module at repo root**, fully separate from `cli/`. No file under `login/` carries a `DO NOT EDIT` marker; `onlycli generate` must never touch it.
- **stdlib only** — no third-party Go dependencies (`go.sum` stays empty).
- OAuth constants are fixed: `client_id=skylight-mobile`, `redirect_uri=skylight-family://welcome`, `scope=everything`, `code_challenge_method=S256`, default base URL `https://app.ourskylight.com`.
- Base-URL resolution order: profile `base_url` → `SKYLIGHT_BASE_URL` env → default.
- Config file is `~/.config/skylight/config.json`, written mode **0600**, 2-space-indented JSON with a trailing newline (matching the generated CLI). The refresh token goes to a sidecar `~/.config/skylight/<profile>.refresh`, also mode 0600.
- Socket path: `$XDG_RUNTIME_DIR/skylight-login.sock`, falling back to `~/.config/skylight/login.sock` if `XDG_RUNTIME_DIR` is unset.
- Browser is opened with `xdg-open`.
- Version control is **jj** (Jujutsu), not git. Commit with `jj commit <paths> -m "..."`. End commit messages with the `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` trailer.

---

## File Structure

```
login/
  go.mod            module github.com/skylight/skylight-cli/login, go 1.22
  main.go           subcommand dispatch + runLogin / runCallback wiring
  pkce.go           GenerateVerifier, Challenge, GenerateState
  pkce_test.go
  oauth.go          buildAuthorizeURL, parseCallback, tokenResponse, exchangeCode
  oauth_test.go
  socket.go         socketPath, listenForCallback, deliverCallback, notifyFallback
  socket_test.go
  config.go         config/profile structs, load/save, resolveBaseURL, targetProfileName, persistTokens
  config_test.go
```

---

## Task 1: Module scaffold + PKCE

**Files:**
- Create: `login/go.mod`
- Create: `login/pkce.go`
- Test: `login/pkce_test.go`

**Interfaces:**
- Produces: `GenerateVerifier() (string, error)`, `Challenge(verifier string) string`, `GenerateState() (string, error)` — all in `package main`.

- [ ] **Step 1: Create the module file**

Create `login/go.mod`:

```
module github.com/skylight/skylight-cli/login

go 1.22
```

- [ ] **Step 2: Write the failing test**

Create `login/pkce_test.go`:

```go
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestGenerateVerifierLength(t *testing.T) {
	v, err := GenerateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if len(v) < 43 || len(v) > 128 {
		t.Fatalf("verifier length %d out of RFC 7636 range 43..128", len(v))
	}
}

func TestChallengeMatchesS256(t *testing.T) {
	v := "test-verifier-value"
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got := Challenge(v); got != want {
		t.Fatalf("Challenge = %q, want %q", got, want)
	}
}

func TestStateIsRandom(t *testing.T) {
	a, _ := GenerateState()
	b, _ := GenerateState()
	if a == "" || a == b {
		t.Fatalf("expected distinct non-empty states, got %q and %q", a, b)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `cd login && go test ./... -run 'PKCE|Verifier|Challenge|State' -v`
Expected: FAIL — `undefined: GenerateVerifier` (build failure).

- [ ] **Step 4: Write the implementation**

Create `login/pkce.go`:

```go
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateVerifier returns a high-entropy PKCE code verifier (RFC 7636):
// 32 random bytes base64url-encoded -> 43 chars, within the 43..128 range.
func GenerateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Challenge returns the S256 code challenge for a verifier:
// base64url(SHA-256(verifier)), no padding.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// GenerateState returns a random opaque state/CSRF token (32 bytes base64url).
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd login && go test ./...`
Expected: PASS (`ok  github.com/skylight/skylight-cli/login`).

- [ ] **Step 6: Commit**

```bash
jj commit login/go.mod login/pkce.go login/pkce_test.go -m "feat(login): PKCE verifier/challenge/state generation

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: OAuth URL build, callback parse, token exchange

**Files:**
- Create: `login/oauth.go`
- Test: `login/oauth_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `buildAuthorizeURL(baseURL, challenge, state string) string`
  - `parseCallback(rawURL, expectedState string) (string, error)` — returns the authorization code.
  - `type tokenResponse struct { AccessToken, RefreshToken, TokenType, Scope string; ExpiresIn int }` (JSON-tagged).
  - `exchangeCode(client *http.Client, baseURL, code, verifier string) (*tokenResponse, error)`
  - exported constants `clientID = "skylight-mobile"`, `redirectURI = "skylight-family://welcome"`, `scope = "everything"`.

- [ ] **Step 1: Write the failing test**

Create `login/oauth_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd login && go test ./... -run OAuth -v` (or just `go test ./...`)
Expected: FAIL — `undefined: buildAuthorizeURL` (build failure).

- [ ] **Step 3: Write the implementation**

Create `login/oauth.go`:

```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd login && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit login/oauth.go login/oauth_test.go -m "feat(login): authorize URL, callback parsing, token exchange

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Unix-socket relay + notify fallback

**Files:**
- Create: `login/socket.go`
- Test: `login/socket_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `socketPath() string`
  - `listenForCallback(timeout time.Duration) (string, error)` — login side; binds, waits for one delivery, always removes the socket.
  - `deliverCallback(rawURL string) (bool, error)` — callback side; returns `false` (no error) when no login is listening.
  - `notifyFallback(rawURL string)` — `wl-copy` + `notify-send` best-effort.

- [ ] **Step 1: Write the failing test**

Create `login/socket_test.go`:

```go
package main

import (
	"testing"
	"time"
)

func TestSocketRelayRoundTrip(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	got := make(chan string, 1)
	errc := make(chan error, 1)
	go func() {
		u, err := listenForCallback(3 * time.Second)
		if err != nil {
			errc <- err
			return
		}
		got <- u
	}()

	// The listener may not have bound yet; retry delivery until it connects.
	deadline := time.Now().Add(2 * time.Second)
	delivered := false
	for time.Now().Before(deadline) {
		ok, err := deliverCallback("skylight-family://welcome?code=ABC&state=S")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			delivered = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !delivered {
		t.Fatal("callback was never delivered")
	}

	select {
	case u := <-got:
		if u != "skylight-family://welcome?code=ABC&state=S" {
			t.Fatalf("relayed url = %q", u)
		}
	case err := <-errc:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("listener did not return")
	}
}

func TestDeliverCallbackNoListener(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	ok, err := deliverCallback("skylight-family://welcome?code=ABC&state=S")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected ok=false when no listener is present")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd login && go test ./... -run Socket -v` (or `go test ./...`)
Expected: FAIL — `undefined: listenForCallback` (build failure).

- [ ] **Step 3: Write the implementation**

Create `login/socket.go`:

```go
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// socketPath returns the well-known path for the login IPC socket.
// Prefers $XDG_RUNTIME_DIR (user-only, 0700); falls back to the config dir.
func socketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "skylight-login.sock")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skylight", "login.sock")
}

// listenForCallback removes any stale socket, listens, and waits up to timeout
// for the scheme handler to deliver one callback URL. It always removes the
// socket before returning.
func listenForCallback(timeout time.Duration) (string, error) {
	path := socketPath()
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", path, err)
	}
	defer func() {
		ln.Close()
		_ = os.Remove(path)
	}()

	if ul, ok := ln.(*net.UnixListener); ok {
		_ = ul.SetDeadline(time.Now().Add(timeout))
	}
	conn, err := ln.Accept()
	if err != nil {
		return "", fmt.Errorf("waiting for callback: %w", err)
	}
	defer conn.Close()
	data, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// deliverCallback connects to an in-flight login and sends the callback URL.
// Returns (false, nil) when no login is listening so the caller can fall back.
func deliverCallback(rawURL string) (bool, error) {
	conn, err := net.DialTimeout("unix", socketPath(), 2*time.Second)
	if err != nil {
		return false, nil // no listener -> not delivered, not an error
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(rawURL)); err != nil {
		return false, err
	}
	return true, nil
}

// notifyFallback preserves the original handler behavior when no login runs.
func notifyFallback(rawURL string) {
	if path, err := exec.LookPath("wl-copy"); err == nil {
		c := exec.Command(path)
		c.Stdin = strings.NewReader(rawURL)
		_ = c.Run()
	}
	if path, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command(path, "Skylight link received", rawURL).Run()
	}
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd login && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj commit login/socket.go login/socket_test.go -m "feat(login): unix-socket callback relay with notify fallback

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Wire `login` + `callback` subcommands; manual end-to-end checkpoint

This task wires the pieces into a runnable binary that **prints** the access token (no config writing yet), so the unverified `POST /oauth/token` exchange gets a real end-to-end test before we build persistence.

**Files:**
- Create: `login/main.go`

**Interfaces:**
- Consumes: `GenerateVerifier`, `GenerateState`, `Challenge` (Task 1); `buildAuthorizeURL`, `parseCallback`, `exchangeCode` (Task 2); `listenForCallback`, `deliverCallback`, `notifyFallback` (Task 3).
- Produces: `runLogin(args []string) error`, `runCallback(args []string) error`, `main()`.

- [ ] **Step 1: Write the implementation**

Create `login/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: skylight-login <login|callback> [args]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "callback":
		err = runCallback(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	baseURLFlag := fs.String("base-url", "", "override base URL (default: $SKYLIGHT_BASE_URL or https://app.ourskylight.com)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	baseURL := *baseURLFlag
	if baseURL == "" {
		if env := os.Getenv("SKYLIGHT_BASE_URL"); env != "" {
			baseURL = env
		} else {
			baseURL = "https://app.ourskylight.com"
		}
	}

	verifier, err := GenerateVerifier()
	if err != nil {
		return err
	}
	state, err := GenerateState()
	if err != nil {
		return err
	}
	authURL := buildAuthorizeURL(baseURL, Challenge(verifier), state)

	fmt.Println("Opening browser to:")
	fmt.Println(authURL)
	if err := exec.Command("xdg-open", authURL).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "could not launch browser (%v); open the URL above manually\n", err)
	}

	rawCallback, err := listenForCallback(3 * time.Minute)
	if err != nil {
		return err
	}
	code, err := parseCallback(rawCallback, state)
	if err != nil {
		return err
	}
	tr, err := exchangeCode(http.DefaultClient, baseURL, code, verifier)
	if err != nil {
		return err
	}

	// Checkpoint build: print to confirm the exchange works.
	// Task 5 replaces this with persistTokens.
	fmt.Println("access_token:", tr.AccessToken)
	return nil
}

func runCallback(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: skylight-login callback <url>")
	}
	delivered, err := deliverCallback(args[0])
	if err != nil {
		return err
	}
	if !delivered {
		notifyFallback(args[0])
	}
	return nil
}
```

- [ ] **Step 2: Build and run the full unit suite**

Run: `cd login && go build -o skylight-login . && go test ./...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 3: MANUAL CHECKPOINT — confirm the token exchange end-to-end**

This is the gate that de-risks the never-captured `POST /oauth/token` step. Do NOT proceed to Task 5 until the access token prints.

1. In terminal A: `cd login && ./skylight-login login`
   The browser opens to `/oauth/authorize`.
2. Log in in the browser. It redirects to `skylight-family://welcome?code=…&state=…`. Your existing bash handler copies that URL to the clipboard (and shows a notification).
3. In terminal B, relay the copied URL to the waiting login:
   `cd login && ./skylight-login callback "$(wl-paste)"`
   (or paste the URL literally inside the quotes).
4. Terminal A should print `access_token: <real token>` and exit 0.

Expected: a real access token prints. If the exchange returns a non-2xx (e.g. `token endpoint returned 400: {"error":...}`), STOP and report the body — the `/oauth/token` request shape needs adjustment (e.g. device-context fields) before continuing.

- [ ] **Step 4: Commit**

```bash
jj commit login/main.go -m "feat(login): wire login/callback subcommands (prints token)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Config persistence (token + refresh sidecar)

**Files:**
- Create: `login/config.go`
- Test: `login/config_test.go`
- Modify: `login/main.go` (add `--profile`, replace the print with persistence and resolve base URL via config)

**Interfaces:**
- Consumes: `tokenResponse` (Task 2).
- Produces:
  - `type profile struct { BaseURL, Token, AuthType string }` and `type config struct { DefaultProfile string; Profiles map[string]*profile }` (JSON shape matching the generated CLI).
  - `configPath() string`, `refreshTokenPath(profileName string) string`
  - `loadConfig() (*config, error)`, `saveConfig(c *config) error`
  - `targetProfileName(c *config, flagProfile string) string`
  - `resolveBaseURL(p *profile) string`
  - `persistTokens(profileName string, tr *tokenResponse) error`

- [ ] **Step 1: Write the failing test**

Create `login/config_test.go`:

```go
package main

import (
	"os"
	"testing"
)

func TestPersistTokensRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := persistTokens("default", &tokenResponse{AccessToken: "AT", RefreshToken: "RT"}); err != nil {
		t.Fatal(err)
	}

	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Profiles["default"].Token != "AT" {
		t.Fatalf("token = %q", c.Profiles["default"].Token)
	}
	if c.DefaultProfile != "default" {
		t.Fatalf("default profile = %q", c.DefaultProfile)
	}

	rt, err := os.ReadFile(refreshTokenPath("default"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rt) != "RT" {
		t.Fatalf("refresh = %q", string(rt))
	}

	info, err := os.Stat(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestPersistPreservesOtherProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	seed := &config{DefaultProfile: "work", Profiles: map[string]*profile{
		"work": {Token: "existing", BaseURL: "https://example.test"},
	}}
	if err := saveConfig(seed); err != nil {
		t.Fatal(err)
	}

	if err := persistTokens("home", &tokenResponse{AccessToken: "NEW"}); err != nil {
		t.Fatal(err)
	}

	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["work"].Token != "existing" || got.Profiles["work"].BaseURL != "https://example.test" {
		t.Fatalf("clobbered existing profile: %+v", got.Profiles["work"])
	}
	if got.Profiles["home"].Token != "NEW" {
		t.Fatalf("new token not written: %+v", got.Profiles["home"])
	}
	if got.DefaultProfile != "work" {
		t.Fatalf("default profile changed to %q", got.DefaultProfile)
	}
}

func TestResolveBaseURL(t *testing.T) {
	t.Setenv("SKYLIGHT_BASE_URL", "")
	if got := resolveBaseURL(&profile{BaseURL: "https://p.test"}); got != "https://p.test" {
		t.Fatalf("profile base = %q", got)
	}
	if got := resolveBaseURL(nil); got != "https://app.ourskylight.com" {
		t.Fatalf("default base = %q", got)
	}
	t.Setenv("SKYLIGHT_BASE_URL", "https://env.test")
	if got := resolveBaseURL(&profile{}); got != "https://env.test" {
		t.Fatalf("env base = %q", got)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd login && go test ./... -run 'Persist|ResolveBaseURL' -v`
Expected: FAIL — `undefined: persistTokens` (build failure).

- [ ] **Step 3: Write the implementation**

Create `login/config.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultBaseURL = "https://app.ourskylight.com"

// profile mirrors the generated CLI's profile shape in config.json.
type profile struct {
	BaseURL  string `json:"base_url,omitempty"`
	Token    string `json:"token,omitempty"`
	AuthType string `json:"auth_type,omitempty"`
}

type config struct {
	DefaultProfile string              `json:"default_profile"`
	Profiles       map[string]*profile `json:"profiles"`
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "skylight", "config.json")
}

func refreshTokenPath(profileName string) string {
	return filepath.Join(filepath.Dir(configPath()), profileName+".refresh")
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &config{Profiles: map[string]*profile{}}, nil
		}
		return nil, err
	}
	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*profile{}
	}
	return &c, nil
}

func saveConfig(c *config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(data, '\n'), 0o600)
}

// targetProfileName picks the profile to write: explicit flag, else the
// config's default, else "default".
func targetProfileName(c *config, flagProfile string) string {
	if flagProfile != "" {
		return flagProfile
	}
	if c.DefaultProfile != "" {
		return c.DefaultProfile
	}
	return "default"
}

// resolveBaseURL: profile base_url -> SKYLIGHT_BASE_URL -> default.
func resolveBaseURL(p *profile) string {
	if p != nil && p.BaseURL != "" {
		return p.BaseURL
	}
	if env := os.Getenv("SKYLIGHT_BASE_URL"); env != "" {
		return env
	}
	return defaultBaseURL
}

// persistTokens writes the access token into the named profile (creating it if
// needed, preserving others) and the refresh token to a 0600 sidecar file.
func persistTokens(profileName string, tr *tokenResponse) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	p := c.Profiles[profileName]
	if p == nil {
		p = &profile{}
		c.Profiles[profileName] = p
	}
	p.Token = tr.AccessToken
	if c.DefaultProfile == "" {
		c.DefaultProfile = profileName
	}
	if err := saveConfig(c); err != nil {
		return err
	}
	if tr.RefreshToken != "" {
		if err := os.WriteFile(refreshTokenPath(profileName), []byte(tr.RefreshToken), 0o600); err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd login && go test ./...`
Expected: PASS.

- [ ] **Step 5: Wire persistence into `runLogin`**

In `login/main.go`, add the `--profile` flag and resolve the base URL from config. Replace everything from the `fs := flag.NewFlagSet("login", ...)` line down through the end of the old base-URL block (the `baseURL := *baseURLFlag` / `if baseURL == "" { ... }` lines) — i.e. everything before `verifier, err := GenerateVerifier()` — with:

```go
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	baseURLFlag := fs.String("base-url", "", "override base URL (default: profile, $SKYLIGHT_BASE_URL, or https://app.ourskylight.com)")
	profileFlag := fs.String("profile", "", "config profile to write (default: active profile)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	profileName := targetProfileName(cfg, *profileFlag)

	baseURL := *baseURLFlag
	if baseURL == "" {
		baseURL = resolveBaseURL(cfg.Profiles[profileName])
	}
```

Then replace the checkpoint print at the end of `runLogin`:

```go
	if err := persistTokens(profileName, tr); err != nil {
		return err
	}
	fmt.Printf("Logged in. Access token saved to profile %q in %s\n", profileName, configPath())
	return nil
```

- [ ] **Step 6: Build and run the full suite**

Run: `cd login && go build -o skylight-login . && go test ./...`
Expected: build succeeds; all tests PASS. (`time` is still used by `runLogin`; no unused imports.)

- [ ] **Step 7: Commit**

```bash
jj commit login/config.go login/config_test.go login/main.go -m "feat(login): persist access token to config, refresh token to sidecar

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Desktop handler integration + docs

**Files:**
- Modify: `~/.local/share/applications/skylight-family-handler.desktop` (point `Exec` at the new binary)
- Modify: `docs/auth.md` (document the working login path)

**Interfaces:** none (integration + documentation).

- [ ] **Step 1: Install the binary**

Run: `cd login && go build -o "$HOME/.local/bin/skylight-login" .`
Expected: binary at `~/.local/bin/skylight-login`.

- [ ] **Step 2: Repoint the desktop handler**

Edit `~/.local/share/applications/skylight-family-handler.desktop` so the `Exec` line reads (use the absolute path so it resolves regardless of the launcher's PATH):

```
Exec=/home/andrea/.local/bin/skylight-login callback %u
```

Then refresh the desktop database:

Run: `update-desktop-database "$HOME/.local/share/applications" 2>/dev/null; xdg-mime query default x-scheme-handler/skylight-family`
Expected: prints `skylight-family-handler.desktop`.

- [ ] **Step 3: Full flow verification**

Run: `skylight-login login`, log in in the browser, and let the OS-launched handler relay the callback automatically (no second terminal this time).
Expected: `Logged in. Access token saved to profile "<name>" in ~/.config/skylight/config.json`. Confirm with `skylight frames get-api` (or any authenticated call) that the saved token works.

- [ ] **Step 4: Document the login path**

In `docs/auth.md`, add a section after "OAuth Authorization (initial login)" describing the tool:

```markdown
## Logging in with `skylight-login`

The `login/` module builds a small helper that completes the PKCE flow on Linux
desktops and writes the bearer token into the CLI config:

1. Build and install: `cd login && go build -o "$HOME/.local/bin/skylight-login" .`
2. Register the `skylight-family://` scheme handler to run
   `skylight-login callback %u` (see the `.desktop` entry under
   `~/.local/share/applications/`).
3. Run `skylight-login login`. It opens the browser at `/oauth/authorize`, and
   when you finish logging in the handler relays the
   `skylight-family://welcome?code=…&state=…` callback back over a Unix socket
   (`$XDG_RUNTIME_DIR/skylight-login.sock`). The helper verifies `state`,
   exchanges the code at `POST /oauth/token`, writes `access_token` into the
   active profile in `~/.config/skylight/config.json`, and saves the rotating
   `refresh_token` to `~/.config/skylight/<profile>.refresh`.

This confirms the previously-uncaptured authorization-code redirect: the server
redirects to the custom scheme with `?code=…&state=…`, and the token exchange
uses `grant_type=authorization_code`, `client_id=skylight-mobile`,
`code_verifier`, and `redirect_uri=skylight-family://welcome`.
```

- [ ] **Step 5: Commit the docs**

```bash
jj commit docs/auth.md -m "docs(auth): document skylight-login PKCE login helper

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

(The `.desktop` file lives outside the repo and is not committed.)

---

## Notes for the implementer

- Run all `go` commands from inside `login/` (it is a separate module from `cli/`).
- `cli/` is generated — never edit it. This tool deliberately re-declares the config shape rather than importing `cli/runtime`.
- The repo's pre-existing uncommitted change to `docs/auth.md` should be left intact; Task 6 only *adds* a section.
- If the manual checkpoint in Task 4 fails at the token exchange, that is the signal to revisit the `/oauth/token` request body (the device-context `skylight_api_client_device_*` fields documented in `docs/auth.md` for the refresh grant may also be required here) before building persistence.
