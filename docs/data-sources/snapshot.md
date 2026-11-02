# `xcloud_snapshot`

Read existing Xcloud snapshots through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `capture_time` | `string` | Read-only |  |
| `created_at` | `string` | Read-only |  |
| `description` | `string` | Read-only |  |
| `disk_gib` | `number` | Read-only |  |
| `id` | `string` | Required | Exact resource id. |
| `image_ref` | `string` | Read-only |  |
| `min_cpu_cores` | `number` | Read-only |  |
| `min_memory_gib` | `number` | Read-only |  |
| `name` | `string` | Read-only |  |
| `platform` | `string` | Read-only |  |
| `region_id` | `string` | Optional / default or computed | Region filter. Required for a lookup by region-local name. |
| `response_json` | `string` | Read-only |  |
| `restorable` | `bool` | Read-only |  |
| `source_instance_id` | `string` | Read-only |  |
| `source_instance_name` | `string` | Read-only |  |
| `status` | `string` | Read-only |  |
| `taken_while_running` | `bool` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
