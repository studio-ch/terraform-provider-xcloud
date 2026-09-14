# `xcloud_network`

Tenant network. The public API has no update operation; changes replace it. Import: region-uuid/name.

## Example

```hcl
resource "xcloud_network" "private" {
  region_id = data.xcloud_region.selected.id
  name      = "private"
  mode      = "nat"
  cidr      = "10.42.0.0/24"
  gateway   = "10.42.0.1"
  dhcp      = true
  labels    = { environment = "development" }
}
```

Choose a network mode/spec supported by your region. The API has no network update endpoint, so configured changes replace the network. If a network disappears from the list, refresh returns an error because regional outages can also produce an empty list; state is retained. Only after confirming deletion, remove the resource from state before recreating it. Cluster default networks are referenced by name in `network_ref` and are not managed here.

Alternatively, use `spec_json = jsonencode({...})` for arbitrary spec fields;
`effective_spec_json` shows the full spec including defaults. Configure each
property either in JSON or through the flat arguments. For VMs that must be
replaced when this network changes but keeps its name, set
`lifecycle { replace_triggered_by = [xcloud_network.private] }` on the VM.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `cidr` | `string` | Optional / default or computed | Subnet CIDR. |
| `dhcp` | `bool` | Optional / default or computed |  |
| `effective_spec_json` | `string` | Read-only | Complete network spec returned by the API, including defaults. |
| `gateway` | `string` | Optional / default or computed | Gateway address. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `labels` | `map(string)` | Optional / default or computed |  |
| `mode` | `string` | Optional / default or computed | Network mode. |
| `name` | `string` | Required | Region-local resource name. Changes replace the resource. |
| `region_id` | `string` | Required | Region UUID. |
| `spec_json` | `string` | Optional | Arbitrary network spec as a JSON object (use jsonencode). Changes replace the network. Do not configure the same property via a flat argument. |

## Import

```sh
terraform import xcloud_network.existing 11111111-1111-4111-8111-111111111111/private
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
