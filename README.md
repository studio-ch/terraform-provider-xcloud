# Terraform Provider for Xcloud

Manage Xcloud through the **public Cloud Console REST API** at
`https://api.cloud.flow.swiss/v1`. The provider uses a tenant-scoped Bearer API
key and the HashiCorp Terraform Plugin Framework (protocol 6). It has no database,
admin-session, CLI subprocess, or upstream Xcloud credentials dependency.

The provider is maintained in its public repository:
[studio-ch/terraform-provider-xcloud](https://github.com/studio-ch/terraform-provider-xcloud).
The provider is published in the [Terraform Registry](https://registry.terraform.io/providers/studio-ch/xcloud/0.1.0-beta.2/docs)
as `studio-ch/xcloud`. The current signed preview is `0.1.0-beta.2`.

## Install from the Terraform Registry

Declare the provider in your Terraform configuration:

```hcl
terraform {
  required_providers {
    xcloud = {
      source  = "studio-ch/xcloud"
      version = "0.1.0-beta.2"
    }
  }
}

provider "xcloud" {}
```

Run `terraform init` and `terraform validate`. For plans and applies, provide your
API key through `XCLOUD_API_TOKEN`. Terraform downloads the correct platform archive
and verifies the release signature. Go and custom CLI installation settings are
not required. Keep the exact prerelease version constraint while evaluating the preview.

If you previously configured an Xcloud `dev_overrides` or mirror entry, remove it
and any matching `direct` exclusion, or select your usual CLI configuration before
initializing from the Registry. The source address remains unchanged.
[Filesystem mirror installation](docs/mirror-installation.md) remains available
for restricted-network environments.

## Resources

| Terraform type | Behavior | Import ID |
| --- | --- | --- |
| `xcloud_instance` | Create, rename, resize, graceful/hard shutdown, suspend/resume, recovery boot, tags, passwords, SSH keys, security groups, lifetime, delete | Instance UUID |
| `xcloud_network` | Create/read/delete; configuration changes require replacement | `region-uuid/name` |
| `xcloud_security_group` | Ordered rule set and labels, updated in place | `region-uuid/name` |
| `xcloud_volume` | Persistent volume; grow in place | Volume UUID |
| `xcloud_volume_attachment` | Attach/detach an existing volume; attach restarts the VM | `instance-uuid/volume-uuid` |
| `xcloud_elastic_ip` | Allocate, assign, unassign, move and release a public IPv4 address | Elastic IP UUID |
| `xcloud_ssh_key` | Register, rename and delete an SSH public key | Key UUID |
| `xcloud_image` | Register an OCI image, update labels, remove the catalog entry; optional registry deletion | `region-uuid/name` |
| `xcloud_registry_credential` | Create, update and delete registry authentication, including password rotation | Credential UUID |

The 20 data sources cover individual regions, flavors, images, instances,
networks, security groups, volumes, Elastic IPs, SSH keys and registry credentials,
plus lists of each (`xcloud_instances`, `xcloud_images`, etc.). Catalog lookups
use slug or region/name; other lookups use UUID or region/name. Typed fields
cover common attributes; `response_json` exposes the complete public DTO,
including additional pricing and availability metadata. Lists return `items`.
See [`docs/`](./docs/index.md) for the schema and examples.

## Download the preview

Download the archive for your workstation from
[GitHub Releases](https://github.com/studio-ch/terraform-provider-xcloud/releases).
The current signed preview is `v0.1.0-beta.2`. Builds are available for macOS, Linux and
Windows on AMD64 and ARM64. Extract the archive into a dedicated directory and
point `dev_overrides` below to that directory. The archive contains
`terraform-provider-xcloud_v0.1.0-beta.2`
(`terraform-provider-xcloud_v0.1.0-beta.2.exe` on Windows).

Verify the archive against the release's
`terraform-provider-xcloud_0.1.0-beta.2_SHA256SUMS` file before extracting it:
`shasum -a 256 <archive.zip>` on macOS or `sha256sum <archive.zip>` on Linux.
Starting with `v0.1.0-beta.2`, the checksums have a detached GPG signature.
Verify that signature with the [public release key](keys/xcloud-release-signing.asc)
before trusting the checksums. Confirm that the key fingerprint matches
the following value before importing it:
`1B4B04AEE0AB49EB8D24410E4F5D61DF38144775`.

```sh
curl -fsSLo xcloud-release-signing.asc https://raw.githubusercontent.com/studio-ch/terraform-provider-xcloud/main/keys/xcloud-release-signing.asc
gpg --show-keys --fingerprint xcloud-release-signing.asc
gpg --import xcloud-release-signing.asc
gpg --verify terraform-provider-xcloud_0.1.0-beta.2_SHA256SUMS.sig terraform-provider-xcloud_0.1.0-beta.2_SHA256SUMS
```

The earlier `v0.1.0-beta.1` download remains unsigned.
No Go installation is needed when using a downloaded binary.

## Build and use locally

Requires Go 1.26.7 and Terraform >= 1.5. Tests also run the Terraform executable.
Clone the public repository, then build:

```sh
git clone https://github.com/studio-ch/terraform-provider-xcloud.git
cd terraform-provider-xcloud
make build
```

Add a `dev_overrides` entry to your Terraform CLI configuration (`~/.terraformrc`,
or a separate file selected with `TF_CLI_CONFIG_FILE`). Replace the directory
below with the absolute path to this module's `dist` directory:

```hcl
provider_installation {
  dev_overrides {
    "studio-ch/xcloud" = "/absolute/path/to/terraform-provider-xcloud/dist"
  }
  direct {}
}
```

Development overrides use your local build during `plan` and `apply`.
`terraform init` still resolves the declared release version from the Registry;
it does not install that local build. Use the Registry installation above for
normal release evaluation.

Issue a **Read + Write** API key in the panel's API-key settings. The key fixes
the organization; use separate provider aliases with different keys for multiple
organizations.

```sh
export XCLOUD_API_TOKEN='your-api-key'
cd examples/basic
terraform plan \
  -var='flavor_slug=your-catalog-flavor' \
  -var='image_name=your-catalog-image' \
  -var='ssh_source_cidr=your-office-address/32'
```

Replace all three values with entries appropriate for your account. Select an
image and flavor with the same platform. The complete example manages an SSH
key, firewall, instance and public IP. Use the same variables with `terraform
apply` to provision it. Terraform displays the billable resources before applying.

## Provider configuration

```hcl
provider "xcloud" {
  api_url                   = "https://api.cloud.flow.swiss"
  operation_timeout_seconds = 1800
  poll_interval_seconds     = 5
}
```

Resolution is per field: explicit HCL, `XCLOUD_API_URL` / `XCLOUD_API_TOKEN`,
`CLOUDCONSOLE_API_URL` / `CLOUDCONSOLE_API_TOKEN`, then the default URL. The URL
may include a trailing `/v1`. HTTPS is required except for loopback development.
Redirects are rejected. The token is sensitive; environment variables avoid
putting it in configuration files. Normal Terraform state still needs protected
storage and access controls.

## Lifecycle details

- Instance creation waits for `running` with no pending action. If configured
  `power_state = "stopped"` or `"suspended"`, it then applies that state. Create failures and timeouts
  retain the allocated ID in state so Terraform can clean up the resource.
- Changing CPU, memory, disk or flavor stops the VM, resizes it, then restores
  `power_state` (default `running`). This causes downtime. Disk and volume
  shrinking fail without deleting data. Use a flavor data source for sizing:
  the API resolves catalog sizing authoritatively, so configured numbers must
  match the selected flavor. Custom sizing requires tenant permission and an
  allowed `hardware_generation`.
- Region, image, network, platform, display dimensions and explicitly changed
  hardware generation/admin username require instance replacement. Name,
  lifetime, tags, passwords, recovery boot, SSH keys and security groups update in place.
- `shutdown_mode` defaults to `graceful` (ACPI). Use `hard` for immediate stop.
  `power_state` accepts `running`, `stopped` and `suspended`. Recovery boot is
  macOS-only and can restart the VM; the API must expose `bootIntoRecovery`.
- `admin_password` is macOS-only. Create and rotation store the desired password
  in the API; guest injection happens asynchronously. A successful apply does
  not verify a guest login. Passwords are sensitive but remain in Terraform
  state. Removing a password argument stops managing it without clearing it.
- Refresh warns when the VM reports `status = "error"`, including the API's
  error reason, and retains its state. The configured `power_state` is not a
  health indicator. The warning permits deliberate repair, replacement or destroy.
- `lifetime_seconds` is relative to the original creation time. Omit it for
  unlimited lifetime. Once the server deletes an expired VM, Terraform detects
  its absence and plans recreation while its resource block remains configured.
- Volume attachments have their own resource so Terraform detaches before
  deleting volumes or VMs. Reference the instance and volume resource IDs to
  establish those dependencies. Data volumes currently require Linux guests.
- Elastic IP operations track the API's recorded assignment. Its `status` is
  a stored upstream observation, not a live connectivity probe; the provider
  does not wait indefinitely for a `bound` observation that list reads do not
  refresh. Moving an IP detaches the old target before attaching the new one.
  VM deletion explicitly retains Elastic IPs (`releaseElasticIps=false`);
  `xcloud_elastic_ip` owns their release, including across VM replacement.
- `spec_json = jsonencode({...})` passes arbitrary network specification fields.
  `effective_spec_json` exposes upstream defaults; drift comparison covers the
  fields configured in `spec_json`. Do not configure a property both there and
  via a flat argument. Network changes replace the network. To replace dependent
  VMs when a network keeps the same name, set their lifecycle
  `replace_triggered_by = [xcloud_network.private]`.
- Images accept saved credentials, ad-hoc username/password, or anonymous
  authentication, and optional precaching. Editable labels exclude worker keys;
  `all_labels` also exposes those keys. Destruction retains registry bytes by
  default. Set `delete_from_registry = true` explicitly to delete them too;
  authenticated deletion uses saved registry credentials, not ad-hoc passwords.
- The API's network list can return an empty result during a region outage.
  Image catalogs have the same ambiguity. An absent managed network or image therefore **stops refresh with an error**, preserving state.
  After confirming an external deletion, use `terraform state rm` for that
  network before recreating it. Default catalog networks cannot be managed as
  resources; reference their names through `network_ref`.
- `public_key` cannot be read back from the API. SSH-key import leaves it null;
  keep it omitted for an imported key, or explicitly configure it to replace
  that key. Existing managed keys retain their configured public key in state.
- GET requests and exactly `POST /v1/xcloud/instances` retry transient
  failures with bounded backoff. Only instance creation persists replay responses;
  merely mounting idempotency middleware does not protect a write. All other
  writes, including network/security-group creation and VM actions, are not
  replayed after transport failures, HTTP 408 or 5xx. An ambiguous create can
  still leave a remote resource: inspect the API and import it before reapplying. All requests can retry
  rate-limit responses, honoring `Retry-After`. No cookies or admin endpoints
  are used.

Elastic IPs, SSH keys and registry credentials have no public GET-by-ID endpoint.
Each resource refresh scans its collection: O(N) rows per refresh, potentially
O(N²) rows for N separately managed resources. API-side lookup or batching
support would reduce this cost.

## Development and verification

```sh
make test
make lint
```

Tests use a local HTTP server and real Terraform provider RPCs; they create no
cloud resources. Coverage includes all nine resources and all 20 data sources, import,
VM replacement with stable Elastic IP, flavor resize, tags/lifetime drift,
password rotation, suspend/recovery, image labels, network specs, and failure
state retention. Requests and successful response status/body semantics are
checked against a bundled public OpenAPI snapshot. This does not validate every
response DTO field or replace a real deployment acceptance run.

GitHub Actions builds the provider, checks Go and Terraform formatting, runs
`go vet`, and executes the full test suite with the race detector on pushes and
pull requests. No monorepo or Go workspace is required.

The signed `0.1.0-beta.2` release is published in the Terraform Registry. A fresh
direct Registry installation, signature verification, validation and schema loading
passed on macOS ARM64. A real deployment acceptance run remains pending; the
provider remains a preview. Building and testing do not create cloud resources.

The implemented surface covers infrastructure lifecycle. Guest command execution,
consoles, image push/build jobs, metrics, maintenance actions and billing/admin operations are not
Terraform resources in this version.

See the [feature audit](docs/feature-audit.md) for the exact scope and remaining
Stage/release work. To refresh schema tables after a build:
`python3 scripts/generate_docs.py /path/to/terraform-schema.json`.
