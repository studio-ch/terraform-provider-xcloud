# `xcloud_volume_attachment`

Attaches a data volume to an instance. Attaching restarts the VM. Import: instance-uuid/volume-uuid.

## Example

```hcl
resource "xcloud_volume_attachment" "data" {
  instance_id = xcloud_instance.server.id
  volume_id   = xcloud_volume.data.id
}
```

The instance and volume must belong to the same region. Data volumes require Linux guests. Attach restarts the VM; the provider polls until attachment completes. Destroy detaches and waits, preserving the volume itself. Use resource references for both IDs so Terraform can order teardown correctly.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `instance_id` | `string` | Required | Target instance UUID. |
| `volume_id` | `string` | Required | Volume UUID in the same region. |

## Import

```sh
terraform import xcloud_volume_attachment.existing 11111111-1111-4111-8111-111111111111/33333333-3333-4333-8333-333333333333
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
