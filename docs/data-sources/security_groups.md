# `xcloud_security_groups`

Read existing Xcloud security_groups through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `items` | `list(object)` | Read-only | Matching public API objects, ordered by identifier/name. response_json contains the complete public DTO. |
| `region_id` | `string` | Optional | Region filter. Required for a lookup by region-local name. |

### `items` fields

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `description` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `name` | `string` | Read-only |  |
| `region_id` | `string` | Read-only |  |
| `response_json` | `string` | Read-only |  |

[Provider setup and lifecycle details](../../README.md)
