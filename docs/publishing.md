# Publishing the Xcloud provider

The public repository is `studio-ch/terraform-provider-xcloud`; the published
Terraform Registry address is `studio-ch/xcloud`.

## Registration status (2026-09-14)

Published: [studio-ch/xcloud 0.1.0-beta.2](https://registry.terraform.io/providers/studio-ch/xcloud/0.1.0-beta.2/docs).
The public versions endpoint lists all six platform archives with protocol 6.
A fresh `terraform init` using only direct Registry installation succeeded on
macOS ARM64 and verified signing key `4F5D61DF38144775`. Validation and schema loading
also passed (9 resources and 20 data sources). No live cloud resources were created.

The `studio-ch` public namespace belongs to HCP Terraform organization `flow-swiss`.
The Terraform Cloud GitHub App has access to the provider repository and the
namespace holds the public release-signing key documented below.

Initial attempts in the Codex in-app browser displayed errors on namespace pages.
The same namespace and settings worked in Dia, where publication completed without
reclaiming the namespace. A server-side namespace defect was not confirmed.
[Support case #50](https://github.com/hashicorp/terraform-registry-support/issues/50)
records the investigation. Use a conventional browser for publication management.

The unsigned legacy `v0.1.0-beta.1` could not be imported because it lacks the
required checksum/signature files. It is retained unchanged on GitHub; the signed
`v0.1.0-beta.2` is the first Registry release.

## Release assets

Tags trigger `.github/workflows/release.yml`, which tests the provider, builds six
platform archives, writes the protocol 6 manifest, signs the versioned checksum
file, verifies its signature and publishes a GitHub prerelease. Existing versions
must not be overwritten. `v0.1.0-beta.1` is the original unsigned preview;
`v0.1.0-beta.2` is the first release published in the Registry.

The repository secrets `GPG_PRIVATE_KEY` and `PASSPHRASE` contain a dedicated
RSA release key. Its public counterpart is [xcloud-release-signing.asc](../keys/xcloud-release-signing.asc).
The expected fingerprint is `1B4B04AEE0AB49EB8D24410E4F5D61DF38144775`.
Recovery material is stored outside the source checkout with restricted filesystem
permissions. Do not commit private keys or passphrases. Preserve the public key
when rotating keys so existing releases can still be verified.

## Registration checklist (completed)

1. Sign in through [HCP Terraform public namespaces](https://app.terraform.io/app/registry/public-namespaces).
2. Connect or select the GitHub namespace `studio-ch`, with repository administration
   rights for `terraform-provider-xcloud`.
3. Add the public release-signing key to that namespace and confirm its fingerprint.
4. Select the provider repository and publish the signed version. Keep the legacy
   unsigned beta.1 release out of the selected publication if the UI asks.
5. Verify that the Registry lists `0.1.0-beta.2` and test a fresh `terraform init`
   with `source = "studio-ch/xcloud"` and `version = "0.1.0-beta.2"`, without
   development overrides or a filesystem mirror. Then run `terraform providers schema -json`.
6. Update availability text and customer documentation only after that check passes.

Initial registration is not performed by the GitHub release workflow. Once the
Registry integration is connected, subsequent signed releases can be indexed
through that integration.

References: [release requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing)
and [provider documentation format](https://developer.hashicorp.com/terraform/registry/providers/docs).
