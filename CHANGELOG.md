# Changelog

## v0.1.0-beta.1 — 2026-09-14

First public evaluation release of the Xcloud Terraform provider, using the
public Cloud Console REST API and organisation-scoped API keys.

- Manage nine resources: instances, networks, security groups, volumes, volume
  attachments, Elastic IPs, SSH keys, OCI images and registry credentials.
- Read individual objects and collections through twenty data sources.
- Import existing resources; manage VM sizing, tags, lifetimes, passwords,
  graceful or hard shutdown, suspend/resume and macOS recovery boot.
- Configure network JSON specifications, image labels and registry authentication.
- Preserve separately managed Elastic IPs during VM replacement and retain image
  registry bytes by default when removing catalog entries.
- Retry uncertain writes only for instance creation, where the API persists replay
  responses. Warn when a VM reports an error and preserve state on ambiguous reads.
- Include examples, resource reference, local Terraform lifecycle tests and an
  OpenAPI contract fixture.

Download a ZIP for macOS, Linux or Windows on AMD64 or ARM64, verify its checksum
using `SHA256SUMS`, and follow the README's `dev_overrides` installation guide.
The binaries and checksums are unsigned. This preview is not published in the
Terraform Registry and has not completed acceptance against a real deployment.
Passwords may remain in Terraform state. After an uncertain create, inspect the
API and import any existing resource before retrying.

The clearer occupied-Elastic-IP conflict response requires the corresponding
Cloud Console API update; it is not a change to the remote API made by this binary.
