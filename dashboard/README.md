# Gumi Dashboard

The dashboard is Gumi's local observability surface. It uses the runtime's
local control APIs through a same-origin proxy and never reads SQLite directly.

## Development

```bash
npm install
npm run dev
```

The development server runs at `http://127.0.0.1:8788` and proxies API
requests to a Gumi runtime at `http://127.0.0.1:8787`.

## Production build

```bash
npm ci
npm run build
```

From the repository root you can also use the top-level Makefile:

```bash
make dashboard
```

After `dashboard/dist` exists, `gumi start` serves it on the configured
dashboard port. Release archives embed `dashboard/dist` next to the binary so the
dashboard works without a separate build step. The Go dashboard server injects
the local API key into proxied requests, so the key is not shipped in the
browser bundle.

## Privacy

The dashboard displays request metadata only. Full prompts, responses, API
keys, and provider tokens are hidden by default.
