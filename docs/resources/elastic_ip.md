# `xcloud_elastic_ip`

Reserved public IPv4 address with optional instance binding. Import with its UUID.

## Example

```hcl
resource "xcloud_elastic_ip" "server" {
  region_id   = data.xcloud_region.selected.id
  instance_id = xcloud_instance.server.id
}
```

Omit `instance_id` to reserve an unbound address. Changing the target detaches the previous instance before attaching the new one. `status` reflects the API’s stored upstream observation; successful apply confirms the recorded assignment, not live network reachability. Create/delete of the address is billable according to the API’s allocation lifecycle.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `instance_id` | `string` | Optional | Target instance UUID. Omit to leave the address unbound. |
| `public_address` | `string` | Read-only | Allocated public IPv4 address. |
| `region_id` | `string` | Required | Region UUID. |
| `status` | `string` | Read-only | Actual allocation or binding status. |

## Import

```sh
terraform import xcloud_elastic_ip.existing 11111111-1111-4111-8111-111111111111
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
