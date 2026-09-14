# `xcloud_security_group`

Read existing Xcloud security_groups through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `description` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `name` | `string` | Required | Exact resource name. |
| `region_id` | `string` | Required | Region filter. Required for a lookup by region-local name. |
| `response_json` | `string` | Read-only |  |

[Provider setup and lifecycle details](../../README.md)
