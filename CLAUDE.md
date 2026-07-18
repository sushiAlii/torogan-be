# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Torogan is a Go backend built on ConnectRPC (gRPC + gRPC-Web + plain HTTP/JSON via Vanguard transcoding), backed by PostgreSQL through GORM, with the schema managed independently via raw SQL migrations (`golang-migrate`).

## Commands

Local infra and DB (via `make`, reads `.env` automatically):

```
make up              # docker compose up --build, then waits and runs pending migrations
make down             # docker compose down
make migrate-up       # apply all pending migrations (runs against the torogan-network docker network)
make migrate-down     # roll back all migrations
make migrate-create name=<name>   # scaffold a new migrations/<seq>_<name>.up.sql / .down.sql pair
make reset-db          # docker compose down -v, rebuild, and re-run migrations from scratch
make proto-gen         # buf generate — regenerate gen/ from proto/
```

There is currently no test suite in the repo — `gorm.io/gorm` and `testify` are present as transitive deps only. The `backend` container hot-reloads via `air`, so container code changes need no rebuild — but a `docker compose up --build` is needed after dependency changes since the image bakes in `go mod download`.

## Proto / codegen workflow

- Source protos live in `proto/` (`auth.proto`, `property.proto`), using `buf` (`buf.yaml`, `buf.gen.yaml`) with the `googleapis` dependency for `google.api.http` annotations (REST-style transcoding).
- Generated Go + Connect code is written to `gen/<service>v1/` and `gen/<service>v1/<service>v1connect/` — **do not hand-edit anything under `gen/`**; change the `.proto` and run `make proto-gen`.
- Every RPC method declares a `google.api.http` option mapping it to a REST path (e.g. `POST /v1/auth/login`, `GET /v1/properties/{id}`). Vanguard (`connectrpc.com/vanguard`) transcodes these at runtime in `cmd/server/main.go`, so each RPC is reachable both as Connect/gRPC and as plain REST/JSON on the same mux.

## Architecture

Request flow: `gen/<x>v1connect` (Connect service interface) → `pkg/handlers/` (Connect request/response, proto <-> domain model mapping, error code translation) → `pkg/services/` (business logic, GORM queries, transactions) → `internal/models/` (GORM structs) → Postgres.

- **`internal/database/init.go`** — single package-level `*gorm.DB` (`ConnectDB()` / `GetDB()`), configured from env vars with sane local-dev defaults (`DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DB_SSLMODE`).
- **`internal/models/`** — GORM models with explicit `TableName()` overrides and `gorm` struct tags mirroring the SQL migrations (these are two separate sources of truth — a schema change requires updating both the migration SQL and the corresponding model's tags).
- **`pkg/services/`** — one service struct per domain (`AuthService`, `PropertyService`, `FeatureService`, `AddressService`), each holding the `*gorm.DB` injected via its constructor. Services return plain Go/model types and errors (`gorm.ErrRecordNotFound` passes through untouched), not proto types or Connect errors — that translation happens in handlers.
- **`pkg/handlers/`** — implements the generated Connect service interfaces (e.g. `propertyv1connect.PropertyServiceHandler`). Responsible for parsing/validating request fields (UUID parsing, numeric parsing of the string-typed `price` field), calling the service layer, mapping domain models back to proto messages (`mapToProto`), and wrapping errors in `connect.NewError` with the appropriate `connect.Code*`.
- **`pkg/utils.go`** — package `pkg` (imported as `utils`), currently just `GetEnv(key, default)`.
- **`cmd/server/main.go`** — wires DB connection, constructs services, builds Connect handlers, wraps them in a `vanguard.NewTranscoder` for REST/gRPC/gRPC-Web multiplexing on one `http.ServeMux`, and serves over h2c (HTTP/2 without TLS) so gRPC works without a reverse proxy in local dev.

## Auth

- Access tokens are HS256 JWTs (`golang-jwt/jwt/v5`) signed with `JWT_SECRET`, containing `sub` (user ID), `role`, `iat`, `exp` (15 min expiry).
- Refresh tokens are **stateless** HS256 JWTs (7-day expiry, `typ:"refresh"` + `jti` claims, same `JWT_SECRET`) set as an `HttpOnly`, `Secure`, `SameSite=Strict` cookie (`setRefreshCookie` in `pkg/handlers/auth.go`). The `RefreshToken` RPC reads that cookie, validates via `ValidateRefreshToken`, and mints a new access token. Nothing is persisted server-side, so a refresh token cannot be revoked before expiry — the `jti` claim exists to make a future Redis/DB denylist cheap.
- Passwords are hashed with bcrypt (`DefaultCost`); OAuth users have no password (`users.password` is nullable).
- Roles (`admin`, `user`) live in a separate `roles` table (see `internal/models/role.go`, migration `000001_roles`); `Register` looks up the `user` role by name and assigns it as the new user's `RoleID`.
- `SignInWithGoogle` verifies the id_token with `google.golang.org/api/idtoken` against `GOOGLE_CLIENT_ID` (no client secret needed), find-or-creates the user, seeds the `google` row in `auth_providers` **in code** (no migration), and upserts `user_auth_providers` with the Google `sub`. It cannot be tested via Bruno/curl — it needs a genuine Google-issued id_token.

## Database migrations

Raw SQL migrations in `internal/database/migrations/`, numbered sequentially (`000001_roles`, ..., `000008_addresses`), each with matching `.up.sql`/`.down.sql`, applied via the `migrate/migrate` Docker image (see `make migrate-*` targets — no local `migrate` CLI install needed). Migration order encodes FK dependencies (e.g. `roles` before `users`, `users`/`properties` before their join/reference tables).

## Environment

Config is loaded from a `.env` file at the repo root (`godotenv`, falls back to real env vars if absent) — see `.env.example` for the full set of variables (DB, Redis, JWT, CORS, Google OAuth). `make up`/`make migrate-*` also read `.env` directly to build the migration DB URL against the `torogan-network` Docker network.
