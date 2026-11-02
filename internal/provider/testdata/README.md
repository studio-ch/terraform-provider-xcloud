# Public API fixture

`public-api.json` was exported from the current Studio Control Plane public API
on 2026-11-02 and reduced to the `/v1/xcloud*`, `/v1/registry-credentials*`,
`/v1/ssh-keys*` and `/v1/regions` paths. It contains no credentials or tenant data.

Regenerate from the monorepo root:

```sh
pnpm --silent --filter @studio-cp/api openapi:dump > /tmp/xcloud-openapi.json
python3 apps/terraform-provider-xcloud/scripts/update_api_contract.py /tmp/xcloud-openapi.json
```

The local mock validates requests and successful response status/body semantics
against this fixture. It is not a complete OpenAPI validator and does not assert
that every response DTO field matches the schema.

The Elastic-IP target operation additionally includes the current exported 409
response for an already assigned address.
