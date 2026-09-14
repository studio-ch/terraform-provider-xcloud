# Install the signed preview through a filesystem mirror

The provider is now published in the Terraform Registry; prefer the
[standard installation](../README.md#install-from-the-terraform-registry).
This mirror method remains an alternative for restricted-network environments. It supports normal `terraform init`, version selection
and dependency locking with the source address `studio-ch/xcloud`.
It does not publish the provider to the public Registry.

## 1. Download and verify the archive

Download `v0.1.0-beta.2` from
[GitHub Releases](https://github.com/studio-ch/terraform-provider-xcloud/releases/tag/v0.1.0-beta.2).
Choose the ZIP for the machine that will execute Terraform:

| Machine | Target |
| --- | --- |
| macOS Apple Silicon | `darwin_arm64` |
| macOS Intel | `darwin_amd64` |
| Linux ARM64 | `linux_arm64` |
| Linux x86-64 | `linux_amd64` |
| Windows ARM64 | `windows_arm64` |
| Windows x86-64 | `windows_amd64` |

Follow the [release signature and checksum verification instructions](../README.md#download-the-preview)
before using the archive. The expected signing-key fingerprint is
`1B4B04AEE0AB49EB8D24410E4F5D61DF38144775`.
Use the verified SHA256SUMS to check the SHA256 digest of your selected ZIP.

Keep the ZIP packed. Place it under this directory structure, replacing the
mirror root and target with your actual values:

```text
/absolute/path/to/xcloud-mirror/
  registry.terraform.io/
    studio-ch/
      xcloud/
        terraform-provider-xcloud_0.1.0-beta.2_darwin_arm64.zip
```

Additional target ZIPs can coexist in that same directory. Store the public key,
checksums and signature outside the mirror's provider directory.

## 2. Select the mirror

Create a separate `xcloud.tfrc` file. Use an absolute path to the mirror root;
on Windows use forward slashes, for example `C:/terraform/xcloud-mirror`.

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/absolute/path/to/xcloud-mirror"
    include = ["studio-ch/xcloud"]
  }
  direct {
    exclude = ["studio-ch/xcloud"]
  }
}
```

Both rules matter: the exclusion prevents Terraform from querying the
Registry entry for Xcloud. Other providers continue to use their normal registries.
If your workflow needs other CLI settings, include those in this file too:
`TF_CLI_CONFIG_FILE` selects a configuration file instead of merging it with your
usual one. Do not add a `dev_overrides` entry for this installation.

Select the file in your shell (macOS/Linux):

```sh
export TF_CLI_CONFIG_FILE=/absolute/path/to/xcloud.tfrc
```

Or PowerShell:

```powershell
$env:TF_CLI_CONFIG_FILE = 'C:/terraform/xcloud.tfrc'
```

## 3. Initialize the Terraform configuration

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

```sh
terraform init
terraform validate
```

Initialization and validation need no Xcloud token and create no cloud resources.
For `plan` and `apply`, provide your tenant's API key through `XCLOUD_API_TOKEN`
and use the [resource examples](../examples/basic/main.tf).

Terraform reports a mirror package as `unauthenticated`: it does not verify the
release's detached GPG signature when reading a filesystem mirror. That is why
the explicit signature and checksum verification in step 1 is required.

Commit `.terraform.lock.hcl`. Initial installation creates checksums for the
current platform. For a mixed-platform team, first put the verified ZIP for every
needed target into the mirror, then generate the corresponding lock entries:

```sh
terraform providers lock \
  -fs-mirror=/absolute/path/to/xcloud-mirror \
  -platform=darwin_arm64 \
  -platform=linux_amd64 \
  studio-ch/xcloud
```

Adjust the platform list to your team. Each machine or CI runner needs access
to the configured mirror. A local mirror on your laptop is not automatically
available to HCP remote runs.

## Verification and later Registry migration

On 2026-09-14, the published macOS ARM64 ZIP passed GPG and SHA256 verification,
then `terraform init`, `terraform validate` and `terraform providers schema -json`
in a fresh directory with a dedicated CLI configuration and no development
overrides. The loaded schema contains 9 resources and 20 data sources.
This does not constitute an acceptance test against live Xcloud resources.

To switch to the verified public Registry installation, remove the Xcloud mirror rule
and its `direct` exclusion (or select your usual CLI configuration). Keep the
same source address and version constraint; no Terraform state address migration
is necessary. Run `terraform init` to verify installation through the Registry.

References: [Terraform installation methods](https://developer.hashicorp.com/terraform/cli/config/config-file#explicit-installation-method-configuration)
and [provider lock command](https://developer.hashicorp.com/terraform/cli/commands/providers/lock).
