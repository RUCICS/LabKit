## API versioning policy

LabKit exposes a versioned HTTP API under `/api`.

### Routing

- **Unversioned**: `/api/...` maps to **v1** for backwards compatibility.\n  This path is considered legacy and must not introduce breaking changes.
- **Explicit v1**: `/api/v1/...` is an alias of `/api/...`.\n  Clients may switch to `/api/v1` to pin behavior.
- **v2 (incremental)**: `/api/v2/...` is the new stable contract.\n  v2 is implemented endpoint-by-endpoint and may be incomplete until parity is reached.

### Contract rules

- **v1**\n  - Must remain compatible with all clients already deployed.\n  - JSON shapes are treated as part of the contract (even if historically they came from Go default encoding).\n  - Bug fixes are allowed only when they are non-breaking.
- **v2**\n  - Uses a stable, explicit JSON contract.\n  - Prefer lowercase keys and a consistent style (snake_case for multiword fields).\n  - Responses should be produced via DTOs / explicit `json:\"...\"` tags rather than relying on default Go struct field names.

### Client guidance

- **New clients** should prefer v2 where available.\n  If an endpoint is not implemented in v2 yet, clients may fall back to v1.\n
- **Existing clients** may continue using `/api/...` indefinitely, but should plan a migration to `/api/v1` (pin) or `/api/v2` (upgrade) when convenient.

### Deprecation

We may add response headers on v1 endpoints to communicate lifecycle:

- `Deprecation: true`
- `Sunset: <RFC 1123 datetime>`
- `Link: <https://.../docs/reference/api-versioning.md>; rel=\"deprecation\"`

No v1 endpoint will be removed until:\n
- v2 parity is achieved for the relevant surface area, and\n- a published migration window has elapsed.

### Known v1 vs v2 differences (example)

- `GET /api/labs/{labID}` (v1) returns the `manifest` using Go-exported keys such as `Schedule.Close`.\n- `GET /api/v2/labs/{labID}` (v2) returns the manifest using a stable lowercase contract such as `schedule.close`.

