# `xcloud_flavor`

Look up an Xcloud flavor through the public catalog API. Missing or ambiguous results produce an error.

## Example

```hcl
data "xcloud_flavor" "selected" {
  region_id = data.xcloud_region.selected.id
  slug      = var.flavor_slug
}
```

The lookup is scoped to the selected region and tenant. Missing or ambiguous matches fail. Pass the returned sizing and platform together with `flavor_slug` to the instance resource; the API resolves the flavor authoritatively.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `cpu_cores` | `number` | Read-only |  |
| `disk_gib` | `number` | Read-only |  |
| `hardware_generation` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `label` | `string` | Read-only |  |
| `memory_gib` | `number` | Read-only |  |
| `platform` | `string` | Read-only |  |
| `region_id` | `string` | Required | Region UUID. Limits the catalog lookup to this region. |
| `response_json` | `string` | Read-only | Complete public catalog DTO as JSON, including availability, pricing and other metadata exposed by the API. |
| `slug` | `string` | Required | Exact catalog slug. |

[Provider setup and lifecycle details](../../README.md)
