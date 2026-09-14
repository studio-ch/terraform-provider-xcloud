# `xcloud_registry_credential`

Tenant registry credential. Import with its UUID. Passwords are sensitive but stored in Terraform state; use a protected backend.

See the [complete catalog example](../../examples/catalog/main.tf).

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `display_name` | `string` | Required | Display name. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `password` | `string` | Optional; sensitive | Required on creation; not returned by the API. Removing it stops managing the password without clearing it. |
| `registry_url` | `string` | Required | Registry HTTP(S) URL. |
| `source` | `string` | Read-only | Credential ownership: user or managed. |
| `username` | `string` | Required | Registry username. |

## Import

```sh
terraform import xcloud_registry_credential.existing 66666666-6666-4666-8666-666666666666
```

Define the matching resource block before import. Passwords and registration authentication cannot be reconstructed from the API.

[Provider setup and lifecycle details](../../README.md)
