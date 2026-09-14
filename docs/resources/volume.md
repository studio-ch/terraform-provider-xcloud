# `xcloud_volume`

Persistent data volume. Sizes can grow in place; shrinking is rejected. Import with the volume UUID.

## Example

```hcl
resource "xcloud_volume" "data" {
  region_id = data.xcloud_region.selected.id
  name      = "build-cache"
  size_gib  = 100
}
```

Use a separate `xcloud_volume_attachment` to attach this volume to a Linux instance. Shrinking is rejected during planning. Renaming or moving regions requires replacement; resizing grows in place. Attached volumes must be detached before deletion. The reported attachment state can change independently through attachment resources.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `name` | `string` | Required | Display name. Renaming requires replacement. |
| `region_id` | `string` | Required | Region UUID. |
| `size_gib` | `number` | Required |  |
| `state` | `string` | Read-only | Actual attachment state. |

## Import

```sh
terraform import xcloud_volume.existing 11111111-1111-4111-8111-111111111111
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
