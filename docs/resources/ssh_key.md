# `xcloud_ssh_key`

Tenant SSH public key. Import with the key UUID. The API does not return public key material, so an imported key leaves public_key null until explicitly configured.

## Example

```hcl
resource "xcloud_ssh_key" "admin" {
  name       = "terraform-admin"
  public_key = file(pathexpand("~/.ssh/id_ed25519.pub"))
}
```

The API returns key metadata, not the public key material. `public_key` is required on creation and retained in state. Import leaves it null; keep it omitted for that imported resource, or explicitly configure it to replace the key. Private keys must never be supplied.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `fingerprint_sha256` | `string` | Read-only | SHA256 fingerprint returned by the API. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `name` | `string` | Required | Display name. |
| `public_key` | `string` | Optional | OpenSSH public key. Required to create, optional after import. Changes replace the key. |

## Import

```sh
terraform import xcloud_ssh_key.existing 11111111-1111-4111-8111-111111111111
```

Define the matching resource block before import.

[Provider setup and lifecycle details](../../README.md)
