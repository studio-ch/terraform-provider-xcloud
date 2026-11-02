# `xcloud_snapshot`

A snapshot of an Xcloud instance's boot disk and firmware settings, never its memory or attached volumes. Taken from a running instance it is crash-consistent. Create waits until the snapshot is ready; delete waits until xcloud has reclaimed it, which waits while an instance created from it has never started. Import with the snapshot UUID.

## Example

```hcl
resource "xcloud_snapshot" "baseline" {
  instance_id = xcloud_instance.server.id
  name        = "build-server-baseline"
  description = "Toolchain installed, before the first build"
}
```

A running instance is snapshotted in place, and the snapshot is crash-consistent,
as after a sudden power loss. For a clean snapshot, set `power_state = "stopped"`
on the instance first. A suspended instance is refused. Without `name` the API
names the snapshot `<instance name> <UTC timestamp>`; names are unique in the
organisation. `name` and `description` change in place; a different `instance_id`
takes a new snapshot.

Creation fails with the API's `lastError` when the snapshot turns `failed`; the
failed snapshot stays in state as tainted and is replaced on the next apply.

A snapshot outlives its instance, keeps being billed, and can still seed new
instances through `xcloud_instance.source_snapshot_id`. Once the instance is
deleted, `instance_id` keeps its last value. Restoring an instance to a snapshot
is an imperative action and is not modelled in Terraform; use the panel, the API
(`POST /v1/xcloud/instances/{id}/restore`) or `cloudconsole instance restore`.

Deletion waits until xcloud has reclaimed the storage, which waits while an
instance created from the snapshot has never started. Start such instances once
before destroying the snapshot, or the destroy runs into
`operation_timeout_seconds`.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `capture_time` | `string` | Read-only | The moment the captured disk contents represent (RFC 3339). |
| `description` | `string` | Optional | Free-text description, at most 1000 characters. Changes are in place. |
| `disk_gib` | `number` | Read-only | Captured boot disk in GiB; the minimum disk of an instance created from it. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `instance_id` | `string` | Required | UUID of the instance to snapshot. A suspended instance is refused. Changes take a new snapshot. |
| `min_cpu_cores` | `number` | Read-only | Minimum CPU cores of an instance created from it. |
| `min_memory_gib` | `number` | Read-only | Minimum memory in GiB of an instance created from it. |
| `name` | `string` | Optional / default or computed | Snapshot name, unique in the organisation. Defaults to `<instance name> <UTC timestamp>`. Renaming is in place. |
| `platform` | `string` | Read-only | Guest platform of the source instance: macos or linux. |
| `region_id` | `string` | Read-only | Region UUID, the instance's. |
| `source_instance_name` | `string` | Read-only | Name of the source instance when the snapshot was taken. |
| `status` | `string` | Read-only | Snapshot status: creating, ready, failed or deleting. |
| `taken_while_running` | `bool` | Read-only | Whether the snapshot was taken from a running instance and is therefore crash-consistent. |

## Import

```sh
terraform import xcloud_snapshot.existing 77777777-7777-4777-8777-777777777777
```

Define the matching resource block before import. `name` is read from the API;
set it in the configuration to keep it managed.

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
