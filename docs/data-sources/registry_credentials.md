# `xcloud_registry_credentials`

Read existing Xcloud registry_credentials through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `items` | `list(object)` | Read-only | Matching public API objects, ordered by identifier/name. response_json contains the complete public DTO. |

### `items` fields

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `display_name` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `registry_url` | `string` | Read-only |  |
| `response_json` | `string` | Read-only |  |
| `source` | `string` | Read-only |  |
| `username` | `string` | Read-only |  |

[Provider setup and lifecycle details](../../README.md)
