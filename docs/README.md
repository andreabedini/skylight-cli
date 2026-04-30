# Docs Site

Two static viewers are included:
- `docs/swagger.html` (Swagger UI)
- `docs/redoc.html` (Redoc)

They load the spec from `openapi/openapi.yaml` (relative to this folder, i.e. `docs/openapi/openapi.yaml`).

## GitHub Pages

1. Commit & push to `main` (or `master`).
2. In repo settings → Pages, set **Source** to **Deploy from a branch**, branch `main`, folder `/ (root)`. This serves the whole repo as-is.
3. Visit:
   - `/docs/swagger.html`
   - `/docs/redoc.html`

## Local (no build tools)

Open `docs/swagger.html` or `docs/redoc.html` in a local HTTP server:
```bash
# from repo root
python3 -m http.server 8080
# then browse http://localhost:8080/docs/swagger.html
```
