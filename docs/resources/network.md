# `xcloud_network`

Tenant network. Network settings and labels update in place. Name and region changes replace the resource. Import: region-uuid/name.

## Example

```hcl
resource "xcloud_network" "private" {
  region_id = data.xcloud_region.selected.id
  name      = "private"
  cidr      = "10.42.0.0/24"
  gateway   = "10.42.0.1"
  dhcp      = true
  dhcp_range_start = "10.42.0.100"
  dhcp_range_end   = "10.42.0.200"
  labels    = { environment = "development" }
}
```

Xcloud provides isolated private IPv4 networks with NAT egress. There is no selectable bridge/NAT/VLAN mode. Subnet and gateway are required; the subnet must be within RFC 1918 private space. DHCP requires both range endpoints within that subnet. Omit DHCP to disable it. Subnet, gateway, DHCP and label changes use PATCH; changing addresses can interrupt attached instances. If a network disappears from the list, refresh returns an error because regional outages can also produce an empty list; state is retained. Only after confirming deletion, remove the resource from state before recreating it. Cluster default networks are referenced by name in `network_ref` and are not managed here.

Alternatively, use `spec_json = jsonencode({...})` with `subnet`, `gateway` and optional `dhcp = {rangeStart = "10.42.0.100", rangeEnd = "10.42.0.200"}`;
`effective_spec_json` shows the full spec including defaults. Configure each
property either in JSON or through the flat arguments. For VMs that must be
replaced when this network changes but keeps its name, set
`lifecycle { replace_triggered_by = [xcloud_network.private] }` on the VM.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `cidr` | `string` | Optional / default or computed | Subnet CIDR. |
| `dhcp` | `bool` | Optional / default or computed | Enable DHCP; requires both range endpoints. Inferred when a range is configured. |
| `dhcp_range_start` | `string` | Optional / computed | First DHCP address. |
| `dhcp_range_end` | `string` | Optional / computed | Last DHCP address. |
| `effective_spec_json` | `string` | Read-only | Complete network spec returned by the API, including defaults. |
| `gateway` | `string` | Optional / default or computed | Gateway address. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `labels` | `map(string)` | Optional / default or computed |  |
| `mode` | `string` | Optional / default or computed | Deprecated compatibility field; remove this argument. Configuring a mode is rejected. |
| `name` | `string` | Required | Region-local resource name. Changes replace the resource. |
| `region_id` | `string` | Required | Region UUID. |
| `spec_json` | `string` | Optional | Validated upstream spec with subnet, gateway and optional DHCP range (use jsonencode). Changes update the network in place. Do not configure the same property via a flat argument. |

## Import

```sh
terraform import xcloud_network.existing 11111111-1111-4111-8111-111111111111/private
```

Define the matching resource block before import.

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
