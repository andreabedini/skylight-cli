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

> **Not yet captured:** because `redirect_uri` is a custom URL scheme (not HTTPS), a proxy cannot see the authorization-code redirect, and the `authorization_code` token exchange body has not been observed. The parameters above are confirmed from a captured `/oauth/authorize` request; the exchange step is the standard PKCE completion. Contributions of a full capture (e.g. via a deep-link interceptor) are welcome.

There is **no client secret** — `skylight-mobile` is a public client, which is why PKCE is used.

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