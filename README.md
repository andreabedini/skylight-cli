# Unofficial Skylight API (Community Reference)

> **Purpose**: A community-maintained reference for documenting the network endpoints used by the Skylight apps, based on observed traffic.  
> **Scope**: Reverse-engineered, incomplete, and **unofficial**. Not affiliated with Skylight. For research, interoperability, and educational use only.

---

## Disclaimers

- Respect the app’s Terms of Service and privacy laws. Only capture traffic from your own account/devices.
- Redact personal data and secrets (tokens, emails, IDs tied to real people) before committing.
- This repository **does not** encourage abusing the service or bypassing protections.

## Repo Organization

- `docs/openapi/openapi.yaml` — OpenAPI 3.0.3 spec for discovered endpoints. The source of truth everything else derives from.
- `examples/` — Example requests/responses (redacted).
- `docs/` — How-to guides (auth, capturing traffic, etc.) and static spec viewers.
- `cli/` — Go CLI generated from the spec by [`onlycli`](https://github.com/onlycli/onlycli). See [CLI](#cli).
- `CONTRIBUTING.md` — How to add endpoints/schemas.
- `SECURITY.md` — Responsible redaction guidelines.
- `LICENSE` — Default: CC BY-NC 4.0.

## Base URL

```
https://app.ourskylight.com
```

Most endpoints live under `/api/...` and follow **JSON:API** (`type`, `id`, `attributes`, `relationships`).

## Authentication

API requests carry an opaque OAuth 2.0 Bearer token:

```
Authorization: Bearer <REDACTED>
```

The app authenticates as a **public OAuth client** (`client_id=skylight-mobile`, **no client secret**). Observed traffic refreshes the access token with the **`refresh_token` grant** against `POST /oauth/token`; the response returns a Bearer access token (~2 h expiry) and a **rotating** refresh token (scope `everything`). The initial login/authorization step that mints the first refresh token has not been captured. Older traffic also used `Authorization: Basic <opaque token>`.

> Note: this is **not** the OAuth client-credentials grant (which would need a client secret and issues no refresh token).

Treat tokens as secrets. See [docs/auth.md](docs/auth.md) for the full request/response shape and capture steps.

## Example Endpoint

- `GET /api/frames/{frameId}/chores` — List chores for a frame within date bounds.  
  See the OpenAPI spec for full parameters and response.

## Tools & Workflow

- **Capture**: Charles / Proxyman / mitmproxy.
- **Inspect**: Confirm hostnames and query params.
- **Document**: Update `docs/openapi/openapi.yaml` with new schemas/endpoints.
- **Test**: Postman/Insomnia against your own account.

## CLI

A Go CLI is generated from the spec, so it stays in lockstep with the documented endpoints. Each operation becomes a subcommand grouped by tag (e.g. `skylight chores get-api-frames-frame-id --frame-id <id>`).

Tooling is pinned in `mise.toml` ([mise](https://mise.jdx.dev/)); run `mise install` once to get [`onlycli`](https://github.com/onlycli/onlycli) and [`vacuum`](https://github.com/daveshanley/vacuum).

```bash
# Regenerate the CLI after editing the spec (never hand-edit the generated Go)
mise run generate

# Build the binary (outputs cli/skylight)
mise run build

# Authenticate and call an endpoint
./cli/skylight auth login --client-id <id>
./cli/skylight chores get-api-frames-frame-id --frame-id <id>
```

Config lives at `~/.config/skylight/config.json` and supports named profiles; see `skylight config --help` and `skylight auth --help`.

## Roadmap

- Add auth/login flow (if observable).
- Expand coverage: chores create/update, categories, profiles, frames, rewards, schedules.
- Document rate limits, error shapes, pagination.

---

## Interactive Docs

You can explore the API spec in your browser:

- [Swagger UI](docs/swagger.html) — interactive “try it” interface  
- [Redoc](docs/redoc.html) — clean reference-style docs

On GitHub Pages these will be published at:

- `https://<user>.github.io/<repo>/docs/swagger.html`  
- `https://<user>.github.io/<repo>/docs/redoc.html`

---

## Local Preview

```bash
# from repo root
python3 -m http.server 8080
# then browse http://localhost:8080/docs/swagger.html
```

---

### Versions

- **v0.2.0** — Categories, Devices, Lists, Task Box endpoints; Basic auth scheme.  
- **v0.3.0** — Frames, Source Calendars, Calendar Events, Rewards, Reward Points; expanded schemas; corrected color formats; explicit `chore.status`.
- **v0.6.0** — New endpoints from 2026-06 captures: `GET /api/activities`, `GET /api/month_in_reviews`, `GET /api/reminder_profile`, `GET /api/frames/{frameId}/devices/{deviceId}` (full device-settings schema), `GET /api/frames/{frameId}/users` (frame members), `GET /api/frames/{frameId}/household_config`, `GET /api/frames/{frameId}/task_notification_settings`, and `POST /api/frames/{frameId}/source_calendars/set_default_for_new_events`. Documented the OAuth authorization-code/PKCE login flow (`/oauth/authorize`).
- **v0.5.1** — Schema corrections from 2026-04 captures: `chore.recurrence_set` is an array of RRULE strings (not a string); added `chore.series`/`timer_seconds`/`up_for_grabs` and chore relationships (`completed_category`, `habit_tracker`, `linked_task_group`); added calendar-event fields (`uid`, `status`, `recurring`, `recurring_config`, `master_event_id`, `source`, `editable`, `owner_email`, `supports_notification_settings`); documented the `category_detail` type and `profile_picture_urls`; corrected `GET /reward_points` to a bare (non-JSON:API) array of per-category balances. Added redacted examples for categories, calendar events, and reward points.

---

Maintainers: add yourself to [docs/maintainers.md](docs/maintainers.md) if you contribute regularly.
