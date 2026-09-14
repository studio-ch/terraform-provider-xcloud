# `xcloud_instances`

Read existing Xcloud instances through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `items` | `list(object)` | Read-only | Matching public API objects, ordered by identifier/name. response_json contains the complete public DTO. |
| `region_id` | `string` | Optional | Region filter. Required for a lookup by region-local name. |

### `items` fields

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `cpu_cores` | `number` | Read-only |  |
| `disk_gib` | `number` | Read-only |  |
| `expires_at` | `string` | Read-only |  |
| `flavor_slug` | `string` | Read-only |  |
| `hardware_generation` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `image_ref` | `string` | Read-only |  |
| `memory_gib` | `number` | Read-only |  |
| `name` | `string` | Read-only |  |
| `network_address` | `string` | Read-only |  |
| `network_ref` | `string` | Read-only |  |
| `platform` | `string` | Read-only |  |
| `region_id` | `string` | Read-only |  |
| `response_json` | `string` | Read-only |  |
| `status` | `string` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
