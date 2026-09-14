# `xcloud_region`

Look up an Xcloud region through the public catalog API. Missing or ambiguous results produce an error.

## Example

```hcl
data "xcloud_region" "selected" {
  slug = "ZRH1"
}
```

The slug match is case-insensitive. The region must advertise the Xcloud service. A missing or ambiguous match is an error.

## Schema

| Attribute | Type | Usage | Description |
| --- | --- | --- | --- |
| `architecture` | `string` | Read-only |  |
| `id` | `string` | Read-only |  |
| `name` | `string` | Read-only |  |
| `response_json` | `string` | Read-only | Complete public catalog DTO as JSON, including availability, pricing and other metadata exposed by the API. |
| `slug` | `string` | Required | Exact catalog slug. |

[Provider setup and lifecycle details](../../README.md)
