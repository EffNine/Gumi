# Novexa Dashboard

The dashboard is Novexa's local observability surface. It uses the runtime's
local control APIs through a same-origin proxy and never reads SQLite directly.

## Development

```bash
npm install
npm run dev
```

The development server runs at `http://127.0.0.1:8788` and proxies API
requests to a Novexa runtime at `http://127.0.0.1:8787`.

## Production build

```bash
npm run build
```

After `dashboard/dist` exists, `novexa start` serves it on the configured
dashboard port. The Go dashboard server injects the local API key into proxied
requests, so the key is not shipped in the browser bundle.

## Privacy

The dashboard displays request metadata only. Full prompts, responses, API
keys, and provider tokens are hidden by default.
