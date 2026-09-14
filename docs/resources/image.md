# `xcloud_image`

Register a tenant OCI image and manage its labels. Import: region-uuid/name. By default deletion removes the catalog entry and retains registry bytes.

See the [complete catalog example](../../examples/catalog/main.tf).

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `all_labels` | `map(string)` | Read-only | All labels, including worker-managed values. |
| `credential_id` | `string` | Optional | Saved registry credential UUID. Omit all authentication fields for anonymous access. |
| `delete_from_registry` | `bool` | Optional / default or computed | Also delete OCI registry bytes when destroying this resource. Defaults to false. Use saved credentials when authenticated deletion is needed. |
| `id` | `string` | Read-only | Resource identifier used for import. |
| `labels` | `map(string)` | Optional / default or computed | Editable image labels, excluding worker keys source, digest and precache. |
| `name` | `string` | Required | Region-local resource name. Changes replace the resource. |
| `oci_reference` | `string` | Required | OCI reference. |
| `password` | `string` | Optional; sensitive | Ad-hoc registry password. Sensitive but stored in Terraform state. |
| `precache` | `bool` | Optional / default or computed | Request image precaching on registration. Changes replace the catalog entry. |
| `region_id` | `string` | Required | Region UUID. |
| `source` | `string` | Read-only | Image ownership. |
| `username` | `string` | Optional | Ad-hoc registry username; requires password and conflicts with credential_id. |

## Import

```sh
terraform import xcloud_image.existing 11111111-1111-4111-8111-111111111111/custom
```

Define the matching resource block before import. Passwords and registration authentication cannot be reconstructed from the API.

[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)
