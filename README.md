# torogan-be

Go backend for **Torogan**, a rental-listing platform for the Philippines — browse verified
rentals and contact landlords directly, no middlemen.

Built on [ConnectRPC](https://connectrpc.com/), which serves every RPC simultaneously as
gRPC, gRPC-Web, and plain REST/JSON (via [Vanguard](https://connectrpc.com/docs/vanguard/)
transcoding) from a single handler. Persistence is PostgreSQL through
[GORM](https://gorm.io/), with the schema managed independently via raw SQL migrations
([`golang-migrate`](https://github.com/golang-migrate/migrate)).

The paired frontend lives at [`torogan-fe`](../torogan-fe).

## Tech stack

- **Go** + [ConnectRPC](https://connectrpc.com/) / [Vanguard](https://connectrpc.com/docs/vanguard/) — one handler, three protocols (gRPC, gRPC-Web, REST/JSON)
- **PostgreSQL** + [GORM](https://gorm.io/) — models in `internal/models/`, schema in raw SQL migrations
- **[buf](https://buf.build/)** — protobuf codegen for both this repo and `torogan-fe`'s TypeScript client
- **Docker Compose** — local Postgres + hot-reloading backend (via [`air`](https://github.com/air-verse/air))
- **AWS** — S3 (presigned uploads for property photos), SES (email verification)
- **Google OAuth** — sign-in via Google ID tokens

## Quick start

```bash
cp .env.example .env
# fill in DB_*, JWT_SECRET, GOOGLE_CLIENT_ID at minimum — see "Environment variables" below

make up
```

`make up` builds the Docker images, starts Postgres + the backend, waits for Postgres to be
ready, and runs all pending migrations. The backend then listens on `:8080` (or `$PORT`) and
hot-reloads on code changes — no rebuild needed unless you change a dependency
(`go.mod`/`go.sum`), in which case run `docker compose up --build` again.

## Make targets

| Command | What it does |
|---|---|
| `make up` | Build + start Postgres and the backend, then run pending migrations |
| `make down` | Stop and remove the containers |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back all migrations |
| `make migrate-create name=<name>` | Scaffold a new `migrations/<seq>_<name>.up.sql`/`.down.sql` pair |
| `make reset-db` | Wipe containers + volumes, rebuild, and re-run migrations from scratch |
| `make proto-gen` | Regenerate Go + Connect code from `proto/` (also regenerates `torogan-fe`'s TypeScript client) |

## Environment variables

See `.env.example` for the full list. Grouped summary:

| Group | Vars | Notes |
|---|---|---|
| Database | `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE` | |
| Redis | `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD` | **Declared but not currently wired into any Go code** — no client is instantiated anywhere yet |
| App | `APP_PORT`, `APP_ENV` (both currently unused) | The server actually reads a plain `PORT` env var (default `8080`) for its listen port — neither `APP_PORT` nor `APP_ENV` is read anywhere in code yet |
| JWT | `JWT_SECRET`, `JWT_EXPIRES_IN` (unused — access/refresh TTLs are hardcoded, 15 min / 7 days) | |
| CORS | `CORS_ALLOWED_ORIGINS` | Comma-separated list |
| Google OAuth | `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` (unused — only the client ID is needed to verify id_tokens) | |
| AWS S3 | `AWS_REGION`, `S3_BUCKET`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | Presigned property-photo uploads; the service is skipped (not fatal) if unset |
| AWS SES | `SES_FROM_ADDRESS`, `SES_AWS_ACCESS_KEY_ID`, `SES_AWS_SECRET_ACCESS_KEY` | Email verification; **deliberately separate credentials from S3's**, scoped to `ses:SendEmail` only. Leave `SES_FROM_ADDRESS` unset to log verification links to stdout instead of sending real email |
| Public URL | `PUBLIC_APP_URL` | Frontend origin used to build the verification email link; defaults to `http://localhost:3000` |

## Project structure

| Path | What's there |
|---|---|
| `cmd/server/` | Entry point — wires DB, services, handlers, and the Vanguard transcoder |
| `proto/` | Source `.proto` definitions (source of truth for the API) |
| `gen/` | Generated Go + Connect code — **never hand-edit**, run `make proto-gen` |
| `internal/database/` | DB connection setup + SQL migrations |
| `internal/models/` | GORM models, mirroring the migration schema |
| `pkg/services/` | Business logic, GORM queries, transactions |
| `pkg/handlers/` | Connect service implementations — request parsing, calling services, error mapping |
| `pkg/interceptors/` | Shared Connect interceptors (auth) |

## Request flow

```
gen/<x>v1connect (Connect interface)
  → pkg/handlers/   (parse request, call service, map errors)
  → pkg/services/   (business logic, GORM queries)
  → internal/models/ (GORM structs)
  → Postgres
```

Every RPC also declares a `google.api.http` annotation mapping it to a REST path, so the same
method is reachable as Connect/gRPC and as plain JSON over HTTP.

## Auth, at a glance

- Access tokens: HS256 JWTs, 15-minute expiry. Refresh tokens: stateless HS256 JWTs, 7-day
  expiry, delivered as an `HttpOnly`/`Secure`/`SameSite=Strict` cookie.
- Passwords are bcrypt-hashed; Google OAuth users have none.
- Traditional (email/password) registration is gated on **email verification** — the account
  is created but not logged in until the user clicks a link emailed via AWS SES (a stateless,
  short-lived JWT — no server-side storage). Google sign-ins skip this, since their email is
  already verified by Google.
- The AWS SES sending identity is currently in the **SES sandbox** (can only send to
  pre-verified addresses) — production access needs to be requested from AWS before real
  users can receive verification email.

## Testing & CI

There's no unit test suite yet (`go test ./...` currently finds nothing to run). CI
(`.github/workflows/ci.yml`) runs `golangci-lint`, a `go mod tidy` drift check, `go build
./...`, and a migration-apply smoke test against a real Postgres service container.

## Deployment

`docker-compose.prod.yml` runs Postgres + the backend behind a [Caddy](https://caddyserver.com/)
reverse proxy (`Caddyfile`) terminating TLS for `api.torogan.com` and forwarding to the
backend over h2c.
