# GitGym

GitGym is a desktop-first web sandbox for practicing Git commands against disposable repositories.

## Services

- `apps/web`: React workbench
- `services/api`: browser-facing API and auth/session orchestration
- `services/runner`: workspace and Git execution service

## Local Development

1. Copy `.env.example` to `.env.local` and fill in your local values.
2. Run `pnpm install`.
3. Run `npm run db:migrate` to create the MySQL schema.
4. Start the full local stack with `npm run dev`.

If you want to run services separately:

- `npm run runner:dev`
- `npm run api:dev`
- `npm run web:dev`

During local development you can set `DEV_AUTH_BYPASS=true` in `.env.local` to bypass GitHub auth and open the workbench directly without a browser session cookie.

The Vite dev server proxies `/api/*` and terminal websocket traffic to `API_BASE_URL` / `VITE_API_PROXY_TARGET`, so the browser app can run on `http://127.0.0.1:5173` while the API listens on `http://127.0.0.1:8080`.
