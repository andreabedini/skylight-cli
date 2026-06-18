# Authentication Guide

> **Unofficial reference** — Use only with accounts you own or are authorized to use. Never commit real tokens.

Skylight requests observed so far use either:
- `Authorization: Basic <opaque token>` — **Not** username:password; an opaque bearer-like token.
- `Authorization: Bearer <token>` — Opaque OAuth 2.0 Bearer token (not a JWT). Expires after 2 hours.

This guide explains the authorization flow, the token refresh flow, and how to capture tokens for testing documented endpoints.

---

## OAuth Authorization (initial login)

The app obtains its first tokens via the **OAuth 2.0 Authorization Code grant with PKCE** (RFC 7636). The mobile client opens the browser at:

```
GET https://app.ourskylight.com/oauth/authorize
```

**Observed query parameters:**

| Field | Value |
|-------|-------|
| `response_type` | `code` |
| `client_id` | `skylight-mobile` |
| `redirect_uri` | `skylight-family://welcome` (custom URL scheme / app deep link) |
| `scope` | `everything` |
| `state` | Random opaque string (CSRF protection) |
| `code_challenge` | Base64url-encoded SHA-256 of the PKCE `code_verifier` (43 chars observed) |
| `code_challenge_method` | `S256` |
| `prompt` | `login` |

When there is no active web session, `/oauth/authorize` responds **302** → `https://app.ourskylight.com/auth/session/new` (the hosted login page). After the user authenticates, the server redirects to `redirect_uri` with `?code=<authorization_code>&state=<state>`, and the app exchanges the code at `POST /oauth/token` with `grant_type=authorization_code`, `client_id=skylight-mobile`, `code`, `code_verifier`, and `redirect_uri`.

> **Since observed:** the authorization-code redirect and the token exchange have now been exercised end-to-end on Linux via the [`skylight-login`](#logging-in-with-skylight-login) helper (which captures the custom-scheme redirect with a local socket). The server redirects to `skylight-family://welcome?code=…&state=…`, and the exchange at `POST /oauth/token` succeeds with `grant_type=authorization_code`, `client_id=skylight-mobile`, `code`, `code_verifier`, and `redirect_uri=skylight-family://welcome` — the standard PKCE completion, with no device-context fields required.

There is **no client secret** — `skylight-mobile` is a public client, which is why PKCE is used.

## Logging in with `skylight-login`

The `login/` module builds a small helper that completes the PKCE flow on Linux desktops and writes the bearer token into the CLI config:

1. Build and install: `cd login && go build -o "$HOME/.local/bin/skylight-login" .`
2. Register the `skylight-family://` scheme handler to run `skylight-login callback %u` (see the `.desktop` entry under `~/.local/share/applications/`).
3. Run `skylight-login login`. It opens the browser at `/oauth/authorize`, and when you finish logging in the handler relays the `skylight-family://welcome?code=…&state=…` callback back over a Unix socket (`$XDG_RUNTIME_DIR/skylight-login.sock`). The helper verifies `state`, exchanges the code at `POST /oauth/token`, writes `access_token` into the active profile in `~/.config/skylight/config.json`, and saves the rotating `refresh_token` to `~/.config/skylight/<profile>.refresh`.

Use `--profile <name>` to target a specific profile, and `--base-url` (or `SKYLIGHT_BASE_URL`) to override the endpoint.

> **Notes:** Run one `skylight-login login` at a time — a second concurrent login replaces the first's socket. The `callback` invocation receives the redirect URL as a command-line argument, so the single-use authorization `code` is briefly visible in the process list (`/proc/<pid>/cmdline`) until it exits; the matching PKCE `code_verifier` never leaves the login process.

---

## Hosted Web Login (the `/auth/session` flow)

`app.ourskylight.com` is its own identity provider. When `/oauth/authorize` has no active web session it 302-redirects (see above) to the **hosted login page** at `/auth/session/new`. The web app at `app.ourskylight.com` uses this same flow directly (no OAuth, no bearer token). It is a **plain Rails cookie-session login** — *not* OAuth — observed as three requests:

**1. `GET /auth/session/new` → 200**

Serves the HTML login form and sets an anonymous session cookie:

```
set-cookie: _skylight_cloud_session=REDACTED; path=/; secure; httponly; samesite=lax
```

The page embeds the Rails CSRF token in a hidden form field (and, separately, a `<meta name="csrf-token">` tag for XHR use):

```html
<form action="/auth/session" accept-charset="UTF-8" method="post">
  <input type="hidden" name="authenticity_token" value="REDACTED">
  <input required type="email"    name="email"    id="email">
  <input required type="password" name="password" id="password">
</form>
```

**2. `POST /auth/session` → 302** → `Location: /auth/session/success`

Standard form POST, carrying the cookie from step 1:

```
Content-Type: application/x-www-form-urlencoded

authenticity_token=<from the form>&email=<email>&password=<password>
```

On success the server **rotates the session cookie** (a fresh `_skylight_cloud_session` is set in the response) and redirects.

**3. `GET /auth/session/success` → 200**

The post-login landing page. Rotates the session cookie once more. No token is exposed in the body — the credential is the cookie itself.

Key points:

- **The credential is the `_skylight_cloud_session` cookie**, a Rails encrypted/signed session (the `…--…--…` shape is the Rails 5.2+ `AES-GCM` encrypted-cookie format). This flow yields **no bearer/access token** — it is entirely separate from the OAuth flows above.
- `authenticity_token` (the hidden form field) is the Rails CSRF token; it is **distinct** from the `csrf-token` `<meta>` tag (the latter is for `X-CSRF-Token` on subsequent XHR/`fetch` calls).
- The session cookie is `httponly; secure; samesite=lax`, so it is sent automatically by the browser on same-site navigations but is not readable from JavaScript.
- **Not yet observed:** whether the JSON:API backend (`/api/...`) accepts this session cookie directly, or whether the web app exchanges it for a bearer token. The capture this is based on contains only the `/auth/session/*` requests (no `/api/...` traffic).

---

## OAuth Token Refresh

The app refreshes its access token via a standard OAuth 2.0 refresh-token grant:

```
POST https://app.ourskylight.com/oauth/token
Content-Type: application/x-www-form-urlencoded
```

**Request body fields:**

| Field | Value |
|-------|-------|
| `grant_type` | `refresh_token` |
| `client_id` | `skylight-mobile` |
| `refresh_token` | Your current refresh token |
| `skylight_api_client_device_fingerprint` | UUID identifying the device installation |
| `skylight_api_client_device_platform` | `ios` (or `android`) |
| `skylight_api_client_device_name` | Human-readable device name (e.g. `iPhone`) |
| `skylight_api_client_device_os_version` | OS version string |
| `skylight_api_client_device_app_version` | App version string |
| `skylight_api_client_device_hardware` | Device model identifier (e.g. `iPhone14,5`) |

The device-context fields (`skylight_api_client_device_*`) are sent by the app on every token refresh. Their server-side enforcement is unknown; omitting them may still work.

**Response (200 OK):**

```json
{
  "access_token": "REDACTED",
  "token_type": "Bearer",
  "expires_in": 7200,
  "refresh_token": "REDACTED",
  "scope": "everything",
  "created_at": 1775915141
}
```

Key points:
- **Access tokens expire after 7200 seconds (2 hours).**
- **Refresh tokens rotate on every use** — save the new `refresh_token` from each response.
- The `scope` value observed is `"everything"`.
- Use the returned `access_token` as `Authorization: Bearer <access_token>` on subsequent API calls.

---

## 1) Capture via Proxy (Recommended)

Use one of these HTTPS debugging proxies:
- **Proxyman** (macOS GUI)
- **Charles Proxy** (macOS/Windows GUI)
- **mitmproxy** (CLI; scriptable)

### Steps
1. **Install and trust** the proxy's root certificate (System Keychain).
2. Enable **SSL Proxying** / **HTTPS capture**.
3. Launch the Skylight app and **log in**.
4. In the proxy session list, find the first authenticated request (e.g., `GET /api/frames/{frameId}/chores`).
5. Copy the **Authorization** header value.

> Tip: If you only see `CONNECT` entries or 4xx errors, enable SSL for the specific hostname and try again.

### Safety
- Tokens are secrets. **Do not** commit real values.
- When sharing examples, replace with `REDACTED` and keep the structure (header name/value format).

---

## 2) Electron/Chromium Apps (DevTools)

If the desktop app is Electron/Chromium-based:

1. Try **View → Toggle Developer Tools** from the app menu, or launch with:
   ```bash
   open -na "/Applications/Skylight.app" --args --remote-debugging-port=9222
   ```
2. Open Chrome → `chrome://inspect` → **inspect** the Skylight target.
3. Go to **Network** tab → click an API call → **Headers** → copy `Authorization`.

This avoids TLS interception and certificate pinning issues.

---

## 3) If HTTPS Decryption Fails (Certificate Pinning)

Some apps validate the server certificate in code (“pinning”). If your proxy shows CONNECT tunnels but no decrypted traffic:

- Try a different proxy (Proxyman/Charles/mitmproxy).
- Use **Frida** to hook common pinning points (`SecTrustEvaluate`, `NSURLSession`, Alamofire) on macOS.
- Run Skylight in a **VM** or use **transparent proxying** (e.g., mitmproxy as gateway) to redirect traffic.

> **Note**: Respect the app’s ToS and local laws. Use these techniques only for legitimate interoperability/debugging.

---

## 4) Using the Token (Postman/Insomnia/cURL)

- Add the header to your request:
  ```http
  Authorization: Basic REDACTED
  ```
  **or**
  ```http
  Authorization: Bearer REDACTED
  ```

- Example cURL:
  ```bash
  curl 'https://app.ourskylight.com/api/frames/REDACTED/chores?after=2025-08-25&before=2025-08-29'     -H 'Authorization: Basic REDACTED'     -H 'Accept: application/json'
  ```

If you receive **401 Unauthorized**:
- Log out/in in the Skylight app and recapture a fresh token.
- Ensure you copied the header **exactly** (no whitespace changes).

---

## 5) Redaction & Sharing

When contributing examples to this repo:
- Replace tokens and any PII with `REDACTED` (keep keys/shape intact).
- Use stable placeholders for related IDs if structure matters (e.g., `"CATEGORY_REDACTED"`).

See also: `../SECURITY.md` and `../CONTRIBUTING.md`.
