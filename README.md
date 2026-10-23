# Backend Golang Coding Test

Solution for the [7-solutions backend challenge](https://github.com/7-solutions/backend-challenge).

| Part | Where |
| --- | --- |
| 1. User Management API (Go, Gin, MongoDB, JWT, gRPC) | this repository |
| 2. Lottery Search System (design only) | [docs/lottery-search-design.th.md](docs/lottery-search-design.th.md) |

## Features

- REST API with Gin: register/login and user CRUD, documented with **Swagger** at `/api-docs`
- MongoDB persistence through the official driver (`go.mongodb.org/mongo-driver/v2`), with a unique index on `email`
- **Versioned database migrations, applied automatically on every deploy** (see [Database migrations](#database-migrations))
- Passwords hashed with bcrypt and never returned in responses
- JWT (HS256) auth middleware: pins the algorithm and checks the issuer and expiry
- Structured access log (method, route, status, `latency_ms`, `request_id`, `user_id`) and panic recovery
- A background goroutine logs the total number of users every 10 seconds
- Tests use the standard `testing` package only:
  - unit tests with mocks/fakes in `internal/mocks`
  - repository and component tests against real MongoDB through **testcontainers**
- A Postman collection with assertions (`make postman`)

**Bonus items (all done):**

- Docker and docker-compose
- Interfaces for every layer
- Input validation
- Graceful shutdown with `context.Context`
- gRPC `CreateUser`/`GetUser`, secured with token metadata
- Hexagonal architecture (ports & adapters) with the dependency rule enforced by a test (see [Architecture](#architecture-hexagonal-ports--adapters))

## Architecture: Hexagonal (Ports & Adapters)

The code is split into the three layers the challenge asks for: **domain, application and infrastructure**. Business logic is decoupled from frameworks and drivers.

```
   HTTP (Gin) ─┐                                              ┌─► MongoDB
   gRPC ───────┤   infrastructure  ──►  application  ──►  domain   │
   timer ──────┘   (adapters)           (use cases +      (rules)  ├─► bcrypt
                                         ports.go)                 └─► JWT
                   └──────────── dependencies point inward ───────────┘
```

| Layer | Folder | What lives there | May import |
| --- | --- | --- | --- |
| **Domain** | `internal/domain` | `User`, validation and patch rules, error codes | standard library only |
| **Application** | `internal/application` | use cases (`UserService`, `AuthService`) + **`ports.go`**, every interface in one file | domain only |
| **Infrastructure** | `internal/infrastructure/*` | adapters: `httpapi`, `grpcapi`, `scheduler`, `mongodb`, `security` | application, domain, frameworks; never another adapter |

**[`internal/application/ports.go`](internal/application/ports.go) is the boundary of the hexagon.** It has two kinds of interface:

- **Driving ports**: what the application *offers* (`UserService`, `AuthService`). HTTP, gRPC and the scheduler call them.
- **Driven ports**: what the application *needs* (`UserRepository`, `PasswordHasher`, `TokenManager`). MongoDB and security implement them.

**The dependency rule is enforced by a test.** [`internal/architecture_test.go`](internal/architecture_test.go) fails if `domain` imports anything outside the standard library, if `application` imports anything but `domain`, or if one adapter imports another.

What this means in the code:

- **The application owns its interfaces.** `UserRepository` is declared in `application`. MongoDB adapts to it, not the other way round.
- **HTTP status codes live in `httpapi`.** The domain only has error codes. `httpapi` maps them to HTTP statuses, and `grpcapi` maps them to gRPC codes.
- **Authentication is a use case.** The HTTP middleware and the gRPC interceptor call `AuthService.VerifyAccessToken`. Neither knows JWT exists.
- **The scheduler is just another adapter.** It calls the `CountUsers` use case on a timer instead of on a request.
- **The authenticated user is an explicit argument** (`PatchUserByID(ctx, actorID, id, patch)`). The application never reads HTTP headers or context.
- **Application tests use only fakes**: no bcrypt, JWT or MongoDB.

## Project structure

```
cmd/api/main.go                 # entrypoint: config, logger, DB, servers, scheduler, graceful shutdown
cmd/migrate/                    # migration CLI: up / down / status / create
config/                         # config.yaml + loader (viper; env vars override YAML keys)
migrations/                     # versioned MongoDB migrations (JSON), embedded into the binaries
proto/user/v1/                  # user.proto + generated gRPC code (make proto)
docs/                           # generated Swagger (make swag) + Lottery design doc
internal/
  domain/                       # ① DOMAIN: User, rules, error codes
  application/                  # ② APPLICATION
    ports.go                    #    every interface (driving + driven) in one place
    user_service.go             #    user use cases
    auth_service.go             #    register / login / verify token
  infrastructure/               # ③ INFRASTRUCTURE (adapters)
    httpapi/                    #    REST with Gin: router, middleware, handlers, DTOs, error mapping
    grpcapi/                    #    gRPC server + interceptors
    scheduler/                  #    logs the user count every 10 s
    mongodb/                    #    UserRepository on MongoDB
    security/                   #    bcrypt (PasswordHasher), JWT (TokenManager)
  container/                    # composition root: plugs infrastructure into the application
  mocks/                        # fakes and mocks of the ports, for tests
  architecture_test.go          # enforces the dependency rule
pkg/                            # small app-agnostic helpers: database (client + migrator), logger, requestctx, bearer
postman/                        # Postman collection + local environment
test/component/                 # end-to-end tests (real router + MongoDB)
test/testutil/                  # MongoDB testcontainer helper
```

**One request, file by file** (`PATCH /api/v1/users/123`):

```
httpapi/router.go → httpapi/middleware.go (Auth) → httpapi/user_handler.go
  → application/user_service.go → domain/user.go (ApplyPatch)
  → [application.UserRepository] → mongodb/user_repository.go → MongoDB
```

## Running

### With Docker (recommended)

```bash
make up          # docker compose up --build -d
```

| What | Where |
| --- | --- |
| REST | `http://localhost:8080` |
| Swagger UI | `http://localhost:8080/api-docs` |
| gRPC | `localhost:50051` |

Use a strong secret instead of the development default:

```bash
JWT_SECRET=$(openssl rand -base64 48) docker compose up --build -d
```

### Locally

Requires Go 1.26+ and Docker for MongoDB.

```bash
make run         # starts mongo in Docker, then go run ./cmd/api with a dev JWT secret
```

In VS Code, choose **Run and Debug → "API: debug"**. MongoDB starts automatically.

### Configuration

Settings come from [`config/config.yaml`](config/config.yaml). Every key can be overridden by an environment variable: replace `.` with `_` and use upper case.

| Key (env var) | Default | Description |
| --- | --- | --- |
| `jwt.secret` (`JWT_SECRET`) | – **required** | HMAC key, at least 32 bytes |
| `jwt.issuer` (`JWT_ISSUER`) | `backend-challenge` | `iss` claim, checked on validation |
| `jwt.ttl` (`JWT_TTL`) | `24h` | Token lifetime |
| `server.http_port` (`SERVER_HTTP_PORT`) | `8080` | REST port |
| `server.grpc_port` (`SERVER_GRPC_PORT`) | `50051` | gRPC port |
| `server.shutdown_timeout` | `10s` | Maximum time allowed for graceful shutdown |
| `database.uri` (`DATABASE_URI`) | `mongodb://localhost:27017` | MongoDB connection string |
| `database.name` (`DATABASE_NAME`) | `user_management` | Database name |
| `database.auto_migrate` (`DATABASE_AUTO_MIGRATE`) | `true` | Apply pending migrations every time the API starts |
| `worker.user_count_interval` | `10s` | Interval for the background user-count log |
| `log.level` / `log.format` | `info` / `json` | `debug`…`error` / `json`, `text` |
| `app.env` (`APP_ENV`) | `local` | Any value other than `local` switches Gin to release mode |

### Database migrations

The schema lives in [`migrations/`](migrations), as versioned pairs of JSON files. Each file is a list of MongoDB commands:

```
migrations/000001_create_users_email_index.up.json     → createIndexes users { email: 1 } unique
migrations/000001_create_users_email_index.down.json   → dropIndexes users uniq_email
```

**Migrations run automatically on every deploy.** With `database.auto_migrate: true` (the default), the API applies all pending migrations when it starts, before it serves traffic:

```json
{"msg":"running database migrations"}
{"msg":"migration applied","version":1,"description":"create users email index"}
{"msg":"database schema is up to date","applied":1}
```

On the next deploy nothing is pending, so the log shows `"applied":0`.

The migrator ([`pkg/database/migrator.go`](pkg/database/migrator.go)) is built to be safe for production:

| Concern | How it is handled |
| --- | --- |
| **History** | One document per applied migration in `schema_migrations` (version, description, checksum, applied_at, execution_ms) |
| **Several replicas start at once** | A lock document (`schema_migrations_lock`) makes them run one at a time. A crashed holder's lock expires after 5 minutes. |
| **Someone edits an applied migration** | Checksums are compared. The API refuses to start and asks for a new migration instead. |
| **A migration fails half way** | It is not recorded, so the next deploy retries it. MongoDB cannot run index commands inside a transaction, so migrations must be safe to repeat (`createIndexes` is). |
| **Rollback** | Every migration has a `.down.json` file |

Run them by hand (needs MongoDB, e.g. `make db`):

```bash
make migrate-status                       # applied + pending
make migrate-up                           # apply pending
make migrate-down N=1                     # roll back the latest one
make migrate-create NAME=add_phone_index  # new 00000N_add_phone_index.{up,down}.json
```

Set `DATABASE_AUTO_MIGRATE=false` to run migrations as a separate deployment step instead, for example with `/app/migrate up` (the binary ships in the image) as a Kubernetes Job or init container.

## Testing

```bash
make test-unit       # unit tests only, no Docker (-short)
make test            # everything; repository + component tests start MongoDB with testcontainers
make cover           # coverage
make postman         # run the Postman collection against a running service
```

Set `TEST_MONGO_URI=mongodb://localhost:27017` to use an existing MongoDB instead of starting a container.

| Package | How it is tested |
| --- | --- |
| `domain` | Validation, patch rules and error codes (table-driven, no dependencies) |
| `application` | Every use case against **fakes only**: in-memory repository (unique index included), fake hasher and fake token manager |
| `infrastructure/httpapi` | Handlers in isolation with `mocks.UserService` / `mocks.AuthService`; exact JSON envelope; middleware; status mapping |
| `infrastructure/grpcapi` | Real gRPC over `bufconn`; status codes and auth interceptor |
| `infrastructure/scheduler` | Ticker, error handling and stopping on context cancellation |
| `infrastructure/mongodb` | Against **real MongoDB** (testcontainers): duplicate keys, invalid IDs, not found |
| `pkg/database` | Migration files load and validate. Against real MongoDB: up/down, re-run is a no-op, edited migrations are refused, failed ones are not recorded, **5 concurrent instances apply each migration exactly once**, an expired lock is taken over |
| `infrastructure/security` | JWT round trip; rejects `alg:none`, HS512, a wrong key, expired tokens, a wrong issuer, tampered tokens; bcrypt |
| `internal` (architecture) | The dependency rule: inner layers never import outer ones, adapters never import each other |
| `test/component` | Full stack over HTTP against real MongoDB |

**How MongoDB is mocked:** use cases depend only on the `application.UserRepository` interface. `mocks.UserRepositoryFake` implements it in memory and behaves like the MongoDB repository, including the unique-email constraint. Its `Err` field simulates database outages. The real repository is covered separately by testcontainers.

## API

All responses use the same envelope:

```json
{ "successful": true, "error_code": "", "data": { ... } }
{ "successful": false, "error_code": "EMAIL_ALREADY_EXISTS", "data": null }
```

| Method | Path | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | – | Register a user |
| `POST` | `/api/v1/auth/login` | – | Log in and get a JWT |
| `POST` | `/api/v1/users` | JWT | Create a user |
| `GET` | `/api/v1/users` | JWT | List all users |
| `GET` | `/api/v1/users/me` | JWT | Get the current user |
| `GET` | `/api/v1/users/{id}` | JWT | Get a user by ID |
| `PATCH` | `/api/v1/users/{id}` | JWT (owner only) | Update name and/or email |
| `DELETE` | `/api/v1/users/{id}` | JWT (owner only) | Delete the user |
| `GET` | `/health` | – | Health check |
| `GET` | `/api-docs` | – | Swagger UI |

| HTTP | `error_code` |
| --- | --- |
| 400 | `INVALID_REQUEST_BODY`, `INVALID_USER_ID`, `INVALID_NAME`, `INVALID_EMAIL`, `INVALID_PASSWORD` |
| 401 | `UNAUTHORIZED` (missing or invalid token), `INVALID_CREDENTIALS` |
| 403 | `FORBIDDEN` (modifying another user's account) |
| 404 | `USER_NOT_FOUND` |
| 409 | `EMAIL_ALREADY_EXISTS` |
| 500 | `INTERNAL_SERVER_ERROR` (the cause is logged server-side and never returned) |

## JWT guide

### 1. Get a token

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -d '{"name":"Alice","email":"alice@example.com","password":"password123"}'

curl -s -X POST localhost:8080/api/v1/auth/login \
  -d '{"email":"alice@example.com","password":"password123"}'
```

```json
{
  "successful": true,
  "error_code": "",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJiYWNrZW5kLWNoYWxsZW5nZSIsInN1YiI6IjZhZDgz...",
    "token_type": "Bearer",
    "expires_at": "2026-10-22T04:24:13Z"
  }
}
```

### 2. Use the token

```bash
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -d '{"email":"alice@example.com","password":"password123"}' | jq -r .data.access_token)

curl -s localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

- **Swagger UI:** click **Authorize** and enter `Bearer <token>`.
- **Postman:** the collection saves the token from the Login request automatically.
- **gRPC:** send the token as metadata:

  ```bash
  grpcurl -plaintext -H "authorization: Bearer $TOKEN" \
    -d '{"id":"<user-id>"}' localhost:50051 user.v1.UserService/GetUser
  ```

### 3. What's inside the token

Header `{"alg":"HS256","typ":"JWT"}`, payload:

```json
{ "iss": "backend-challenge", "sub": "<user id>", "iat": 1792556653, "nbf": 1792556653, "exp": 1792643053 }
```

When a token is validated, the server checks:

- the HMAC-SHA256 signature
- that the algorithm is exactly `HS256`, which blocks `alg:none` and algorithm-confusion attacks
- that `exp` is present and in the future, and that `nbf` has passed
- that `iss` matches
- that `sub` is not empty

## Sample requests & responses

<details>
<summary>Register — <code>201 Created</code></summary>

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -d '{"name":"Alice","email":"Alice@Example.com","password":"password123"}'
```
```json
{"successful":true,"error_code":"","data":{"id":"6ad83e6db6f99c0a372a5356","name":"Alice","email":"alice@example.com","created_at":"2026-10-21T04:24:13.532Z"}}
```
Registering the same email again (case-insensitive) returns `409 {"successful":false,"error_code":"EMAIL_ALREADY_EXISTS","data":null}`.
</details>

<details>
<summary>List users — <code>200 OK</code></summary>

```bash
curl -s localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```
```json
{
  "successful": true,
  "error_code": "",
  "data": {
    "users": [
      {"id":"6ad83e6db6f99c0a372a5356","name":"Alice","email":"alice@example.com","created_at":"2026-10-21T04:24:13.532Z"},
      {"id":"6ad83e6db6f99c0a372a5358","name":"Bob","email":"bob@example.com","created_at":"2026-10-21T04:24:13.911Z"}
    ],
    "total": 2
  }
}
```
</details>

<details>
<summary>Get user — <code>200</code> / <code>400</code> / <code>404</code></summary>

```bash
curl -s localhost:8080/api/v1/users/6ad83e6db6f99c0a372a5356 -H "Authorization: Bearer $TOKEN"
```
```json
{"successful":true,"error_code":"","data":{"id":"6ad83e6db6f99c0a372a5356","name":"Alice","email":"alice@example.com","created_at":"2026-10-21T04:24:13.532Z"}}
```
- `/users/xyz` returns `400 INVALID_USER_ID`.
- An unknown ID returns `404 USER_NOT_FOUND`.
</details>

<details>
<summary>Update user — <code>200 OK</code></summary>

```bash
curl -s -X PATCH localhost:8080/api/v1/users/6ad83e6db6f99c0a372a5356 \
  -H "Authorization: Bearer $TOKEN" -d '{"name":"Alicia"}'
```
```json
{"successful":true,"error_code":"","data":{"id":"6ad83e6db6f99c0a372a5356","name":"Alicia","email":"alice@example.com","created_at":"2026-10-21T04:24:13.532Z"}}
```
- Another user's ID returns `403 FORBIDDEN`.
- An empty body `{}` returns `400 INVALID_REQUEST_BODY`.
</details>

<details>
<summary>Delete user — <code>200 OK</code></summary>

```bash
curl -s -X DELETE localhost:8080/api/v1/users/6ad83e6db6f99c0a372a5356 -H "Authorization: Bearer $TOKEN"
```
```json
{"successful":true,"error_code":""}
```
</details>

Logs are JSON lines (`slog`). Each request gets one access-log line, and the worker logs a user count every 10 seconds:

```json
{"level":"INFO","msg":"http request","service":"user-service","method":"DELETE","route":"/api/v1/users/:id","status":200,"latency_ms":1.159,"request_id":"70418c22...","user_id":"6ad8a1eb..."}
{"level":"INFO","msg":"user count","service":"user-service","layer":"worker","total_users":1}
```

## gRPC

The service is defined in [`proto/user/v1/user.proto`](proto/user/v1/user.proto) and provides `CreateUser` and `GetUser`.

- Server reflection is enabled, so `grpcurl list` works.
- Every RPC requires `authorization: Bearer <jwt>` metadata, otherwise it returns `UNAUTHENTICATED`.
- Errors return the same `error_code` as the status message, with these status codes: `INVALID_ARGUMENT`, `NOT_FOUND`, `ALREADY_EXISTS`, `UNAUTHENTICATED`.
- The generated `*.pb.go` files sit next to `user.proto`. Never edit them by hand; regenerate with `make proto` after changing the `.proto`.

## Assumptions & design decisions

- **Register vs. create user.** Both share the `UserService.CreateUser` use case. `POST /auth/register` is public so new users can sign up. `POST /users` is the protected "create user" operation from the spec.
- **Authorization.** There are no roles in the spec, so any authenticated user can read or list users, but only the **account owner** can update or delete it (`403 FORBIDDEN` otherwise).
- **Updates** use `PATCH` with optional `name` and `email`. Passwords cannot be changed this way, and unknown fields are rejected.
- **Email is unique, case-insensitively.** Emails are trimmed and lowercased. A MongoDB unique index (migration `000001`) enforces uniqueness, which avoids the race in "check then insert", and duplicates map to `409`.
- **Validation:**
  - name: required, at most 100 characters
  - email: bare RFC 5322 address with a dotted domain, at most 254 characters
  - password: 8 to 72 bytes (72 is bcrypt's input limit)
  - JSON bodies are limited to 1 MiB, and unknown fields and trailing data are rejected
- **Login errors** are the same for an unknown email and a wrong password, to avoid revealing which emails exist.
- **JWT.** HS256 with a secret of at least 32 bytes, validated at startup (no insecure fallback). Tokens are stateless with a 24h default. There is no refresh or revocation, so a deleted user's token stays valid until it expires but can't modify anything. A production system would use short-lived access tokens plus refresh tokens or a denylist.
- **The list endpoint** returns all users, as the spec asks. In production it should be paginated (cursor on `_id`).
- **Graceful shutdown.** On SIGINT/SIGTERM the server:
  - cancels the root context, which stops the worker
  - drains HTTP (`Shutdown`) and gRPC (`GracefulStop`, falling back to `Stop` after `shutdown_timeout`)
  - waits for the worker, then disconnects from MongoDB
- **Tests use the standard `testing` package only,** as the challenge requires, so there is no testify. Mocks are hand-written in `internal/mocks`.
- **The container** runs as non-root on a distroless static image.
