# GitGym

GitGym is a desktop-first web sandbox for practicing Git commands against disposable repositories.

## Services

- `apps/web`: React workbench
- `services/api`: browser-facing API and auth/session orchestration
- `services/runner`: workspace and Git execution service

## Local Development

1. Copy `.env.example` to `.env.local` and fill in your local values.
2. For real local GitHub login, keep `DEV_AUTH_BYPASS=false` and configure a GitHub OAuth app with callback URL `http://127.0.0.1:8080/api/v1/auth/github/callback`.
3. Set `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `API_BASE_URL=http://127.0.0.1:8080`, and `FRONTEND_REDIRECT_URL=http://127.0.0.1:5173` in `.env.local` so the API can complete the OAuth redirect back to the Vite app.
4. Use `DEV_AUTH_BYPASS=true` only as a loopback-only emergency shortcut when you cannot complete GitHub OAuth locally.
5. Run `pnpm install`.
6. Run `npm run db:migrate` to create the MySQL schema.
7. Start the full local stack with `npm run dev`.

If you want to run services separately:

- `npm run runner:dev`
- `npm run api:dev`
- `npm run web:dev`

`DEV_AUTH_BYPASS=true` only applies to loopback requests. Non-loopback requests still require a real persisted browser session cookie.

The Vite dev server proxies `/api/*` and terminal websocket traffic to `API_BASE_URL` / `VITE_API_PROXY_TARGET`, so the browser app can run on `http://127.0.0.1:5173` while the API listens on `http://127.0.0.1:8080`.
