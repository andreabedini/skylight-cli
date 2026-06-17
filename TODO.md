# Documentation TODO

Known gaps identified by inspection of the current spec. Each item needs a
HAR capture or other evidence before it can be filled in.

---

## 1. Missing individual-resource GET endpoints

Several resources have collection GET and write operations but no single-item GET.

| Missing | Already have |
|---------|-------------|
| `GET /api/frames/{frameId}/chores/{choreId}` | `GET /chores`, `PUT/DELETE /chores/{id}` |
| `GET /api/frames/{frameId}/calendar_events/{eventId}` | `GET /calendar_events`, `PUT/DELETE /{id}` |
| `GET /api/frames/{frameId}/meals/sittings/{sittingId}` | `GET /sittings`, `GET /{id}/instances` |
| `GET /api/frames/{frameId}/reward_points/{pointId}` | `GET /reward_points`, `POST /reward_points` |
| `GET /api/frames/{frameId}/lists/{listId}/list_items/{itemId}` | `POST/PUT /{itemId}` |

---

## 2. Missing write operations on read-only resources

Resources where only GET is documented; create/update/delete almost certainly exist.

### Devices
- `DELETE /api/frames/{frameId}/devices/{deviceId}` — remove/unlink a device from a frame
- Possibly `PATCH /api/frames/{frameId}/devices/{deviceId}` — rename or configure a device

### Source calendars
- `POST /api/frames/{frameId}/source_calendars` — connect a calendar account
- `DELETE /api/frames/{frameId}/source_calendars/{calendarId}` — disconnect
- The related *calendar account* resource (Google/Apple/etc. OAuth credentials) may
  be a separate resource; only a single GET collection is currently documented.

### Event notification settings
- `PUT` or `PATCH /api/frames/{frameId}/event_notification_settings/{settingId}` — update notification preferences

### Meal sittings
- `PUT` or `PATCH /api/frames/{frameId}/meals/sittings/{sittingId}` — edit sitting
- `DELETE /api/frames/{frameId}/meals/sittings/{sittingId}` — delete/cancel sitting

### Meal categories
- Likely read-only reference data, but `POST/PUT/DELETE` may exist if categories are user-managed.

---

## 3. Missing collection endpoints

Resources where a single-item endpoint is documented but no list endpoint.

| Missing | Already have |
|---------|-------------|
| `GET /api/frames/{frameId}/auto_creation_intents` | `GET /{intentId}` |

~~`GET /api/frames` (list all frames, any type)~~ — **documented** (confirmed 200 in a 2026-06 capture).

---

## 4. Messages — write operations entirely absent

Only read operations are documented. A social messaging feature almost certainly supports:

- `POST /api/frames/{frameId}/messages` — create/send a message
- `DELETE /api/frames/{frameId}/messages/{messageId}` — delete a message
- `POST /api/frames/{frameId}/messages/{messageId}/comments` — add a comment
- `DELETE /api/frames/{frameId}/messages/{messageId}/comments/{commentId}` — delete a comment
- `POST /api/frames/{frameId}/messages/{messageId}/likes` (or `/all_likes`) — like a message
- `DELETE /api/frames/{frameId}/messages/{messageId}/likes/{likeId}` — unlike

---

## 5. Albums — full CRUD absent

Only `GET /api/frames/{frameId}/albums` is documented.

- `POST /api/frames/{frameId}/albums` — create album
- `GET/PUT/DELETE /api/frames/{frameId}/albums/{albumId}` — individual album operations
- Album contents (photos/media): `GET/POST/DELETE /api/frames/{frameId}/albums/{albumId}/photos`
  or similar sub-resource

---

## 6. Auto-creation intents — CRUD absent

Only `GET /{intentId}` is documented.

- `GET /api/frames/{frameId}/auto_creation_intents` — list all intents
- `POST /api/frames/{frameId}/auto_creation_intents` — create automation rule
- `PUT/PATCH /api/frames/{frameId}/auto_creation_intents/{intentId}` — update rule
- `DELETE /api/frames/{frameId}/auto_creation_intents/{intentId}` — delete rule

---

## 7. Auth — likely missing operations

- `DELETE /api/sessions` or `DELETE /api/sessions/{sessionId}` — logout / invalidate session
- `PATCH /api/user` or `PUT /api/user` — update profile (name, avatar, etc.)
- Password reset flow (`POST /api/passwords` or similar)
- Email verification endpoints

---

## 8. List items — missing DELETE

List items can be created (`POST`) and updated (`PUT`) but not deleted.

- `DELETE /api/frames/{frameId}/lists/{listId}/list_items/{itemId}`

---

## 9. Task box items — missing individual operations

Only collection GET and POST are documented.

- `GET /api/frames/{frameId}/task_box/items/{itemId}`
- `PUT` or `PATCH /api/frames/{frameId}/task_box/items/{itemId}`
- `DELETE /api/frames/{frameId}/task_box/items/{itemId}`

---

## 10. Reward points — missing individual operations and DELETE

- `GET /api/frames/{frameId}/reward_points/{pointId}`
- `DELETE /api/frames/{frameId}/reward_points/{pointId}` — reverse/void a point entry

---

## 11. Undocumented resource types (likely to exist)

| Resource | Rationale |
|----------|-----------|
| **Calendar accounts** | `source_calendars` suggests connected Google/Apple/etc. accounts are managed somewhere; account connection/OAuth flow is undocumented |
| **Photos / media uploads** | Albums exist but no upload or photo management endpoints are documented |
| **Frame settings / configuration** | Display preferences, timezone, timezone, display name, etc. |
| **Chore bulk-create** | TheEagleByte's spec hints at `POST .../chores/{choreId}create_multiple` (possible typo in their path); bulk recurring-chore creation may exist |
| **Profile picture upload** | `CategoryAttributes` has `profile_pic_url`; an upload endpoint must exist to set it |

---

## 12. Cross-cutting concerns not yet documented

- **Pagination** — `sync_token`, `page_token`, `after`/`before` cursor params observed in the
  community spec but not formally documented on any list endpoint in our spec.
- **Error response schemas** — no `4xx`/`5xx` response bodies are documented anywhere.
- **`apply_to` values** — recurring-resource endpoints accept an `apply_to` param but the
  full set of valid values (`this`, `this_and_following`, `all`) needs confirmation.
- **`include` / `filter` params** — present on several endpoints; the full set of accepted
  values per endpoint is not yet enumerated.
