# `xcloud_security_group`

Region-local security group. Rules are ordered and replaced atomically. Import: region-uuid/name.

## Example

```hcl
resource "xcloud_security_group" "ssh" {
  region_id = data.xcloud_region.selected.id
  name      = "ssh"
  rules = [
    {
      direction = "ingress"
      protocol  = "tcp"
      cidr      = "192.0.2.10/32"
      port_from = 22
      port_to   = 22
    },
    {
      direction = "egress"
      cidr      = "0.0.0.0/0"
    }
  ]
}
```

Rules are ordered and stateless: include egress rules for reply traffic. The provider manages the entire rules list. Protocol defaults to `any`, action to `allow`, description to an empty string, and omitted ports match any port. Set both `port_from` and `port_to`, or omit both. An empty rules list explicitly removes all rules. Labels default to an empty map.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `labels` | `map(string)` | Optional / default or computed |  |
| `name` | `string` | Required | Region-local resource name. Changes replace the resource. |
| `region_id` | `string` | Required | Region UUID. |
| `rules` | `list(object)` | Required | Complete ordered rule set. An empty list removes every rule. |

### `rules` fields

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `action` | `string` | Optional / default or computed |  |
| `cidr` | `string` | Required | Source or destination CIDR. |
| `description` | `string` | Optional / default or computed |  |
| `direction` | `string` | Required | Rule direction. |
| `port_from` | `number` | Optional |  |
| `port_to` | `number` | Optional |  |
| `protocol` | `string` | Optional / default or computed |  |

## Import

```sh
terraform import xcloud_security_group.existing 11111111-1111-4111-8111-111111111111/ssh
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
