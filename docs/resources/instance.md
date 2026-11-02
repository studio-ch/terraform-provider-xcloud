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

The defaults are `platform = "macos"`, `network_ref = "default"`, `display_width = 1920`, `display_height = 1080`, `power_state = "running"`, and empty SSH key/security group sets. The API supplies the admin username and hardware generation. New Linux guests require at least one SSH key unless a complete custom startup configuration is supplied. Catalog sizing must match the flavor; use the flavor data source. CPU/RAM/disk changes stop the VM and cause downtime. Disk shrinking is rejected at plan time. The operation timeout defaults to 30 minutes. Creation waits for the VM lifecycle, not SSH-key injection or application readiness. An omitted lifetime means unlimited lifetime.

Use `tags = { environment = "development" }` for managed metadata.
`shutdown_mode` defaults to `graceful`; `hard` immediately stops the VM.
`power_state = "suspended"` saves the guest state, and `running` resumes it.
macOS guests also support `boot_into_recovery` and `admin_password`.
Passwords remain in sensitive Terraform state and are injected asynchronously.
VM deletion preserves Elastic IPs; their resource owns address release.
Before destroying or replacing an instance, set `power_state = "stopped"` and
run `terraform apply`. Wait for that apply to finish before destroying or
replacing it. The API rejects deletion unless the instance is stopped with
no pending action (HTTP 409).

## Creating an instance from a snapshot

Set `source_snapshot_id` instead of `image_ref` to boot a copy of a ready
`xcloud_snapshot` in the same region, with a machine identity of its own.

```hcl
resource "xcloud_instance" "copy" {
  region_id          = xcloud_snapshot.baseline.region_id
  name               = "build-server-2"
  source_snapshot_id = xcloud_snapshot.baseline.id
  platform           = xcloud_snapshot.baseline.platform
  cpu_cores          = xcloud_snapshot.baseline.min_cpu_cores
  memory_gib         = xcloud_snapshot.baseline.min_memory_gib
  disk_gib           = xcloud_snapshot.baseline.disk_gib
}
```

Exactly one of `image_ref` and `source_snapshot_id` is required; with a snapshot,
`image_ref` reads the snapshot's source image. `platform` must match the
snapshot's, `disk_gib` must be at least its `disk_gib`, and `cpu_cores` and
`memory_gib` at least its minimums. The guest boots as it was when the snapshot
was taken, users and keys included, so `user_data` and `admin_username` are
rejected and Linux guests need no `ssh_key_ids`. Changing `source_snapshot_id`
replaces the VM; deleting the snapshot later does not. Destroying the instance
keeps its snapshots.

## Linux startup configuration

For Fedora CoreOS, provide a complete Ignition JSON file. Convert Butane YAML to
Ignition first. For cloud-init images, use `user_data_format = "cloud-init"` and a
YAML file beginning with `#cloud-config` instead.

```hcl
resource "xcloud_instance" "coreos" {
  region_id        = data.xcloud_region.selected.id
  name             = "coreos-01"
  image_ref        = data.xcloud_image.selected.name
  platform         = "linux"
  flavor_slug      = data.xcloud_flavor.selected.slug
  cpu_cores        = data.xcloud_flavor.selected.cpu_cores
  memory_gib       = data.xcloud_flavor.selected.memory_gib
  disk_gib         = data.xcloud_flavor.selected.disk_gib
  admin_username   = "core"
  user_data_format = "ignition"
  user_data        = file("${path.module}/config.ign")
}
```

Select a CoreOS image and Linux flavor in the referenced data sources. Define
users and SSH keys in `config.ign` and omit `ssh_key_ids`: this document replaces
automatic setup. With custom data, `admin_username` only records the user for
connection information; it does not create a guest user.

Both arguments are required together. The maximum size is 64 KiB of UTF-8.
Changing or removing either argument **replaces the VM**, because first-boot
configuration cannot update an existing guest. The API encrypts the document and
does not return it; Terraform retains it as a sensitive value in state. Secure
your state backend. Import cannot recover custom data, so adding these arguments
to an imported instance plans replacement.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `admin_password` | `string` | Optional; sensitive | macOS admin password (8–128 printable ASCII characters). Stored in Terraform state. Removing this argument stops managing the password; it does not clear the guest password. |
| `admin_username` | `string` | Optional / default or computed | Guest admin user, defaulted by the image/API. With user_data, this is connection metadata only; define the user in your document. |
| `boot_into_recovery` | `bool` | Optional / default or computed | macOS recovery boot. Changing this setting may restart the VM. Requires an API exposing bootIntoRecovery in the instance DTO. |
| `cpu_cores` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `disk_gib` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `display_height` | `number` | Optional / default or computed |  |
| `display_width` | `number` | Optional / default or computed |  |
| `expires_at` | `string` | Read-only | Server-enforced deletion deadline, or null. |
| `flavor_slug` | `string` | Optional / default or computed | Catalog flavor slug. CPU, memory and disk must match the flavor; this is not a sizing resolver. |
| `hardware_generation` | `string` | Optional / default or computed | Hardware generation constraint. Resolved from the flavor when flavor_slug is supplied. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `image_ref` | `string` | Optional / default or computed | Image name or OCI reference accepted by the public API. Exactly one of image_ref and source_snapshot_id is required; with source_snapshot_id it reads the snapshot's source image. Changes replace the VM. |
| `lifetime_seconds` | `number` | Optional | Automatic deletion deadline relative to creation. Omit for unlimited lifetime. |
| `memory_gib` | `number` | Required | Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected. |
| `name` | `string` | Required | Display name. |
| `network_address` | `string` | Read-only | Primary guest IP address. |
| `network_ref` | `string` | Optional / default or computed | Network name in this region. |
| `platform` | `string` | Optional / default or computed | Guest platform: macos or linux. Linux requires SSH keys or a complete user_data configuration. |
| `power_state` | `string` | Optional / default or computed | Desired power state: running, stopped or suspended. |
| `region_id` | `string` | Required | Region UUID. |
| `security_groups` | `set(string)` | Optional / default or computed | Names of security groups in the same region. |
| `shutdown_mode` | `string` | Optional / default or computed | How to stop before resize or when power_state is stopped: graceful (ACPI shutdown, default) or hard (immediate stop). |
| `source_snapshot_id` | `string` | Optional | UUID of a ready snapshot in the same region to create the VM from instead of image_ref. platform must match the snapshot's, disk_gib must be at least its disk_gib and cpu_cores and memory_gib at least its minimums. The guest keeps the snapshot's users: user_data and admin_username do not apply. Changes replace the VM; deleting the snapshot later does not. |
| `ssh_key_ids` | `set(string)` | Optional / default or computed | SSH key UUIDs to inject into the guest. |
| `status` | `string` | Read-only | Actual API lifecycle status. |
| `tags` | `map(string)` | Optional / default or computed | Metadata tags. Keys: lowercase letters, digits, dots, underscores, slashes, hyphens; at most 20 tags. |
| `user_data` | `string` | Optional; sensitive | Complete first-boot configuration, using the Terraform file function. Maximum 64 KiB UTF-8. Replaces automatic SSH setup: define users and keys here and omit ssh_key_ids. Encrypted by the API, but stored in Terraform state; secure your state backend. Changes (including removal) replace the VM; imported user-data cannot be recovered. |
| `user_data_format` | `string` | Optional | cloud-init (YAML starting with #cloud-config) or ignition (JSON for CoreOS). Required together with user_data. Changes replace the VM. |

## Import

> **Check for VM replacement after import.** The provider emits an import warning:
> `user_data` and `user_data_format` cannot be recovered and remain null. If your
> configuration sets either argument, the next plan replaces the imported VM.
> To keep it, omit both arguments and inspect the plan before applying.


```sh
terraform import xcloud_instance.existing 11111111-1111-4111-8111-111111111111
```

Define the matching resource block before import.

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
