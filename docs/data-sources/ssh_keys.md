# `xcloud_ssh_keys`

Read existing Xcloud ssh_keys through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `items` | `list(object)` | Read-only | Matching public API objects, ordered by identifier/name. response_json contains the complete public DTO. |

### `items` fields

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `fingerprint_sha256` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `name` | `string` | Read-only |  |
| `response_json` | `string` | Read-only |  |

[Provider setup and lifecycle details](../../README.md)
