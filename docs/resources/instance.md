# `xcloud_instance`

An Xcloud VM. Create, resize and delete wait for completion. Sizing changes cause downtime. Import with the instance UUID.

## Example

```hcl
resource "xcloud_instance" "server" {
  region_id       = data.xcloud_region.selected.id
  name            = "build-server"
  image_ref       = data.xcloud_image.selected.name
  platform        = data.xcloud_flavor.selected.platform
  flavor_slug     = data.xcloud_flavor.selected.slug
  cpu_cores       = data.xcloud_flavor.selected.cpu_cores
  memory_gib      = data.xcloud_flavor.selected.memory_gib
  disk_gib        = data.xcloud_flavor.selected.disk_gib
  ssh_key_ids     = [xcloud_ssh_key.admin.id]
  security_groups = [xcloud_security_group.ssh.name]
}
```

The defaults are `platform = "macos"`, `network_ref = "default"`, `display_width = 1920`, `display_height = 1080`, `power_state = "running"`, and empty SSH key/security group sets. The API supplies the admin username and hardware generation. Linux guests require at least one SSH key. Catalog sizing must match the flavor; use the flavor data source. CPU/RAM/disk changes stop the VM and cause downtime. Disk shrinking is rejected at plan time. The operation timeout defaults to 30 minutes. Creation waits for the VM lifecycle, not SSH-key injection or application readiness. An omitted lifetime means unlimited lifetime.

Use `tags = { environment = "development" }` for managed metadata.
`shutdown_mode` defaults to `graceful`; `hard` immediately stops the VM.
`power_state = "suspended"` saves the guest state, and `running` resumes it.
macOS guests also support `boot_into_recovery` and `admin_password`.
Passwords remain in sensitive Terraform state and are injected asynchronously.
VM deletion preserves Elastic IPs; their resource owns address release.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `admin_password` | `string` | Optional; sensitive | macOS admin password (8–128 printable ASCII characters). Stored in Terraform state. Removing this argument stops managing the password; it does not clear the guest password. |
| `admin_username` | `string` | Optional / default or computed | Guest admin user, defaulted by the image/API. |
| `boot_into_recovery` | `bool` | Optional / default or computed | macOS recovery boot. Changing this setting may restart the VM. Requires an API exposing bootIntoRecovery in the instance DTO. |
| `cpu_cores` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `disk_gib` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `display_height` | `number` | Optional / default or computed |  |
| `display_width` | `number` | Optional / default or computed |  |
| `expires_at` | `string` | Read-only | Server-enforced deletion deadline, or null. |
| `flavor_slug` | `string` | Optional / default or computed | Catalog flavor slug. CPU, memory and disk must match the flavor; this is not a sizing resolver. |
| `hardware_generation` | `string` | Optional / default or computed | Hardware generation constraint. Resolved from the flavor when flavor_slug is supplied. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `image_ref` | `string` | Required | Image name or OCI reference accepted by the public API. |
| `lifetime_seconds` | `number` | Optional | Automatic deletion deadline relative to creation. Omit for unlimited lifetime. |
| `memory_gib` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `name` | `string` | Required | Display name. |
| `network_address` | `string` | Read-only | Primary guest IP address. |
| `network_ref` | `string` | Optional / default or computed | Network name in this region. |
| `platform` | `string` | Optional / default or computed | Guest platform: macos or linux. Linux requires at least one SSH key. |
| `power_state` | `string` | Optional / default or computed | Desired power state: running, stopped or suspended. |
| `region_id` | `string` | Required | Region UUID. |
| `security_groups` | `set(string)` | Optional / default or computed | Names of security groups in the same region. |
| `shutdown_mode` | `string` | Optional / default or computed | How to stop before resize or when power_state is stopped: graceful (ACPI shutdown, default) or hard (immediate stop). |
| `ssh_key_ids` | `set(string)` | Optional / default or computed | SSH key UUIDs to inject into the guest. |
| `status` | `string` | Read-only | Actual API lifecycle status. |
| `tags` | `map(string)` | Optional / default or computed | Metadata tags. Keys: lowercase letters, digits, dots, underscores, slashes, hyphens; at most 20 tags. |

## Import

```sh
terraform import xcloud_instance.existing 11111111-1111-4111-8111-111111111111
```

Define the matching resource block before import.

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
