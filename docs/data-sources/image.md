# `xcloud_image`

Look up an Xcloud image through the public catalog API. Missing or ambiguous results produce an error.

## Example

```hcl
data "xcloud_image" "selected" {
  region_id = data.xcloud_region.selected.id
  name      = var.image_name
}
```

Looks up an existing catalog image. This data source does not build, upload or register images. The match is scoped to the region. Use its `name` as the instance’s `image_ref`; `oci_reference` exposes the underlying reference when present.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Read-only |  |
| `labels` | `map(string)` | Read-only |  |
| `name` | `string` | Required | Exact catalog name. |
| `oci_reference` | `string` | Read-only |  |
| `region_id` | `string` | Required | Region UUID. Limits the catalog lookup to this region. |
| `response_json` | `string` | Read-only | Complete public catalog DTO as JSON, including availability, pricing and other metadata exposed by the API. |
| `source` | `string` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
