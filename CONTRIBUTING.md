# Contributing

Thanks for helping improve the Skylight CLI and the spec it's generated from!

The CLI is produced entirely from `docs/openapi/openapi.yaml` by [`onlycli`](https://github.com/onlycli/onlycli). So contributions are almost always **edits to the spec** (new endpoints, schemas, fixes) — never hand-edits to the generated Go under `cli/`.

## What to add

- New endpoints (paths, params, request/response shapes), discovered from real traffic.
- Schemas (resources, relationships, enums) and corrections to existing ones.
- Redacted request/response examples under `examples/`.

## How to capture & redact safely

1. Capture traffic from **your own account** with a proxy (Proxyman/Charles/mitmproxy) or DevTools.
2. Keep the raw capture local — `captures/` is gitignored precisely because HAR files hold live tokens and PII. **Never commit a raw capture.**
3. **Redact** before anything goes into `examples/` or the spec:
   - Access tokens / `Authorization` headers, cookies, IDs tied to real people.
   - Emails, phone numbers, addresses, GPS coordinates.
   - Anything you wouldn't want public. Use placeholders like `REDACTED`.
4. Confirm the example still illustrates the structure after redaction.

## Editing the spec

- Edit `docs/openapi/openapi.yaml`. Keep resources aligned with JSON:API where applicable.
- Document **only observed behavior**; phrase descriptions objectively ("observed …").
- Put enums on query/path params when you've seen the accepted values.
- Prefer small, focused changes — one endpoint at a time — and note how you discovered it.

## Style

- The spec is **OpenAPI 3.0.3**; match the existing idioms.
- Keep descriptions objective and evidence-based.

## Regenerate & verify

After editing the spec:

```bash
vacuum lint docs/openapi/openapi.yaml   # lint the spec
mise run generate-cli                   # regenerate cli/ from the spec
mise run build                          # rebuild the binary (-> bin/skylight)
```

A few pre-existing `operation-operationId` lint findings are expected (the spec intentionally omits operationIds); your change shouldn't add new error *types*. Sanity-check the generated command with `--dry-run`.

## Legal & ethics

- Only capture from accounts you own or are authorized to use.
- Don't automate abusive traffic.
- Respect takedown requests from rights holders (see [SECURITY.md](SECURITY.md)).
