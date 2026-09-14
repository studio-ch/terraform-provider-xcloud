# `xcloud_ssh_key`

Read existing Xcloud ssh_keys through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `fingerprint_sha256` | `string` | Read-only |  |
| `id` | `string` | Required | Exact resource id. |
| `name` | `string` | Read-only |  |
| `response_json` | `string` | Read-only |  |

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
