# `xcloud_volume`

Read existing Xcloud volumes through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `attached_instance_id` | `string` | Read-only |  |
| `id` | `string` | Required | Exact resource id. |
| `name` | `string` | Read-only |  |
| `region_id` | `string` | Optional / default or computed | Region filter. Required for a lookup by region-local name. |
| `response_json` | `string` | Read-only |  |
| `size_gib` | `number` | Read-only |  |
| `state` | `string` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
