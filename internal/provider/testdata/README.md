# Public API fixture

`public-api.json` was exported from the current Studio Control Plane public API
on 2026-09-14 and reduced to the `/v1/xcloud*`, `/v1/registry-credentials*`,
`/v1/ssh-keys*` and `/v1/regions` paths. It contains no credentials or tenant data.

Obtain a current public OpenAPI JSON export from the API maintainers, then
regenerate from this repository root:

```sh
python3 scripts/update_api_contract.py /path/to/xcloud-openapi.json
```

The local mock validates requests and successful response status/body semantics
against this fixture. It is not a complete OpenAPI validator and does not assert
that every response DTO field matches the schema.

The Elastic-IP target operation additionally includes the current exported 409
response for an already assigned address.
