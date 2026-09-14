# Publishing the Xcloud provider

The public repository is `studio-ch/terraform-provider-xcloud`; the intended
Terraform Registry address is `studio-ch/xcloud`.

## Registration status (2026-09-14)

The `studio-ch` public namespace is claimed by the HCP Terraform organization
`flow-swiss`. The Terraform Cloud GitHub App has access to the provider repository,
and the namespace lists release-signing key `4F5D61DF38144775` (the fingerprint below).

The signed GitHub release `v0.1.0-beta.2` is published and its downloaded artifacts
have passed signature, checksum and local Terraform loading checks. Public Registry
publication is still pending: HCP intermittently fails to load the namespace's
artifact list with "An unexpected error occurred. Please try again later."
The public versions endpoint still returns `provider not found`.

Until publication is restored, [filesystem mirror installation](mirror-installation.md)
provides a verified path for normal `terraform init` using the signed GitHub
release. This is an installation workaround, not public Registry publication.

Resume from [the existing namespace](https://app.terraform.io/app/flow-swiss/registry/public-namespaces/studio-ch),
using **Publish → Provider** once that page loads. The namespace and signing key
are already configured; do not recreate them. Complete the fresh Registry
installation check below before changing customer-facing availability claims.

## Release assets

Tags trigger `.github/workflows/release.yml`, which tests the provider, builds six
platform archives, writes the protocol 6 manifest, signs the versioned checksum
file, verifies its signature and publishes a GitHub prerelease. Existing versions
must not be overwritten. `v0.1.0-beta.1` is the original unsigned preview;
`v0.1.0-beta.2` is the first release prepared for Registry registration.

The repository secrets `GPG_PRIVATE_KEY` and `PASSPHRASE` contain a dedicated
RSA release key. Its public counterpart is [xcloud-release-signing.asc](../keys/xcloud-release-signing.asc).
The expected fingerprint is `1B4B04AEE0AB49EB8D24410E4F5D61DF38144775`.
Recovery material is stored outside the source checkout with restricted filesystem
permissions. Do not commit private keys or passphrases. Preserve the public key
when rotating keys so existing releases can still be verified.

## One-time Registry registration

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
