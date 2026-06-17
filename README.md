# Skylight CLI

> **Unofficial** command-line client for the Skylight family calendar/frame API
> (`https://app.ourskylight.com`). The CLI is generated from a reverse-engineered
> OpenAPI spec, so it tracks exactly the endpoints we've documented.
>
> Not affiliated with Skylight. For personal interoperability, automation, and
> research against **your own account** only.

---

## Disclaimers

- Respect the app's Terms of Service and privacy laws. Only call the API with an account you own or are authorized to use.
- Tokens and personal data are secrets — never commit them. Raw traffic captures stay local (`captures/` is gitignored).
- This project **does not** encourage abusing the service or bypassing protections.

## Install

Tooling is pinned in `mise.toml` ([mise](https://mise.jdx.dev/)). Run once to get [`onlycli`](https://github.com/onlycli/onlycli) (the generator) and [`vacuum`](https://github.com/daveshanley/vacuum) (the spec linter):

```bash
mise install
mise run build      # compiles to bin/skylight (on your PATH inside the mise env)
```

`mise.toml` puts `bin/` on the path, so once built you can call `skylight` directly. Or build with Go (1.22+): `cd cli && go build -o ../bin/skylight .`

## Authenticate

The API authenticates with an opaque OAuth 2.0 **Bearer** token. The mobile app obtains it via an Authorization Code + PKCE login (see [docs/auth.md](docs/auth.md)); the CLI does **not** implement that browser flow, so supply a token captured from your own session:

```bash
# Save a token under the "default" profile and select it
skylight config set default.token <BEARER_TOKEN>
skylight config use-profile default

# …or pass it per-invocation via the environment
export SKYLIGHT_TOKEN=<BEARER_TOKEN>
```

`docs/auth.md` explains how to capture a token (proxy / DevTools) and the refresh-token flow the app uses.

> There is a scaffolded `skylight auth login`, but it was generated from the spec's OAuth scheme and uses device-code/client-credentials grants that don't match the real service — use a captured token as above.

## Usage

Commands are grouped by API area; each operation is `<group> <method-and-path>`. Path parameters become required flags, query parameters become optional flags.

```bash
# Current user
skylight user get-api

# List your frames (by type)
skylight frames get-api-frames-tv

# Chores for a frame on a given day
skylight chores get-api-frames-frame-id \
    --frame-id 5125905 --after 2026-06-17 --before 2026-06-17

# Calendar events for a date range, with related resources included
skylight calendars get-api-frames-frame-id-calendar-events \
    --frame-id 5125905 --date-min 2026-06-17 --date-max 2026-07-23 \
    --timezone Australia/Perth --include categories,calendar_account
```

Run `skylight --help`, or `<group> --help`, to discover commands.

**Global flags** (available on every command):

| Flag | Purpose |
|------|---------|
| `--format` | Output as `json` (default), `pretty`, `yaml`, `jsonl`, `table`, `csv`, `raw` |
| `--transform '<expr>'` | Filter/reshape JSON output with a [GJSON](https://github.com/tidwall/gjson) expression |
| `--template '<tmpl>'` | Format output with a Go template |
| `--profile <name>` | Use a named config profile |
| `--page-limit <n>` | Auto-paginate up to N pages |
| `--max-retries <n>` | Retry 429/5xx responses |
| `--stream` | Stream SSE/NDJSON line-by-line |
| `--dry-run` | Print the HTTP request without sending it |
| `--verbose` | Log request/response details to stderr |

**Configuration** lives at `~/.config/skylight/config.json` and supports multiple profiles (keys: `token`, `base_url`, `auth_type`). Manage it with `skylight config set|get|list|use-profile`. The base URL resolves from the profile, then `SKYLIGHT_BASE_URL`, then the default `https://app.ourskylight.com`.

## How it works

`docs/openapi/openapi.yaml` is the source of truth. `onlycli` generates the entire `cli/` tree from it — so the way to change the CLI is to edit the spec and regenerate (never hand-edit the generated Go):

```bash
mise run generate-cli   # regenerate cli/ from the spec
mise run build          # rebuild the binary (-> bin/skylight)
```

The spec itself is reverse-engineered from observed traffic and is intentionally incomplete; known gaps are tracked in [TODO.md](TODO.md).

## API reference

- **Spec**: [`docs/openapi/openapi.yaml`](docs/openapi/openapi.yaml) (OpenAPI 3.0.3) — endpoints follow JSON:API (`type`, `id`, `attributes`, `relationships`).
- **Auth**: [docs/auth.md](docs/auth.md) — login/refresh flow and how to capture a token.
- **Examples**: [`examples/`](examples/) — redacted request/response samples.
- **Browse the spec**:
  - [Swagger UI](docs/swagger.html) (interactive) · [Redoc](docs/redoc-static.html) (self-contained, built with `mise run generate-docs`)
  - Locally: `python3 -m http.server 8080`, then open <http://localhost:8080/docs/swagger.html>
  - On GitHub Pages: `https://andreabedini.github.io/Skylight/docs/swagger.html`

## Contributing

New endpoints are added by capturing traffic from your own account, redacting it, updating the spec, and regenerating. See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Never commit tokens or PII; keep raw HAR files local.

## Changelog (spec coverage)

- **v0.6.0** — Endpoints from 2026-06 captures: `GET /api/activities`, `GET /api/month_in_reviews`, `GET /api/reminder_profile`, `GET /api/frames/{frameId}/devices/{deviceId}` (full device-settings schema), `GET /api/frames/{frameId}/users`, `GET /api/frames/{frameId}/household_config`, `GET /api/frames/{frameId}/task_notification_settings`, `POST /api/frames/{frameId}/source_calendars/set_default_for_new_events`. Documented the OAuth authorization-code/PKCE login flow.
- **v0.5.1** — Schema corrections from 2026-04 captures: `chore.recurrence_set` is an array of RRULE strings; added `chore.series`/`timer_seconds`/`up_for_grabs` and chore relationships; added calendar-event fields (`uid`, `status`, `recurring`, `master_event_id`, `source`, `editable`, `owner_email`, …); documented the `category_detail` type and `profile_picture_urls`; corrected `GET /reward_points` to a bare (non-JSON:API) array.
- **v0.3.0** — Frames, Source Calendars, Calendar Events, Rewards, Reward Points; expanded schemas; corrected color formats; explicit `chore.status`.
- **v0.2.0** — Categories, Devices, Lists, Task Box endpoints.

## License

Released under CC BY-NC 4.0 — see [LICENSE](LICENSE).

---

## Acknowledgements

This project is a fork of **[ColinScattergood/Skylight](https://github.com/ColinScattergood/Skylight)** by Colin Scattergood, the original author of the reverse-engineered Skylight API documentation this CLI is built on. Thank you, Colin, for the groundwork. 🙏
