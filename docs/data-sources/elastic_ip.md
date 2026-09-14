# `xcloud_elastic_ip`

Read existing Xcloud elastic_ips through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Required | Exact resource id. |
| `name` | `string` | Read-only |  |
| `public_address` | `string` | Read-only |  |
| `region_id` | `string` | Optional / default or computed | Region filter. Required for a lookup by region-local name. |
| `response_json` | `string` | Read-only |  |
| `status` | `string` | Read-only |  |
| `target_instance_id` | `string` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
