# Authentication

## Overview

The system uses username/password authentication with JWT bearer tokens. Users register or log in via public API endpoints, receive a JWT token, and include it in subsequent requests.

## Authentication Flow

1. User calls `POST /api/auth/register` or `POST /api/auth/login` with credentials
2. Server validates credentials and returns a JWT token
3. Client stores the token and sends it as `Authorization: Bearer <token>` on all subsequent API requests
4. The JWT middleware validates the token on every protected request and extracts the user identity

## JWT Token

- **Signing method:** HMAC-SHA256
- **Secret:** Configured via `JWT_SECRET` environment variable
- **Expiry:** 24 hours from issuance
- **Claims:**

| Claim | Type | Description |
|---|---|---|
| `user_id` | `float64` | The user's database ID |
| `username` | `string` | The user's username |
| `exp` | `int64` | Unix timestamp of token expiry |

## API Endpoints

### `POST /api/auth/register`

Creates a new user account and returns a JWT token.

**Request body:**

```json
{
  "username": "alice",
  "password": "securepassword",
  "display_name": "Alice Smith"
}
```

- `username` — required, must be unique
- `password` — required, minimum 8 characters
- `display_name` — optional

**Success response (201):**

```json
{
  "token": "eyJhbG...",
  "user": {
    "id": 1,
    "username": "alice",
    "display_name": "Alice Smith",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**Error responses:**
- `400` — missing/invalid fields or password too short
- `409` — username already taken

### `POST /api/auth/login`

Authenticates an existing user and returns a JWT token.

**Request body:**

```json
{
  "username": "alice",
  "password": "securepassword"
}
```

**Success response (200):**

```json
{
  "token": "eyJhbG...",
  "user": {
    "id": 1,
    "username": "alice",
    "display_name": "Alice Smith",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

**Error responses:**
- `400` — invalid request body
- `401` — invalid username or password

## Password Storage

Passwords are hashed using bcrypt with the default cost factor before storage. The `password_hash` column in the `users` table stores the bcrypt hash. Plaintext passwords are never stored or logged.

## Admin User

A superuser admin account is automatically created on server startup when the following environment variables are set:

| Variable | Description |
|---|---|
| `ADMIN_USERNAME` | Username for the admin account |
| `ADMIN_PASSWORD` | Password for the admin account |

The seeding is idempotent — if a user with the given username already exists, no changes are made.

In the default `docker-compose.yml`, the admin credentials are `admin` / `admin`.

## Superuser

Users with the `superuser` flag set to `true` bypass all permission checks. This flag is only set via the admin seeding mechanism (not exposed through the API). Superusers can perform any action on any resource without needing explicit role assignments.

## Protected Routes

All routes under `/api/*` (except `/api/auth/*`) require a valid JWT token. Requests without a token or with an expired/invalid token receive a `401 Unauthorized` response.
