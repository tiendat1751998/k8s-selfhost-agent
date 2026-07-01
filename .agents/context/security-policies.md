# Security Policies

## Authentication

### JWT Token Authentication
- All `/api/v1/*` routes require `Authorization: Bearer <jwt-token>` header.
- WebSocket connections accept token via query parameter: `GET /ws?token=<jwt>`.
- JWT payload contains: `sub` (user ID), `role` (RBAC role), `tenant` (tenant ID).
- Token is decoded from Base64 URL-encoded JWT payload segment.
- Invalid or missing tokens return `401 Unauthorized`.

### Login Endpoint
- `POST /api/v1/auth/login` — unauthenticated.
- Password hashing: **bcrypt** (cost 10).
- Default admin: `admin@k8sselfhost.local` / `admin`.

---

## Authorization (RBAC)

### Roles
| Role | Permissions | Scope |
|------|-------------|-------|
| `platform_admin` | `["*"]` | Full access to all resources |
| `tenant_admin` | `["tenant:*"]` | Full access within tenant scope |
| `viewer` | `["tenant:read"]` | Read-only within tenant |

### Multi-Tenancy
- Every request is scoped by `tenant_id` from JWT claims.
- `tenant_bindings` table maps users to tenants with specific roles.
- RBAC middleware (`RBACMiddleware`) enforces role requirements per route.

---

## Encryption

### AES-256-GCM
- Cluster credentials (kubeconfig tokens) are encrypted with **AES-256-GCM**.
- Encryption key: 32-byte key loaded from `ENCRYPTION_KEY` environment variable.
- Encrypted data stored in `fleet_clusters.encrypted_token` column.
- Decryption happens only at point of use (never persisted in plaintext).

### Password Hashing
- User passwords hashed with **bcrypt** (cost factor 10).
- Stored in `users.password_hash` column.

---

## SQL Injection Prevention

- All database queries use **parameter-bound placeholders** (`$1, $2, $3`).
- pgx driver enforces parameterized queries natively.
- **Forbidden**: `fmt.Sprintf` or string concatenation for query building.
- All `rows` and connections closed in `defer` statements.

---

## HTTP Security Headers

Applied via `SecurityHeaders` middleware to all responses:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `X-XSS-Protection` | `1; mode=block` |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` |
| `Content-Security-Policy` | Restrictive CSP |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |

---

## CORS Policy

Applied via `CORS` middleware:
- Allowed Origins: configurable (default: `*` for development).
- Allowed Methods: `GET, POST, PUT, DELETE, OPTIONS`.
- Allowed Headers: `Authorization, Content-Type, X-Request-ID`.
- Exposed Headers: `X-Request-ID`.

---

## Secrets Management

### Environment Variables (never in source control)
| Variable | Purpose |
|----------|---------|
| `ENCRYPTION_KEY` | 32-byte AES-256 key for cluster credential encryption |
| `JWT_SECRET` | JWT signing secret |
| `GOOGLE_API_KEY` | Gemini API key for ADK agents |
| `POSTGRES_PASSWORD` | Database password |

### .gitignore Protections
- `.env` files excluded from version control.
- `config.yaml` with production credentials excluded.
- Generated media files (`.mp3`, `.srt`) excluded.

---

## Container Security (Helm)

| Control | Value |
|---------|-------|
| `runAsNonRoot` | `true` |
| `runAsUser` | `65534` (nobody) |
| `readOnlyRootFilesystem` | `true` |
| `allowPrivilegeEscalation` | `false` |
| `capabilities.drop` | `ALL` |

---

## Audit Logging

- All security and state-modifying actions are recorded in `platform_audit_logs` table.
- Audit entries capture: who (user_id/email), what (action), when (timestamp), result (success/failure).
- Audit handler: `GET /api/v1/audit` — query audit trail.
