# `xcloud_network`

Read existing Xcloud networks through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `cidr` | `string` | Read-only |  |
| `dhcp` | `bool` | Read-only |  |
| `gateway` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `labels` | `map(string)` | Read-only |  |
| `mode` | `string` | Read-only |  |
| `name` | `string` | Required | Exact resource name. |
| `region_id` | `string` | Required | Region filter. Required for a lookup by region-local name. |
| `response_json` | `string` | Read-only |  |
| `source` | `string` | Read-only |  |

[Provider setup and lifecycle details](../../README.md)
